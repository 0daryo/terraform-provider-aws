package acctest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/dnaeon/go-vcr/v2/cassette"
	"github.com/dnaeon/go-vcr/v2/recorder"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	sdkacctest "github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
)

const (
	envVarVCRMode = "VCR_MODE"
	envVarVCRPath = "VCR_PATH"
)

type randomnessSource struct {
	seed   int64
	source rand.Source
}

var (
	providerStatesLock = sync.RWMutex{}
	providerStates     = make(map[string]interface{}, 0)

	randomnessSourcesLock = sync.RWMutex{}
	randomnessSources     = make(map[string]*randomnessSource, 0)
)

// ProviderState returns the current provider's state (AKA "Config" or "conns.AWSClient").
func ProviderState(t *testing.T) *conns.AWSClient {
	providerStatesLock.RLock()
	v, ok := providerStates[t.Name()]
	providerStatesLock.RUnlock()

	if !ok {
		v = Provider.Meta()
	}

	return v.(*conns.AWSClient)
}

func isVCREnabled() bool {
	return os.Getenv(envVarVCRMode) != "" && os.Getenv(envVarVCRPath) != ""
}

func vcrMode() (recorder.Mode, error) {
	switch v := os.Getenv(envVarVCRMode); v {
	case "RECORDING":
		return recorder.ModeRecording, nil
	case "REPLAYING":
		return recorder.ModeReplaying, nil
	default:
		return recorder.ModeDisabled, fmt.Errorf("unsupported value for %s: %s", envVarVCRMode, v)
	}
}

// vcrEnabledProviderFactories returns provider factories ready for use with VCR.
func vcrEnabledProviderFactories(t *testing.T, input map[string]func() (*schema.Provider, error)) map[string]func() (*schema.Provider, error) {
	output := make(map[string]func() (*schema.Provider, error), len(input))

	for k, fn := range input {
		fn := fn

		output[k] = func() (*schema.Provider, error) {
			provider, err := fn()

			if err != nil {
				return nil, err
			}

			provider.ConfigureContextFunc = vcrProviderConfigureContextFunc(provider.ConfigureContextFunc, t.Name())

			return provider, nil
		}
	}

	return output
}

// vcrProviderConfigureContextFunc returns a provider configuration function returning cached provider instance state.
// This is necessary as ConfigureContextFunc is called multiple times for a given test, each time creating a new HTTP client.
// VCR requires a single HTTP client to handle all interactions.
func vcrProviderConfigureContextFunc(configureFunc schema.ConfigureContextFunc, testName string) schema.ConfigureContextFunc {
	return func(ctx context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
		providerStatesLock.RLock()
		v, ok := providerStates[testName]
		providerStatesLock.RUnlock()

		if ok {
			return v, nil
		}

		v, diags := configureFunc(ctx, d)

		if diags.HasError() {
			return nil, diags
		}

		vcrMode, err := vcrMode()

		if err != nil {
			return nil, diag.FromErr(err)
		}

		path := filepath.Join(os.Getenv(envVarVCRPath), vcrFileName(testName))
		c := v.(*conns.AWSClient)

		recorder, err := recorder.NewAsMode(path, vcrMode, c.Session.Config.HTTPClient.Transport)

		if err != nil {
			return nil, diag.FromErr(err)
		}

		// Remove sensitive HTTP headers.
		recorder.AddFilter(func(i *cassette.Interaction) error {
			delete(i.Request.Headers, "Authorization")
			delete(i.Request.Headers, "X-Amz-Security-Token")

			return nil
		})

		// Defines how VCR will match requests to responses.
		recorder.SetMatcher(func(r *http.Request, i cassette.Request) bool {
			// Default matcher compares method and URL only.
			if !cassette.DefaultMatcher(r, i) {
				return false
			}

			if r.Body == nil {
				return true
			}

			contentType := r.Header.Get("Content-Type")
			// If body contains media, don't try to compare.
			if strings.Contains(contentType, "multipart/related") {
				return true
			}

			var b bytes.Buffer
			if _, err := b.ReadFrom(r.Body); err != nil {
				log.Printf("[DEBUG] Failed to read request body from cassette: %v", err)
				return false
			}

			r.Body = ioutil.NopCloser(&b)
			body := b.String()
			// If body matches identically, we are done.
			if body == i.Body {
				return true
			}

			// JSON might be the same, but reordered. Try parsing and comparing.
			if strings.Contains(contentType, "application/json") {
				var requestJson, cassetteJson interface{}

				if err := json.Unmarshal([]byte(body), &requestJson); err != nil {
					log.Printf("[DEBUG] Failed to unmarshal request JSON: %v", err)
					return false
				}

				if err := json.Unmarshal([]byte(i.Body), &cassetteJson); err != nil {
					log.Printf("[DEBUG] Failed to unmarshal cassette JSON: %v", err)
					return false
				}

				return reflect.DeepEqual(requestJson, cassetteJson)
			}

			return false
		})

		c.Session.Config.HTTPClient.Transport = recorder

		providerStatesLock.Lock()
		providerStates[testName] = v
		providerStatesLock.Unlock()

		return v, nil
	}
}

// vcrRandomnessSource returns a rand.Source for VCR testing.
// In RECORDING mode, generates a new seed and saves it to a file, using the seed for the source.
// In REPLAYING mode, reads a seed from a file and creates a source from it.
func vcrRandomnessSource(t *testing.T) (*randomnessSource, error) {
	testName := t.Name()

	randomnessSourcesLock.RLock()
	s, ok := randomnessSources[testName]
	randomnessSourcesLock.RUnlock()

	if ok {
		return s, nil
	}

	vcrMode, err := vcrMode()

	if err != nil {
		return nil, err
	}

	switch vcrMode {
	case recorder.ModeRecording:
		seed := rand.Int63()
		s = &randomnessSource{
			seed:   seed,
			source: rand.NewSource(seed),
		}
	case recorder.ModeReplaying:
		seed, err := readSeedFromFile(vcrSeedFile(os.Getenv(envVarVCRPath), testName))

		if err != nil {
			return nil, fmt.Errorf("no cassette found on disk for %s, please replay this testcase in recording mode - %w", testName, err)
		}

		s = &randomnessSource{
			seed:   seed,
			source: rand.NewSource(seed),
		}
	default:
		t.FailNow()
	}

	randomnessSourcesLock.Lock()
	randomnessSources[testName] = s
	randomnessSourcesLock.Unlock()

	return s, nil
}

func vcrFileName(name string) string {
	return strings.ReplaceAll(name, "/", "_")
}

func vcrSeedFile(path, name string) string {
	return filepath.Join(path, fmt.Sprintf("%s.seed", vcrFileName(name)))
}

func readSeedFromFile(fileName string) (int64, error) {
	// Max number of digits for int64 is 19.
	data := make([]byte, 19)
	f, err := os.Open(fileName)

	if err != nil {
		return 0, err
	}

	defer f.Close()

	_, err = f.Read(data)

	if err != nil {
		return 0, err
	}

	// Remove NULL characters from seed.
	return strconv.ParseInt(string(bytes.Trim(data, "\x00")), 10, 64)
}

func writeSeedToFile(seed int64, fileName string) error {
	f, err := os.Create(fileName)

	if err != nil {
		return err
	}

	defer f.Close()

	_, err = f.WriteString(strconv.FormatInt(seed, 10))

	if err != nil {
		return err
	}

	return nil
}

// closeVCRRecorder closes the VCR recorder, saving the cassette.
func closeVCRRecorder(t *testing.T) {
	testName := t.Name()
	providerStatesLock.RLock()
	v, ok := providerStates[testName]
	providerStatesLock.RUnlock()

	if ok {
		if !t.Failed() {
			log.Print("[DEBUG] closing VCR recorder")
			if err := v.(*conns.AWSClient).Session.Config.HTTPClient.Transport.(*recorder.Recorder).Stop(); err != nil {
				t.Error(err)
			}

			// Save the randomness seed.
			randomnessSourcesLock.RLock()
			s, ok := randomnessSources[testName]
			randomnessSourcesLock.RUnlock()

			if ok {
				if err := writeSeedToFile(s.seed, vcrSeedFile(os.Getenv(envVarVCRPath), t.Name())); err != nil {
					t.Error(err)
				}
			}
		}

		providerStatesLock.Lock()
		delete(providerStates, testName)
		providerStatesLock.Unlock()

		randomnessSourcesLock.Lock()
		delete(randomnessSources, testName)
		randomnessSourcesLock.Unlock()
	}
}

// ParallelTest wraps resource.ParallelTest, initializing VCR if enabled.
func ParallelTest(t *testing.T, c resource.TestCase) {
	if isVCREnabled() {
		log.Print("[DEBUG] initializing VCR")
		c.ProviderFactories = vcrEnabledProviderFactories(t, c.ProviderFactories)
		defer closeVCRRecorder(t)
	} else {
		log.Printf("[DEBUG] %s or %s not set, skipping VCR", envVarVCRMode, envVarVCRPath)
	}

	resource.ParallelTest(t, c)
}

// Test wraps resource.Test, initializing VCR if enabled.
func Test(t *testing.T, c resource.TestCase) {
	if isVCREnabled() {
		log.Print("[DEBUG] initializing VCR")
		c.ProviderFactories = vcrEnabledProviderFactories(t, c.ProviderFactories)
		defer closeVCRRecorder(t)
	} else {
		log.Printf("[DEBUG] %s or %s not set, skipping VCR", envVarVCRMode, envVarVCRPath)
	}

	resource.Test(t, c)
}

// RandInt is a VCR-friendly replacement for acctest.RandInt.
func RandInt(t *testing.T) int {
	if !isVCREnabled() {
		return sdkacctest.RandInt()
	}

	s, err := vcrRandomnessSource(t)

	if err != nil {
		t.Fatal(err)
	}

	return rand.New(s.source).Int()
}

// RandomWithPrefix is a VCR-friendly replacement for acctest.RandomWithPrefix.
func RandomWithPrefix(t *testing.T, prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, RandInt(t))
}
