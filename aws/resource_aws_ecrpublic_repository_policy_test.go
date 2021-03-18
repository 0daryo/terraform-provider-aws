package aws

import (
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccAWSEcrPublicRepositoryPolicy_basic(t *testing.T) {
	randString := acctest.RandString(10)
	resourceName := "aws_ecrpublic_repository_policy.default"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t); testAccPreCheckAwsEcrPublic(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAwsEcrPublicRepositoryPolicyDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSEcrPublicRepositoryPolicy(randString),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSEcrPublicRepositoryPolicyExists(resourceName),
					resource.TestCheckResourceAttrSet(resourceName, "policy"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccAWSEcrPublicRepositoryPolicyUpdated(randString),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSEcrPublicRepositoryPolicyExists(resourceName),
					resource.TestCheckResourceAttrSet(resourceName, "policy"),
				),
			},
		},
	})
}

func TestAccAWSEcrPublicRepositoryPolicy_iam(t *testing.T) {
	randString := acctest.RandString(10)
	resourceName := "aws_ecrpublic_repository_policy.default"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t); testAccPreCheckAwsEcrPublic(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAwsEcrPublicRepositoryPolicyDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAWSEcrPublicRepositoryPolicyWithIAMRole(randString),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSEcrPublicRepositoryPolicyExists(resourceName),
					resource.TestCheckResourceAttrSet(resourceName, "policy"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccAWSEcrPublicRepositoryPolicyWithIAMRoleUpdated(randString),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSEcrPublicRepositoryPolicyExists(resourceName),
					resource.TestCheckResourceAttrSet(resourceName, "policy"),
				),
			},
		},
	})
}

func testAccCheckAwsEcrPublicRepositoryPolicyDestroy(s *terraform.State) error {
	conn := testAccProvider.Meta().(*AWSClient).ecrconn

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "aws_ecrpublic_repository_policy" {
			continue
		}

		_, err := conn.GetRepositoryPolicy(&ecr.GetRepositoryPolicyInput{
			RegistryId:     aws.String(rs.Primary.Attributes["registry_id"]),
			RepositoryName: aws.String(rs.Primary.Attributes["repository"]),
		})
		if err != nil {
			if ecrerr, ok := err.(awserr.Error); ok && ecrerr.Code() == "RepositoryNotFoundException" {
				return nil
			}
			return err
		}
	}

	return nil
}

func testAccCheckAWSEcrPublicRepositoryPolicyExists(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		_, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("Not found: %s", name)
		}

		return nil
	}
}

func testAccAWSEcrPublicRepositoryPolicy(randString string) string {
	return fmt.Sprintf(`
resource "aws_ecrpublic_repository" "foo" {
  repository_name = "tf-acc-test-ecr-%s"
}

resource "aws_ecrpublic_repository_policy" "default" {
  repository = aws_ecrpublic_repository.foo.repository_name

  policy = <<EOF
{
    "Version": "2008-10-17",
    "Statement": [
        {
            "Sid": "testpolicy",
            "Effect": "Allow",
            "Principal": "*",
            "Action": [
                "ecr-public:ListImages"
            ]
        }
    ]
}
EOF
}
`, randString)
}

func testAccAWSEcrPublicRepositoryPolicyUpdated(randString string) string {
	return fmt.Sprintf(`
resource "aws_ecrpublic_repository" "foo" {
  repository_name = "tf-acc-test-ecr-%s"
}

resource "aws_ecrpublic_repository_policy" "default" {
  repository = aws_ecrpublic_repository.foo.repository_name

  policy = <<EOF
{
    "Version": "2008-10-17",
    "Statement": [
        {
            "Sid": "testpolicy",
            "Effect": "Allow",
            "Principal": "*",
            "Action": [
                "ecr-public:ListImages",
                "ecr-public:PutImage"
            ]
        }
    ]
}
EOF
}
`, randString)
}

// testAccAwsEcrPublicRepositoryPolicyWithIAMRole creates a new IAM Role and tries
// to use it's ARN in an ECR Repository Policy. IAM changes need some time to
// be propagated to other services - like ECR. So the following code should
// exercise our retry logic, since we try to use the new resource instantly.
func testAccAWSEcrPublicRepositoryPolicyWithIAMRole(randString string) string {
	return fmt.Sprintf(`
resource "aws_ecrpublic_repository" "foo" {
  repository_name = "tf-acc-test-ecr-%s"
}

resource "aws_iam_role" "foo" {
  name = "tf-acc-test-ecr-%s"

  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": "sts:AssumeRole",
      "Effect": "Allow",
      "Principal": {
        "Service": "ec2.amazonaws.com"
      }
    }
  ]
}
EOF
}

resource "aws_ecrpublic_repository_policy" "default" {
  repository = aws_ecrpublic_repository.foo.repository_name

  policy = <<EOF
{
    "Version": "2008-10-17",
    "Statement": [
        {
            "Sid": "testpolicy",
            "Effect": "Allow",
            "Principal": {
              "AWS": "${aws_iam_role.foo.arn}"
            },
            "Action": [
                "ecr-public:ListImages"
            ]
        }
    ]
}
EOF
}
`, randString, randString)
}

func testAccAWSEcrPublicRepositoryPolicyWithIAMRoleUpdated(randString string) string {
	return fmt.Sprintf(`
resource "aws_ecrpublic_repository" "foo" {
  repository_name = "tf-acc-test-ecr-%s"
}

resource "aws_iam_role" "foo" {
  name = "tf-acc-test-ecr-%s"

  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": "sts:AssumeRole",
      "Effect": "Allow",
      "Principal": {
        "Service": "ec2.amazonaws.com"
      }
    }
  ]
}
EOF
}

resource "aws_ecrpublic_repository_policy" "default" {
  repository = aws_ecrpublic_repository.foo.repository_name

  policy = <<EOF
{
    "Version": "2008-10-17",
    "Statement": [
        {
            "Sid": "testpolicy",
            "Effect": "Allow",
            "Principal": {
              "AWS": "${aws_iam_role.foo.arn}"
            },
            "Action": [
                "ecr-public:ListImages",
                "ecr-public:PutImage"
            ]
        }
    ]
}
EOF
}
`, randString, randString)
}
