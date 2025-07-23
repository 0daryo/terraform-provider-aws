// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package ec2_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	sdkacctest "github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-provider-aws/internal/acctest"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	"github.com/hashicorp/terraform-provider-aws/internal/create"
	"github.com/hashicorp/terraform-provider-aws/internal/retry"
	tfec2 "github.com/hashicorp/terraform-provider-aws/internal/service/ec2"
	"github.com/hashicorp/terraform-provider-aws/names"
)

func TestAccVPCNATGatewayEIPAssociation_basic(t *testing.T) {
	ctx := acctest.Context(t)
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	var vpcnatgatewayeipassociation types.NatGatewayAddress
	rName := sdkacctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "aws_nat_gateway_eip_association.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.EC2ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckVPCNATGatewayEIPAssociationDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccVPCNATGatewayEIPAssociationConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckVPCNATGatewayEIPAssociationExists(ctx, resourceName, &vpcnatgatewayeipassociation),
					resource.TestCheckResourceAttrPair(resourceName, "allocation_id", "aws_eip.secondary", names.AttrID),
					resource.TestCheckResourceAttrPair(resourceName, "nat_gateway_id", "aws_nat_gateway.test", names.AttrID),
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

func TestAccVPCNATGatewayEIPAssociation_disappears(t *testing.T) {
	ctx := acctest.Context(t)
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	var vpcnatgatewayeipassociation types.NatGatewayAddress
	rName := sdkacctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "aws_nat_gateway_eip_association.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(ctx, t) },
		ErrorCheck:               acctest.ErrorCheck(t, names.EC2ServiceID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckVPCNATGatewayEIPAssociationDestroy(ctx),
		Steps: []resource.TestStep{
			{
				Config: testAccVPCNATGatewayEIPAssociationConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckVPCNATGatewayEIPAssociationExists(ctx, resourceName, &vpcnatgatewayeipassociation),
					acctest.CheckFrameworkResourceDisappears(ctx, acctest.Provider, tfec2.ResourceNATGatewayEIPAssociation, resourceName),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func testAccCheckVPCNATGatewayEIPAssociationDestroy(ctx context.Context) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn := acctest.Provider.Meta().(*conns.AWSClient).EC2Client(ctx)

		for _, rs := range s.RootModule().Resources {
			if rs.Type != "aws_nat_gateway_eip_association" {
				continue
			}

			_, err := tfec2.FindNATGatewayAddressByNATGatewayIDAndAllocationIDSucceeded(ctx, conn, rs.Primary.Attributes["nat_gateway_id"], rs.Primary.Attributes["allocation_id"])
			if retry.NotFound(err) {
				return nil
			}
			if err != nil {
				return create.Error(names.EC2, create.ErrActionCheckingDestroyed, tfec2.ResNameVPCNATGatewayEIPAssociation, rs.Primary.ID, err)
			}

			return create.Error(names.EC2, create.ErrActionCheckingDestroyed, tfec2.ResNameVPCNATGatewayEIPAssociation, rs.Primary.ID, errors.New("not destroyed"))
		}

		return nil
	}
}

func testAccCheckVPCNATGatewayEIPAssociationExists(ctx context.Context, name string, vpcnatgatewayeipassociation *types.NatGatewayAddress) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return create.Error(names.EC2, create.ErrActionCheckingExistence, tfec2.ResNameVPCNATGatewayEIPAssociation, name, errors.New("not found"))
		}

		if rs.Primary.ID == "" {
			return create.Error(names.EC2, create.ErrActionCheckingExistence, tfec2.ResNameVPCNATGatewayEIPAssociation, name, errors.New("not set"))
		}

		conn := acctest.Provider.Meta().(*conns.AWSClient).EC2Client(ctx)

		resp, err := tfec2.FindNATGatewayAddressByNATGatewayIDAndAllocationIDSucceeded(ctx, conn, rs.Primary.Attributes["nat_gateway_id"], rs.Primary.Attributes["allocation_id"])
		if err != nil {
			return create.Error(names.EC2, create.ErrActionCheckingExistence, tfec2.ResNameVPCNATGatewayEIPAssociation, rs.Primary.ID, err)
		}

		*vpcnatgatewayeipassociation = *resp

		return nil
	}
}

func testAccVPCNATGatewayEIPAssociationConfig_basic(rName string) string {
	return acctest.ConfigCompose(testAccVPCNATGatewayConfig_basic(rName),
		fmt.Sprintf(`
resource "aws_eip" "secondary" {
  domain = "vpc"

  tags = {
    Name = %[1]q
  }
}

resource "aws_nat_gateway_eip_association" "test" {
  allocation_id  = aws_eip.secondary.id
  nat_gateway_id = aws_nat_gateway.test.id
}
`, rName))
}
