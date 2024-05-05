// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package waf_test

import (
	"fmt"
	"testing"

	"github.com/YakDriver/regexache"
	sdkacctest "github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-provider-aws/internal/acctest"
	"github.com/hashicorp/terraform-provider-aws/names"
)

func TestAccWAFRuleDataSource_basic(t *testing.T) {
	ctx := acctest.Context(t)
	name := sdkacctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "aws_waf_rule.wafrule"
	datasourceName := "data.aws_waf_rule.wafrule"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t); acctest.PreCheckPartitionHasService(t, names.WAFServiceID) },
		ErrorCheck:               acctest.ErrorCheck(t, names.WAFEndpointID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccRuleDataSourceConfig_nonExistent,
				ExpectError: regexache.MustCompile(`WAF Rules not found`),
			},
			{
				Config: testAccRuleDataSourceConfig_name(name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair(datasourceName, "id", resourceName, "id"),
					resource.TestCheckResourceAttrPair(datasourceName, "name", resourceName, "name"),
				),
			},
		},
	})
}

func testAccRuleDataSourceConfig_name(name string) string {
	return fmt.Sprintf(`
resource "aws_waf_rule" "wafrule" {
  name        = %[1]q
  metric_name = "WafruleTest"
}

data "aws_waf_rule" "wafrule" {
  name = aws_waf_rule.wafrule.name
}
`, name)
}

const testAccRuleDataSourceConfig_nonExistent = `
data "aws_waf_rule" "wafrule" {
  name = "tf-acc-test-does-not-exist"
}
`
