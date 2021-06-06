package aws

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	awspolicy "github.com/jen20/awspolicyequivalence"
	"github.com/terraform-providers/terraform-provider-aws/aws/internal/naming"
	tfsqs "github.com/terraform-providers/terraform-provider-aws/aws/internal/service/sqs"
	"github.com/terraform-providers/terraform-provider-aws/aws/internal/service/sqs/finder"
	"github.com/terraform-providers/terraform-provider-aws/aws/internal/tfresource"
)

func init() {
	resource.AddTestSweepers("aws_sqs_queue", &resource.Sweeper{
		Name: "aws_sqs_queue",
		F:    testSweepSqsQueues,
		Dependencies: []string{
			"aws_autoscaling_group",
			"aws_cloudwatch_event_rule",
			"aws_elastic_beanstalk_environment",
			"aws_iot_topic_rule",
			"aws_lambda_function",
			"aws_s3_bucket",
			"aws_sns_topic",
		},
	})
}

func testSweepSqsQueues(region string) error {
	client, err := sharedClientForRegion(region)
	if err != nil {
		return fmt.Errorf("error getting client: %w", err)
	}
	conn := client.(*AWSClient).sqsconn
	input := &sqs.ListQueuesInput{}
	var sweeperErrs *multierror.Error

	err = conn.ListQueuesPages(input, func(page *sqs.ListQueuesOutput, lastPage bool) bool {
		if page == nil {
			return !lastPage
		}

		for _, queueUrl := range page.QueueUrls {
			r := resourceAwsSqsQueue()
			d := r.Data(nil)
			d.SetId(aws.StringValue(queueUrl))
			err = r.Delete(d, client)

			if err != nil {
				log.Printf("[ERROR] %s", err)
				sweeperErrs = multierror.Append(sweeperErrs, err)
				continue
			}
		}

		return !lastPage
	})

	if testSweepSkipSweepError(err) {
		log.Printf("[WARN] Skipping SQS Queue sweep for %s: %s", region, err)
		return sweeperErrs.ErrorOrNil() // In case we have completed some pages, but had errors
	}

	if err != nil {
		sweeperErrs = multierror.Append(sweeperErrs, fmt.Errorf("error listing SQS Queues: %w", err))
	}

	return sweeperErrs.ErrorOrNil()
}

func TestAccAWSSQSQueue_basic(t *testing.T) {
	var queueAttributes map[string]string
	resourceName := "aws_sqs_queue.queue"
	rName := acctest.RandomWithPrefix("tf-acc-test")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		ErrorCheck:   testAccErrorCheck(t, sqs.EndpointsID),
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSSQSQueueDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSSQSConfigName(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckAWSSQSQueueExists(resourceName, &queueAttributes),
					testAccCheckResourceAttrRegionalARN(resourceName, "arn", "sqs", rName),
					resource.TestCheckResourceAttr(resourceName, "content_based_deduplication", "false"),
					resource.TestCheckResourceAttr(resourceName, "deduplication_scope", ""),
					resource.TestCheckResourceAttr(resourceName, "delay_seconds", strconv.Itoa(tfsqs.DefaultQueueDelaySeconds)),
					resource.TestCheckResourceAttr(resourceName, "fifo_queue", "false"),
					resource.TestCheckResourceAttr(resourceName, "fifo_throughput_limit", ""),
					resource.TestCheckResourceAttr(resourceName, "kms_data_key_reuse_period_seconds", strconv.Itoa(tfsqs.DefaultQueueKmsDataKeyReusePeriodSeconds)),
					resource.TestCheckResourceAttr(resourceName, "kms_master_key_id", ""),
					resource.TestCheckResourceAttr(resourceName, "max_message_size", strconv.Itoa(tfsqs.DefaultQueueMaximumMessageSize)),
					resource.TestCheckResourceAttr(resourceName, "message_retention_seconds", strconv.Itoa(tfsqs.DefaultQueueMessageRetentionPeriod)),
					resource.TestCheckResourceAttr(resourceName, "name", rName),
					resource.TestCheckResourceAttr(resourceName, "name_prefix", ""),
					resource.TestCheckResourceAttr(resourceName, "policy", ""),
					resource.TestCheckResourceAttr(resourceName, "receive_wait_time_seconds", strconv.Itoa(tfsqs.DefaultQueueReceiveMessageWaitTimeSeconds)),
					resource.TestCheckResourceAttr(resourceName, "redrive_policy", ""),
					resource.TestCheckResourceAttr(resourceName, "tags.%", "0"),
					resource.TestCheckResourceAttr(resourceName, "visibility_timeout_seconds", strconv.Itoa(tfsqs.DefaultQueueVisibilityTimeout)),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccAWSSQSConfigWithOverrides(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSSQSQueueExists(resourceName, &queueAttributes),
					resource.TestCheckResourceAttr(resourceName, "delay_seconds", "90"),
					resource.TestCheckResourceAttr(resourceName, "max_message_size", "2048"),
					resource.TestCheckResourceAttr(resourceName, "message_retention_seconds", "86400"),
					resource.TestCheckResourceAttr(resourceName, "receive_wait_time_seconds", "10"),
					resource.TestCheckResourceAttr(resourceName, "visibility_timeout_seconds", "60"),
				),
			},
			{
				Config: testAccAWSSQSConfigName(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSSQSQueueExists(resourceName, &queueAttributes),
					resource.TestCheckResourceAttr(resourceName, "delay_seconds", strconv.Itoa(tfsqs.DefaultQueueDelaySeconds)),
					resource.TestCheckResourceAttr(resourceName, "max_message_size", strconv.Itoa(tfsqs.DefaultQueueMaximumMessageSize)),
					resource.TestCheckResourceAttr(resourceName, "message_retention_seconds", strconv.Itoa(tfsqs.DefaultQueueMessageRetentionPeriod)),
					resource.TestCheckResourceAttr(resourceName, "receive_wait_time_seconds", strconv.Itoa(tfsqs.DefaultQueueReceiveMessageWaitTimeSeconds)),
					resource.TestCheckResourceAttr(resourceName, "visibility_timeout_seconds", strconv.Itoa(tfsqs.DefaultQueueVisibilityTimeout)),
				),
			},
		},
	})
}

func TestAccAWSSQSQueue_disappears(t *testing.T) {
	var queueAttributes map[string]string
	resourceName := "aws_sqs_queue.queue"
	rName := acctest.RandomWithPrefix("tf-acc-test")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		ErrorCheck:   testAccErrorCheck(t, sqs.EndpointsID),
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSSQSQueueDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSSQSConfigName(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSSQSQueueExists(resourceName, &queueAttributes),
					testAccCheckResourceDisappears(testAccProvider, resourceAwsSqsQueue(), resourceName),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestAccAWSSQSQueue_Tags(t *testing.T) {
	var queueAttributes map[string]string
	resourceName := "aws_sqs_queue.queue"
	rName := acctest.RandomWithPrefix("tf-acc-test")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		ErrorCheck:   testAccErrorCheck(t, sqs.EndpointsID),
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSSQSQueueDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSSQSConfigTags1(rName, "key1", "value1"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSSQSQueueExists(resourceName, &queueAttributes),
					resource.TestCheckResourceAttr(resourceName, "tags.%", "1"),
					resource.TestCheckResourceAttr(resourceName, "tags.key1", "value1"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccAWSSQSConfigTags2(rName, "key1", "value1updated", "key2", "value2"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSSQSQueueExists(resourceName, &queueAttributes),
					resource.TestCheckResourceAttr(resourceName, "tags.%", "2"),
					resource.TestCheckResourceAttr(resourceName, "tags.key1", "value1updated"),
					resource.TestCheckResourceAttr(resourceName, "tags.key2", "value2"),
				),
			},
			{
				Config: testAccAWSSQSConfigTags1(rName, "key2", "value2"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSSQSQueueExists(resourceName, &queueAttributes),
					resource.TestCheckResourceAttr(resourceName, "tags.%", "1"),
					resource.TestCheckResourceAttr(resourceName, "tags.key2", "value2"),
				),
			},
		},
	})
}

func TestAccAWSSQSQueue_Name_Generated(t *testing.T) {
	var queueAttributes map[string]string
	resourceName := "aws_sqs_queue.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		ErrorCheck:   testAccErrorCheck(t, sqs.EndpointsID),
		CheckDestroy: testAccCheckAWSSQSQueueDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSSQSQueueConfigNameGenerated,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSSQSQueueExists(resourceName, &queueAttributes),
					naming.TestCheckResourceAttrNameGenerated(resourceName, "name"),
					resource.TestCheckResourceAttr(resourceName, "name_prefix", "terraform-"),
					resource.TestCheckResourceAttr(resourceName, "delay_seconds", strconv.Itoa(tfsqs.DefaultQueueDelaySeconds)),
					resource.TestCheckResourceAttr(resourceName, "fifo_queue", "false"),
					resource.TestCheckResourceAttr(resourceName, "max_message_size", strconv.Itoa(tfsqs.DefaultQueueMaximumMessageSize)),
					resource.TestCheckResourceAttr(resourceName, "message_retention_seconds", strconv.Itoa(tfsqs.DefaultQueueMessageRetentionPeriod)),
					resource.TestCheckResourceAttr(resourceName, "receive_wait_time_seconds", strconv.Itoa(tfsqs.DefaultQueueReceiveMessageWaitTimeSeconds)),
					resource.TestCheckResourceAttr(resourceName, "visibility_timeout_seconds", strconv.Itoa(tfsqs.DefaultQueueVisibilityTimeout)),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccAWSSQSQueue_Name_Generated_FIFOQueue(t *testing.T) {
	var queueAttributes map[string]string
	resourceName := "aws_sqs_queue.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		ErrorCheck:   testAccErrorCheck(t, sqs.EndpointsID),
		CheckDestroy: testAccCheckAWSSQSQueueDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSSQSQueueConfigNameGeneratedFIFOQueue,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSSQSQueueExists(resourceName, &queueAttributes),
					naming.TestCheckResourceAttrNameWithSuffixGenerated(resourceName, "name", tfsqs.FifoQueueNameSuffix),
					resource.TestCheckResourceAttr(resourceName, "name_prefix", "terraform-"),
					resource.TestCheckResourceAttr(resourceName, "fifo_queue", "true"),
					resource.TestCheckResourceAttr(resourceName, "delay_seconds", strconv.Itoa(tfsqs.DefaultQueueDelaySeconds)),
					resource.TestCheckResourceAttr(resourceName, "max_message_size", strconv.Itoa(tfsqs.DefaultQueueMaximumMessageSize)),
					resource.TestCheckResourceAttr(resourceName, "message_retention_seconds", strconv.Itoa(tfsqs.DefaultQueueMessageRetentionPeriod)),
					resource.TestCheckResourceAttr(resourceName, "receive_wait_time_seconds", strconv.Itoa(tfsqs.DefaultQueueReceiveMessageWaitTimeSeconds)),
					resource.TestCheckResourceAttr(resourceName, "visibility_timeout_seconds", strconv.Itoa(tfsqs.DefaultQueueVisibilityTimeout)),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccAWSSQSQueue_NamePrefix(t *testing.T) {
	var queueAttributes map[string]string
	resourceName := "aws_sqs_queue.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		ErrorCheck:   testAccErrorCheck(t, sqs.EndpointsID),
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSSQSQueueDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSSQSQueueConfigNamePrefix("tf-acc-test-prefix-"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSSQSQueueExists(resourceName, &queueAttributes),
					naming.TestCheckResourceAttrNameFromPrefix(resourceName, "name", "tf-acc-test-prefix-"),
					resource.TestCheckResourceAttr(resourceName, "name_prefix", "tf-acc-test-prefix-"),
					resource.TestCheckResourceAttr(resourceName, "fifo_queue", "false"),
					resource.TestCheckResourceAttr(resourceName, "delay_seconds", strconv.Itoa(tfsqs.DefaultQueueDelaySeconds)),
					resource.TestCheckResourceAttr(resourceName, "max_message_size", strconv.Itoa(tfsqs.DefaultQueueMaximumMessageSize)),
					resource.TestCheckResourceAttr(resourceName, "message_retention_seconds", strconv.Itoa(tfsqs.DefaultQueueMessageRetentionPeriod)),
					resource.TestCheckResourceAttr(resourceName, "receive_wait_time_seconds", strconv.Itoa(tfsqs.DefaultQueueReceiveMessageWaitTimeSeconds)),
					resource.TestCheckResourceAttr(resourceName, "visibility_timeout_seconds", strconv.Itoa(tfsqs.DefaultQueueVisibilityTimeout)),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccAWSSQSQueue_NamePrefix_FIFOQueue(t *testing.T) {
	var queueAttributes map[string]string
	resourceName := "aws_sqs_queue.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		ErrorCheck:   testAccErrorCheck(t, sqs.EndpointsID),
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSSQSQueueDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSSQSQueueConfigNamePrefixFIFOQueue("tf-acc-test-prefix-"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSSQSQueueExists(resourceName, &queueAttributes),
					naming.TestCheckResourceAttrNameWithSuffixFromPrefix(resourceName, "name", "tf-acc-test-prefix-", tfsqs.FifoQueueNameSuffix),
					resource.TestCheckResourceAttr(resourceName, "name_prefix", "tf-acc-test-prefix-"),
					resource.TestCheckResourceAttr(resourceName, "fifo_queue", "true"),
					resource.TestCheckResourceAttr(resourceName, "delay_seconds", strconv.Itoa(tfsqs.DefaultQueueDelaySeconds)),
					resource.TestCheckResourceAttr(resourceName, "max_message_size", strconv.Itoa(tfsqs.DefaultQueueMaximumMessageSize)),
					resource.TestCheckResourceAttr(resourceName, "message_retention_seconds", strconv.Itoa(tfsqs.DefaultQueueMessageRetentionPeriod)),
					resource.TestCheckResourceAttr(resourceName, "receive_wait_time_seconds", strconv.Itoa(tfsqs.DefaultQueueReceiveMessageWaitTimeSeconds)),
					resource.TestCheckResourceAttr(resourceName, "visibility_timeout_seconds", strconv.Itoa(tfsqs.DefaultQueueVisibilityTimeout)),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccAWSSQSQueue_policy(t *testing.T) {
	var queueAttributes map[string]string
	resourceName := "aws_sqs_queue.test-email-events"
	queueName := fmt.Sprintf("sqs-queue-%s", acctest.RandString(10))
	topicName := fmt.Sprintf("sns-topic-%s", acctest.RandString(10))

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		ErrorCheck:   testAccErrorCheck(t, sqs.EndpointsID),
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSSQSQueueDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSSQSConfig_PolicyFormat(topicName, queueName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSSQSQueueExists(resourceName, &queueAttributes),
					testAccCheckAWSSQSQueuePolicyAttribute(&queueAttributes, topicName, queueName),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccAWSSQSQueue_queueDeletedRecently(t *testing.T) {
	var queueAttributes map[string]string
	resourceName := "aws_sqs_queue.queue"
	queueName := fmt.Sprintf("sqs-queue-%s", acctest.RandString(10))

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		ErrorCheck:   testAccErrorCheck(t, sqs.EndpointsID),
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSSQSQueueDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSSQSConfigName(queueName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSSQSQueueExists(resourceName, &queueAttributes),
					resource.TestCheckResourceAttr(resourceName, "delay_seconds", strconv.Itoa(tfsqs.DefaultQueueDelaySeconds)),
					resource.TestCheckResourceAttr(resourceName, "max_message_size", strconv.Itoa(tfsqs.DefaultQueueMaximumMessageSize)),
					resource.TestCheckResourceAttr(resourceName, "message_retention_seconds", strconv.Itoa(tfsqs.DefaultQueueMessageRetentionPeriod)),
					resource.TestCheckResourceAttr(resourceName, "receive_wait_time_seconds", strconv.Itoa(tfsqs.DefaultQueueReceiveMessageWaitTimeSeconds)),
					resource.TestCheckResourceAttr(resourceName, "visibility_timeout_seconds", strconv.Itoa(tfsqs.DefaultQueueVisibilityTimeout)),
				),
			},
			{
				Config: testAccAWSSQSConfigName(queueName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSSQSQueueExists(resourceName, &queueAttributes),
					resource.TestCheckResourceAttr(resourceName, "delay_seconds", strconv.Itoa(tfsqs.DefaultQueueDelaySeconds)),
					resource.TestCheckResourceAttr(resourceName, "max_message_size", strconv.Itoa(tfsqs.DefaultQueueMaximumMessageSize)),
					resource.TestCheckResourceAttr(resourceName, "message_retention_seconds", strconv.Itoa(tfsqs.DefaultQueueMessageRetentionPeriod)),
					resource.TestCheckResourceAttr(resourceName, "receive_wait_time_seconds", strconv.Itoa(tfsqs.DefaultQueueReceiveMessageWaitTimeSeconds)),
					resource.TestCheckResourceAttr(resourceName, "visibility_timeout_seconds", strconv.Itoa(tfsqs.DefaultQueueVisibilityTimeout)),
				),
				Taint: []string{resourceName},
			},
		},
	})
}

func TestAccAWSSQSQueue_redrivePolicy(t *testing.T) {
	var queueAttributes map[string]string
	resourceName := "aws_sqs_queue.my_dead_letter_queue"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		ErrorCheck:   testAccErrorCheck(t, sqs.EndpointsID),
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSSQSQueueDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSSQSConfigWithRedrive(acctest.RandString(10)),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSSQSQueueExists(resourceName, &queueAttributes),
					resource.TestCheckResourceAttr(resourceName, "delay_seconds", strconv.Itoa(tfsqs.DefaultQueueDelaySeconds)),
					resource.TestCheckResourceAttr(resourceName, "max_message_size", strconv.Itoa(tfsqs.DefaultQueueMaximumMessageSize)),
					resource.TestCheckResourceAttr(resourceName, "message_retention_seconds", strconv.Itoa(tfsqs.DefaultQueueMessageRetentionPeriod)),
					resource.TestCheckResourceAttr(resourceName, "receive_wait_time_seconds", strconv.Itoa(tfsqs.DefaultQueueReceiveMessageWaitTimeSeconds)),
					resource.TestCheckResourceAttr(resourceName, "visibility_timeout_seconds", strconv.Itoa(tfsqs.DefaultQueueVisibilityTimeout)),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// Tests formatting and compacting of Policy, Redrive json
func TestAccAWSSQSQueue_Policybasic(t *testing.T) {
	var queueAttributes map[string]string
	resourceName := "aws_sqs_queue.test-email-events"
	queueName := fmt.Sprintf("sqs-queue-%s", acctest.RandString(10))
	topicName := fmt.Sprintf("sns-topic-%s", acctest.RandString(10))

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		ErrorCheck:   testAccErrorCheck(t, sqs.EndpointsID),
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSSQSQueueDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSSQSConfig_PolicyFormat(topicName, queueName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSSQSQueueExists(resourceName, &queueAttributes),
					resource.TestCheckResourceAttr(resourceName, "delay_seconds", "90"),
					resource.TestCheckResourceAttr(resourceName, "max_message_size", "2048"),
					resource.TestCheckResourceAttr(resourceName, "message_retention_seconds", "86400"),
					resource.TestCheckResourceAttr(resourceName, "receive_wait_time_seconds", "10"),
					resource.TestCheckResourceAttr(resourceName, "visibility_timeout_seconds", "60"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccAWSSQSQueue_FIFO(t *testing.T) {
	var queueAttributes map[string]string
	resourceName := "aws_sqs_queue.queue"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		ErrorCheck:   testAccErrorCheck(t, sqs.EndpointsID),
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSSQSQueueDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSSQSConfigWithFIFO(acctest.RandString(10)),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSSQSQueueExists(resourceName, &queueAttributes),
					resource.TestCheckResourceAttr(resourceName, "fifo_queue", "true"),
					resource.TestCheckResourceAttr(resourceName, "deduplication_scope", "queue"),
					resource.TestCheckResourceAttr(resourceName, "fifo_throughput_limit", "perQueue"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccAWSSQSQueue_FIFOExpectNameError(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		ErrorCheck:   testAccErrorCheck(t, sqs.EndpointsID),
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSSQSQueueDestroy,
		Steps: []resource.TestStep{
			{
				Config:      testAccAWSSQSConfigWithFIFOExpectError(acctest.RandString(10)),
				ExpectError: regexp.MustCompile(`invalid queue name:`),
			},
		},
	})
}

func TestAccAWSSQSQueue_FIFOWithContentBasedDeduplication(t *testing.T) {
	var queueAttributes map[string]string
	resourceName := "aws_sqs_queue.queue"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		ErrorCheck:   testAccErrorCheck(t, sqs.EndpointsID),
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSSQSQueueDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSSQSConfigWithFIFOContentBasedDeduplication(acctest.RandString(10)),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSSQSQueueExists(resourceName, &queueAttributes),
					resource.TestCheckResourceAttr(resourceName, "fifo_queue", "true"),
					resource.TestCheckResourceAttr(resourceName, "content_based_deduplication", "true"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccAWSSQSQueue_FIFOWithHighThroughputMode(t *testing.T) {
	var queueAttributes map[string]string

	resourceName := "aws_sqs_queue.queue"
	queueName := acctest.RandString(10)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		ErrorCheck:   testAccErrorCheck(t, sqs.EndpointsID),
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSSQSQueueDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSSQSConfigWithFIFOHighThroughputMode1(queueName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSSQSQueueExists(resourceName, &queueAttributes),
					resource.TestCheckResourceAttr(resourceName, "fifo_queue", "true"),
					resource.TestCheckResourceAttr(resourceName, "deduplication_scope", "queue"),
					resource.TestCheckResourceAttr(resourceName, "fifo_throughput_limit", "perQueue"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccAWSSQSConfigWithFIFOHighThroughputMode2(queueName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSSQSQueueExists(resourceName, &queueAttributes),
					resource.TestCheckResourceAttr(resourceName, "deduplication_scope", "messageGroup"),
					resource.TestCheckResourceAttr(resourceName, "fifo_throughput_limit", "perMessageGroupId"),
				),
			},
		},
	})
}

func TestAccAWSSQSQueue_ExpectContentBasedDeduplicationError(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		ErrorCheck:   testAccErrorCheck(t, sqs.EndpointsID),
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSSQSQueueDestroy,
		Steps: []resource.TestStep{
			{
				Config:      testAccExpectContentBasedDeduplicationError(acctest.RandString(10)),
				ExpectError: regexp.MustCompile(`content-based deduplication can only be set for FIFO queue`),
			},
		},
	})
}

func TestAccAWSSQSQueue_Encryption(t *testing.T) {
	var queueAttributes map[string]string
	resourceName := "aws_sqs_queue.queue"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		ErrorCheck:   testAccErrorCheck(t, sqs.EndpointsID),
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSSQSQueueDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSSQSConfigWithEncryption(acctest.RandString(10)),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSSQSQueueExists(resourceName, &queueAttributes),
					resource.TestCheckResourceAttr(resourceName, "kms_master_key_id", "alias/aws/sqs"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccCheckAWSSQSQueuePolicyAttribute(queueAttributes *map[string]string, topicName, queueName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		accountID := testAccProvider.Meta().(*AWSClient).accountid

		expectedPolicyFormat := `{"Version": "2012-10-17","Id": "sqspolicy","Statement":[{"Sid": "Stmt1451501026839","Effect": "Allow","Principal":"*","Action":"sqs:SendMessage","Resource":"arn:%[1]s:sqs:%[2]s:%[3]s:%[4]s","Condition":{"ArnEquals":{"aws:SourceArn":"arn:%[1]s:sns:%[2]s:%[3]s:%[5]s"}}}]}`
		expectedPolicyText := fmt.Sprintf(expectedPolicyFormat, testAccGetPartition(), testAccGetRegion(), accountID, topicName, queueName)

		var actualPolicyText string
		for key, value := range *queueAttributes {
			if key == sqs.QueueAttributeNamePolicy {
				actualPolicyText = value
				break
			}
		}

		equivalent, err := awspolicy.PoliciesAreEquivalent(actualPolicyText, expectedPolicyText)
		if err != nil {
			return fmt.Errorf("Error testing policy equivalence: %s", err)
		}
		if !equivalent {
			return fmt.Errorf("Non-equivalent policy error:\n\nexpected: %s\n\n     got: %s\n",
				expectedPolicyText, actualPolicyText)
		}

		return nil
	}
}

func testAccCheckAWSSQSQueueExists(resourceName string, v *map[string]string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", resourceName)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("No SQS Queue URL is set")
		}

		conn := testAccProvider.Meta().(*AWSClient).sqsconn

		output, err := finder.QueueAttributesByURL(conn, rs.Primary.ID)

		if err != nil {
			return err
		}

		*v = output

		return nil
	}
}

func testAccCheckAWSSQSQueueDestroy(s *terraform.State) error {
	conn := testAccProvider.Meta().(*AWSClient).sqsconn

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "aws_sqs_queue" {
			continue
		}

		_, err := finder.QueueAttributesByURL(conn, rs.Primary.ID)

		if tfresource.NotFound(err) {
			continue
		}

		if err != nil {
			return err
		}

		return fmt.Errorf("SQS Queue %s still exists", rs.Primary.ID)
	}

	return nil
}

const testAccAWSSQSQueueConfigNameGenerated = `
resource "aws_sqs_queue" "test" {}
`

const testAccAWSSQSQueueConfigNameGeneratedFIFOQueue = `
resource "aws_sqs_queue" "test" {
  fifo_queue = true
}
`

func testAccAWSSQSConfigName(rName string) string {
	return fmt.Sprintf(`
resource "aws_sqs_queue" "queue" {
  name = %[1]q
}
`, rName)
}

func testAccAWSSQSQueueConfigNamePrefix(prefix string) string {
	return fmt.Sprintf(`
resource "aws_sqs_queue" "test" {
  name_prefix = %[1]q
}
`, prefix)
}

func testAccAWSSQSQueueConfigNamePrefixFIFOQueue(prefix string) string {
	return fmt.Sprintf(`
resource "aws_sqs_queue" "test" {
  name_prefix = %[1]q
  fifo_queue  = true
}
`, prefix)
}

func testAccAWSSQSConfigTags1(rName, tagKey1, tagValue1 string) string {
	return fmt.Sprintf(`
resource "aws_sqs_queue" "queue" {
  name = %[1]q

  tags = {
    %[2]q = %[3]q
  }
}
`, rName, tagKey1, tagValue1)
}

func testAccAWSSQSConfigTags2(rName, tagKey1, tagValue1, tagKey2, tagValue2 string) string {
	return fmt.Sprintf(`
resource "aws_sqs_queue" "queue" {
  name = %[1]q

  tags = {
    %[2]q = %[3]q
    %[4]q = %[5]q
  }
}
`, rName, tagKey1, tagValue1, tagKey2, tagValue2)
}

func testAccAWSSQSConfigWithOverrides(rName string) string {
	return fmt.Sprintf(`
resource "aws_sqs_queue" "queue" {
  name                       = %[1]q
  delay_seconds              = 90
  max_message_size           = 2048
  message_retention_seconds  = 86400
  receive_wait_time_seconds  = 10
  visibility_timeout_seconds = 60
}
`, rName)
}

func testAccAWSSQSConfigWithRedrive(name string) string {
	return fmt.Sprintf(`
resource "aws_sqs_queue" "my_queue" {
  name                       = "tftestqueuq-%[1]s"
  delay_seconds              = 0
  visibility_timeout_seconds = 300

  redrive_policy = <<EOF
{
  "maxReceiveCount": 3,
  "deadLetterTargetArn": "${aws_sqs_queue.my_dead_letter_queue.arn}"
}
EOF
}

resource "aws_sqs_queue" "my_dead_letter_queue" {
  name = "tfotherqueuq-%[1]s"
}
`, name)
}

func testAccAWSSQSConfig_PolicyFormat(queue, topic string) string {
	return fmt.Sprintf(`
variable "sns_name" {
  default = "%s"
}

variable "sqs_name" {
  default = "%s"
}

resource "aws_sns_topic" "test_topic" {
  name = var.sns_name
}

data "aws_partition" "current" {}

data "aws_region" "current" {}

data "aws_caller_identity" "current" {}

resource "aws_sqs_queue" "test-email-events" {
  name                       = var.sqs_name
  depends_on                 = [aws_sns_topic.test_topic]
  delay_seconds              = 90
  max_message_size           = 2048
  message_retention_seconds  = 86400
  receive_wait_time_seconds  = 10
  visibility_timeout_seconds = 60

  policy = <<EOF
{
  "Version": "2012-10-17",
  "Id": "sqspolicy",
  "Statement": [
    {
      "Sid": "Stmt1451501026839",
      "Effect": "Allow",
      "Principal": "*",
      "Action": "sqs:SendMessage",
      "Resource": "arn:${data.aws_partition.current.partition}:sqs:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:${var.sqs_name}",
      "Condition": {
        "ArnEquals": {
          "aws:SourceArn": "arn:${data.aws_partition.current.partition}:sns:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:${var.sns_name}"
        }
      }
    }
  ]
}
EOF
}

resource "aws_sns_topic_subscription" "test_queue_target" {
  topic_arn = aws_sns_topic.test_topic.arn
  protocol  = "sqs"
  endpoint  = aws_sqs_queue.test-email-events.arn
}
`, topic, queue)
}

func testAccAWSSQSConfigWithFIFO(queue string) string {
	return fmt.Sprintf(`
resource "aws_sqs_queue" "queue" {
  name       = "%s.fifo"
  fifo_queue = true
}
`, queue)
}

func testAccAWSSQSConfigWithFIFOContentBasedDeduplication(queue string) string {
	return fmt.Sprintf(`
resource "aws_sqs_queue" "queue" {
  name                        = "%s.fifo"
  fifo_queue                  = true
  content_based_deduplication = true
}
`, queue)
}

func testAccAWSSQSConfigWithFIFOHighThroughputMode1(queue string) string {
	return fmt.Sprintf(`
resource "aws_sqs_queue" "queue" {
  name       = "%s.fifo"
  fifo_queue = true
}
`, queue)
}

func testAccAWSSQSConfigWithFIFOHighThroughputMode2(queue string) string {
	return fmt.Sprintf(`
resource "aws_sqs_queue" "queue" {
  name                  = "%s.fifo"
  fifo_queue            = true
  deduplication_scope   = "messageGroup"
  fifo_throughput_limit = "perMessageGroupId"
}
`, queue)
}

func testAccAWSSQSConfigWithFIFOExpectError(queue string) string {
	return fmt.Sprintf(`
resource "aws_sqs_queue" "queue" {
  name       = "%s"
  fifo_queue = true
}
`, queue)
}

func testAccExpectContentBasedDeduplicationError(queue string) string {
	return fmt.Sprintf(`
resource "aws_sqs_queue" "queue" {
  name                        = "%s"
  content_based_deduplication = true
}
`, queue)
}

func testAccAWSSQSConfigWithEncryption(queue string) string {
	return fmt.Sprintf(`
resource "aws_sqs_queue" "queue" {
  name                              = "%s"
  kms_master_key_id                 = "alias/aws/sqs"
  kms_data_key_reuse_period_seconds = 300
}
`, queue)
}
