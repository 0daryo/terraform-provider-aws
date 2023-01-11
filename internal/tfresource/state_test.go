package tfresource_test

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-provider-aws/internal/tfresource"
)

func failedStateRefreshFunc() resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		return nil, "", errors.New("failed")
	}
}

func timeoutStateRefreshFunc() resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		time.Sleep(100 * time.Second)
		return nil, "", errors.New("failed")
	}
}

func successfulStateRefreshFunc() resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		return struct{}{}, "running", nil
	}
}

type stateGenerator struct {
	position      int
	stateSequence []string
}

func (r *stateGenerator) NextState() (int, string, error) {
	p, v := r.position, ""
	if len(r.stateSequence)-1 >= p {
		v = r.stateSequence[p]
	} else {
		return -1, "", errors.New("No more states available")
	}

	r.position += 1

	return p, v, nil
}

func newStateGenerator(sequence []string) *stateGenerator {
	r := &stateGenerator{}
	r.stateSequence = sequence

	return r
}

func inconsistentStateRefreshFunc() resource.StateRefreshFunc {
	sequence := []string{
		"done", "replicating",
		"done", "done", "done",
		"replicating",
		"done", "done", "done",
	}

	r := newStateGenerator(sequence)

	return func() (interface{}, string, error) {
		idx, s, err := r.NextState()
		if err != nil {
			return nil, "", err
		}

		return idx, s, nil
	}
}

func unknownPendingStateRefreshFunc() resource.StateRefreshFunc {
	sequence := []string{
		"unknown1", "unknown2", "done",
	}

	r := newStateGenerator(sequence)

	return func() (interface{}, string, error) {
		idx, s, err := r.NextState()
		if err != nil {
			return nil, "", err
		}

		return idx, s, nil
	}
}

func TestWaitForState_inconsistent_positive(t *testing.T) {
	t.Parallel()

	conf := &tfresource.StateChangeConf{
		Pending:                   []string{"replicating"},
		Target:                    []string{"done"},
		Refresh:                   inconsistentStateRefreshFunc(),
		Timeout:                   90 * time.Millisecond,
		PollInterval:              10 * time.Millisecond,
		ContinuousTargetOccurence: 3,
	}

	idx, err := conf.WaitForStateContext(context.Background())

	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if idx != 4 {
		t.Fatalf("Expected index 4, given %d", idx.(int))
	}
}

func TestWaitForState_inconsistent_negative(t *testing.T) {
	t.Parallel()

	refreshCount := int64(0)
	f := inconsistentStateRefreshFunc()
	refresh := func() (interface{}, string, error) {
		atomic.AddInt64(&refreshCount, 1)
		return f()
	}

	conf := &tfresource.StateChangeConf{
		Pending:                   []string{"replicating"},
		Target:                    []string{"done"},
		Refresh:                   refresh,
		Timeout:                   85 * time.Millisecond,
		PollInterval:              10 * time.Millisecond,
		ContinuousTargetOccurence: 4,
	}

	_, err := conf.WaitForStateContext(context.Background())

	if err == nil {
		t.Fatal("Expected timeout error. No error returned.")
	}

	// we can't guarantee the exact number of refresh calls in the tests by
	// timing them, but we want to make sure the test at least went through th
	// required states.
	if atomic.LoadInt64(&refreshCount) < 6 {
		t.Fatal("refreshed called too few times")
	}

	expectedErr := "timeout while waiting for state to become 'done'"
	if !strings.HasPrefix(err.Error(), expectedErr) {
		t.Fatalf("error prefix doesn't match.\nExpected: %q\nGiven: %q\n", expectedErr, err.Error())
	}
}

func TestWaitForState_timeout(t *testing.T) {
	t.Parallel()

	old := tfresource.RefreshGracePeriod
	tfresource.RefreshGracePeriod = 5 * time.Millisecond
	defer func() {
		tfresource.RefreshGracePeriod = old
	}()

	conf := &tfresource.StateChangeConf{
		Pending: []string{"pending", "incomplete"},
		Target:  []string{"running"},
		Refresh: timeoutStateRefreshFunc(),
		Timeout: 1 * time.Millisecond,
	}

	obj, err := conf.WaitForStateContext(context.Background())

	if err == nil {
		t.Fatal("Expected timeout error. No error returned.")
	}

	expectedErr := "timeout while waiting for state to become 'running' (timeout: 1ms)"
	if err.Error() != expectedErr {
		t.Fatalf("Errors don't match.\nExpected: %q\nGiven: %q\n", expectedErr, err.Error())
	}

	if obj != nil {
		t.Fatalf("should not return obj")
	}
}

// Make sure a timeout actually cancels the refresh goroutine and waits for its
// return.
func TestWaitForState_cancel(t *testing.T) {
	t.Parallel()

	// make this refresh func block until we cancel it
	cancel := make(chan struct{})
	refresh := func() (interface{}, string, error) {
		<-cancel
		return nil, "pending", nil
	}
	conf := &tfresource.StateChangeConf{
		Pending:      []string{"pending", "incomplete"},
		Target:       []string{"running"},
		Refresh:      refresh,
		Timeout:      10 * time.Millisecond,
		PollInterval: 10 * time.Second,
	}

	var obj interface{}
	var err error

	waitDone := make(chan struct{})
	go func() {
		defer close(waitDone)
		obj, err = conf.WaitForStateContext(context.Background())
	}()

	// make sure WaitForState is blocked
	select {
	case <-waitDone:
		t.Fatal("WaitForState returned too early")
	case <-time.After(10 * time.Millisecond):
	}

	// unlock the refresh function
	close(cancel)
	// make sure WaitForState returns
	select {
	case <-waitDone:
	case <-time.After(time.Second):
		t.Fatal("WaitForState didn't return after refresh finished")
	}

	if err == nil {
		t.Fatal("Expected timeout error. No error returned.")
	}

	expectedErr := "timeout while waiting for state to become 'running'"
	if !strings.HasPrefix(err.Error(), expectedErr) {
		t.Fatalf("Errors don't match.\nExpected: %q\nGiven: %q\n", expectedErr, err.Error())
	}

	if obj != nil {
		t.Fatalf("should not return obj")
	}
}

func TestWaitForState_success(t *testing.T) {
	t.Parallel()

	conf := &tfresource.StateChangeConf{
		Pending: []string{"pending", "incomplete"},
		Target:  []string{"running"},
		Refresh: successfulStateRefreshFunc(),
		Timeout: 200 * time.Second,
	}

	obj, err := conf.WaitForStateContext(context.Background())
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if obj == nil {
		t.Fatalf("should return obj")
	}
}

func TestWaitForState_successUnknownPending(t *testing.T) {
	t.Parallel()

	conf := &tfresource.StateChangeConf{
		Target:  []string{"done"},
		Refresh: unknownPendingStateRefreshFunc(),
		Timeout: 200 * time.Second,
	}

	obj, err := conf.WaitForStateContext(context.Background())
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if obj == nil {
		t.Fatalf("should return obj")
	}
}

func TestWaitForState_successEmpty(t *testing.T) {
	t.Parallel()

	conf := &tfresource.StateChangeConf{
		Pending: []string{"pending", "incomplete"},
		Target:  []string{},
		Refresh: func() (interface{}, string, error) {
			return nil, "", nil
		},
		Timeout: 200 * time.Second,
	}

	obj, err := conf.WaitForStateContext(context.Background())
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if obj != nil {
		t.Fatalf("obj should be nil")
	}
}

func TestWaitForState_failureEmpty(t *testing.T) {
	t.Parallel()

	conf := &tfresource.StateChangeConf{
		Pending:        []string{"pending", "incomplete"},
		Target:         []string{},
		NotFoundChecks: 1,
		Refresh: func() (interface{}, string, error) {
			return 42, "pending", nil
		},
		PollInterval: 10 * time.Millisecond,
		Timeout:      100 * time.Millisecond,
	}

	_, err := conf.WaitForStateContext(context.Background())
	if err == nil {
		t.Fatal("Expected timeout error. Got none.")
	}
	expectedErr := "timeout while waiting for resource to be gone (last state: 'pending', timeout: 100ms)"
	if err.Error() != expectedErr {
		t.Fatalf("Errors don't match.\nExpected: %q\nGiven: %q\n", expectedErr, err.Error())
	}
}

func TestWaitForState_failure(t *testing.T) {
	t.Parallel()

	conf := &tfresource.StateChangeConf{
		Pending: []string{"pending", "incomplete"},
		Target:  []string{"running"},
		Refresh: failedStateRefreshFunc(),
		Timeout: 200 * time.Second,
	}

	obj, err := conf.WaitForStateContext(context.Background())
	if err == nil {
		t.Fatal("Expected error. No error returned.")
	}
	expectedErr := "failed"
	if err.Error() != expectedErr {
		t.Fatalf("Errors don't match.\nExpected: %q\nGiven: %q\n", expectedErr, err.Error())
	}
	if obj != nil {
		t.Fatalf("should not return obj")
	}
}

func TestWaitForStateContext_cancel(t *testing.T) {
	t.Parallel()

	// make this refresh func block until we cancel it
	ctx, cancel := context.WithCancel(context.Background())
	refresh := func() (interface{}, string, error) {
		<-ctx.Done()
		return nil, "pending", nil
	}
	conf := &tfresource.StateChangeConf{
		Pending: []string{"pending", "incomplete"},
		Target:  []string{"running"},
		Refresh: refresh,
		Timeout: 10 * time.Second,
	}

	var err error

	waitDone := make(chan struct{})
	go func() {
		defer close(waitDone)
		_, err = conf.WaitForStateContext(ctx)
	}()

	// make sure WaitForState is blocked
	select {
	case <-waitDone:
		t.Fatal("WaitForState returned too early")
	case <-time.After(10 * time.Millisecond):
	}

	// unlock the refresh function
	cancel()
	// make sure WaitForState returns
	select {
	case <-waitDone:
	case <-time.After(time.Second):
		t.Fatal("WaitForState didn't return after refresh finished")
	}

	if err != context.Canceled {
		t.Fatalf("Expected canceled context error, got: %s", err)
	}
}
