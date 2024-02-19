// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package batch_test

import (
	"fmt"
	"testing"

	"github.com/YakDriver/regexache"
	sdkacctest "github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-provider-aws/internal/acctest"
	"github.com/hashicorp/terraform-provider-aws/names"
)

func TestAccBatchJobDefinitionDataSource_basicName(t *testing.T) {
	ctx := acctest.Context(t)
	rName := sdkacctest.RandomWithPrefix(acctest.ResourcePrefix)
	dataSourceName := "data.aws_batch_job_definition.test"
	resourceName := "aws_batch_job_definition.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			acctest.PreCheck(ctx, t)
			acctest.PreCheckPartitionHasService(t, names.BatchEndpointID)
			testAccPreCheck(ctx, t)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.BatchEndpointID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckJobDefinitionDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccJobDefinitionDataSourceConfig_basicName(rName, "1"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair(dataSourceName, "arn", resourceName, "arn"),
					resource.TestCheckResourceAttr(dataSourceName, "revision", "1"),
					resource.TestCheckResourceAttr(dataSourceName, "retry_strategy.attempts", "10"),
				),
			},
			{
				// specify revision
				Config: testAccJobDefinitionDataSourceConfig_basicNameRevision(rName, "2", 2),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(dataSourceName, "revision", "2"),
				),
			},
		},
	})
}

func TestAccBatchJobDefinitionDataSource_basicARN(t *testing.T) {
	ctx := acctest.Context(t)
	rName := sdkacctest.RandomWithPrefix(acctest.ResourcePrefix)
	dataSourceName := "data.aws_batch_job_definition.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			acctest.PreCheck(ctx, t)
			acctest.PreCheckPartitionHasService(t, names.BatchEndpointID)
			testAccPreCheck(ctx, t)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.BatchEndpointID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckJobDefinitionDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccJobDefinitionDataSourceConfig_basicARN(rName, "1"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(dataSourceName, "revision", "1"),
					resource.TestCheckResourceAttr(dataSourceName, "retry_strategy.attempts", "10"),
					acctest.MatchResourceAttrRegionalARN(dataSourceName, "arn", "batch", regexache.MustCompile(fmt.Sprintf(`job-definition/%s:\d+`, rName))),
					acctest.MatchResourceAttrRegionalARN(dataSourceName, "arn_prefix", "batch", regexache.MustCompile(fmt.Sprintf(`job-definition/%s`, rName))),
				),
			},
			{
				Config: testAccJobDefinitionDataSourceConfig_basicARN(rName, "2"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(dataSourceName, "revision", "2"),
				),
			},
		},
	})
}

func TestAccBatchJobDefinitionDataSource_basicARN_NodeProperties(t *testing.T) {
	ctx := acctest.Context(t)
	rName := sdkacctest.RandomWithPrefix(acctest.ResourcePrefix)
	dataSourceName := "data.aws_batch_job_definition.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			acctest.PreCheck(ctx, t)
			acctest.PreCheckPartitionHasService(t, names.BatchEndpointID)
			testAccPreCheck(ctx, t)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.BatchEndpointID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckJobDefinitionDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccJobDefinitionDataSourceConfig_basicARNNode(rName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(dataSourceName, "node_properties.main_node", "0"),
					resource.TestCheckResourceAttr(dataSourceName, "node_properties.node_range_properties.#", "2"),
					resource.TestCheckResourceAttr(dataSourceName, "node_properties.node_range_properties.0.container.image", "busybox"),
				),
			},
		},
	})
}

func TestAccBatchJobDefinitionDataSource_basicARN_EKSProperties(t *testing.T) {
	ctx := acctest.Context(t)
	rName := sdkacctest.RandomWithPrefix(acctest.ResourcePrefix)
	dataSourceName := "data.aws_batch_job_definition.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			acctest.PreCheck(ctx, t)
			acctest.PreCheckPartitionHasService(t, names.BatchEndpointID)
			testAccPreCheck(ctx, t)
		},
		ErrorCheck:               acctest.ErrorCheck(t, names.BatchEndpointID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckJobDefinitionDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccJobDefinitionDataSourceConfig_basicARNEKS(rName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(dataSourceName, "type", "container"),
					resource.TestCheckResourceAttr(dataSourceName, "eks_properties.pod_properties.containers.#", "1"),
					resource.TestCheckResourceAttr(dataSourceName, "eks_properties.pod_properties.containers.0.image", "public.ecr.aws/amazonlinux/amazonlinux:1"),
				),
			},
		},
	})
}

func testAccJobDefinitionDataSourceConfig_basicARN(rName string, increment string) string {
	return acctest.ConfigCompose(
		testAccJobDefinitionDataSourceConfig_container(rName, increment),
		`
data "aws_batch_job_definition" "test" {
  arn = aws_batch_job_definition.test.arn
}
`)
}

func testAccJobDefinitionDataSourceConfig_basicName(rName string, increment string) string {
	return acctest.ConfigCompose(
		testAccJobDefinitionDataSourceConfig_container(rName, increment),
		fmt.Sprintf(`
data "aws_batch_job_definition" "test" {
  name = %[1]q

  depends_on = [aws_batch_job_definition.test]
}
`, rName, increment))
}

func testAccJobDefinitionDataSourceConfig_basicNameRevision(rName string, increment string, revision int) string {
	return acctest.ConfigCompose(
		testAccJobDefinitionDataSourceConfig_container(rName, increment),
		fmt.Sprintf(`
data "aws_batch_job_definition" "test" {
  name     = %[1]q
  revision = %[2]d
}
`, rName, revision))
}

func testAccJobDefinitionDataSourceConfig_container(rName string, increment string) string {
	return fmt.Sprintf(`
resource "aws_batch_job_definition" "test" {
  container_properties = jsonencode({
    command = ["echo", "test%[2]s"]
    image   = "busybox"
    memory  = 128
    vcpus   = 1
  })
  name = %[1]q
  type = "container"
  retry_strategy {
    attempts = 10
  }
}
`, rName, increment)
}

func testAccJobDefinitionDataSourceConfig_basicARNNode(rName string) string {
	return acctest.ConfigCompose(
		testAccJobDefinitionConfig_NodeProperties(rName), `
data "aws_batch_job_definition" "test" {
  arn = aws_batch_job_definition.test.arn
}`)
}

func testAccJobDefinitionDataSourceConfig_basicARNEKS(rName string) string {
	return acctest.ConfigCompose(
		testAccJobDefinitionConfig_EKSProperties_basic(rName), `
data "aws_batch_job_definition" "test" {
  arn = aws_batch_job_definition.test.arn
}
`)
}
