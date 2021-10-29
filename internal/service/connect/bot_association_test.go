package connect

import (
	"context"
	"errors"
	"fmt"
	"log"
	"testing"

	"github.com/aws/aws-sdk-go/service/connect"
	"github.com/hashicorp/aws-sdk-go-base/tfawserr"
	sdkacctest "github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/hashicorp/terraform-provider-aws/internal/acctest"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	"github.com/hashicorp/terraform-provider-aws/internal/tfresource"
)

//Serialized acceptance tests due to Connect account limits (max 2 parallel tests)
func TestAccBotAssociation_serial(t *testing.T) {
	testCases := map[string]func(t *testing.T){
		"basic":      testAccBotAssociation_basic,
		"disappears": testAccBotAssociation_disappears,
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			tc(t)
		})
	}
}

func testAccBotAssociation_basic(t *testing.T) {
	var v connect.LexBot
	rName := sdkacctest.RandStringFromCharSet(8, sdkacctest.CharSetAlpha)
	rName2 := sdkacctest.RandomWithPrefix("resource-test-terraform")
	resourceName := "aws_connect_bot_association.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t) },
		ErrorCheck:   acctest.ErrorCheck(t, connect.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckBotAssociationDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccBotV1AssociationConfigBasic(rName, rName2),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckBotAssociationExists(resourceName, &v),
					resource.TestCheckResourceAttrSet(resourceName, "instance_id"),
					resource.TestCheckResourceAttrSet(resourceName, "bot_name"),
					resource.TestCheckResourceAttrSet(resourceName, "lex_region"),
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

func testAccBotAssociation_disappears(t *testing.T) {
	var v connect.LexBot
	rName := sdkacctest.RandStringFromCharSet(8, sdkacctest.CharSetAlpha)
	rName2 := sdkacctest.RandomWithPrefix("resource-test-terraform")
	resourceName := "aws_connect_bot_association.test"
	instanceResourceName := "aws_connect_bot_association.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t) },
		ErrorCheck:   acctest.ErrorCheck(t, connect.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckBotAssociationDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccBotV1AssociationConfigBasic(rName, rName2),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckBotAssociationExists(resourceName, &v),
					acctest.CheckResourceDisappears(acctest.Provider, ResourceBotAssociation(), instanceResourceName),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func testAccCheckBotAssociationExists(resourceName string, function *connect.LexBot) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("Connect Bot V1 Association not found: %s", resourceName)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("Connect Bot V1 Association ID not set")
		}
		instanceID, name, _, err := resourceBotV1AssociationParseID(rs.Primary.ID)

		if err != nil {
			return err
		}

		conn := acctest.Provider.Meta().(*conns.AWSClient).ConnectConn

		lexBot, err := FindBotAssociationV1ByNameWithContext(context.Background(), conn, instanceID, name)

		if err != nil {
			return fmt.Errorf("error finding Connect Bot V1 Association by name (%s): %w", name, err)
		}

		if lexBot == nil {
			return fmt.Errorf("error finding Connect Bot V1 Association by name (%s): not found", name)
		}

		*function = *lexBot

		return nil
	}
}

func testAccCheckBotAssociationDestroy(s *terraform.State) error {
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "aws_connect_bot_association" {
			continue
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("Connect Connect Bot V1 Association ID not set")
		}
		instanceID, name, _, err := resourceBotV1AssociationParseID(rs.Primary.ID)
		if err != nil {
			return err
		}

		conn := acctest.Provider.Meta().(*conns.AWSClient).ConnectConn

		lexBot, err := FindBotAssociationV1ByNameWithContext(context.Background(), conn, instanceID, name)

		if tfawserr.ErrCodeEquals(err, BotAssociationStatusNotFound, "") || errors.Is(err, tfresource.ErrEmptyResult) {
			log.Printf("[DEBUG] Connect Bot V1 Association (%s) not found, has been removed from state", name)
			return nil
		}

		if err != nil {
			return fmt.Errorf("error finding Connect Bot V1 Association by name (%s) potentially still exists: %w", name, err)
		}

		if lexBot != nil {
			return fmt.Errorf("error Connect Bot V1 Association by name (%s): still exists", name)
		}
	}
	return nil
}

func testAccBotV1AssociationConfigBase(rName string, rName2 string) string {
	return fmt.Sprintf(`
resource "aws_lex_intent" "test" {
  create_version = true
  name           = %[1]q
  fulfillment_activity {
    type = "ReturnIntent"
  }
  sample_utterances = [
    "I would like to pick up flowers",
  ]
}

resource "aws_lex_bot" "test" {
  abort_statement {
    message {
      content      = "Sorry, I am not able to assist at this time"
      content_type = "PlainText"
    }
  }
  clarification_prompt {
    max_attempts = 2

    message {
      content      = "I didn't understand you, what would you like to do?"
      content_type = "PlainText"
    }
  }
  intent {
    intent_name    = aws_lex_intent.test.name
    intent_version = "1"
  }

  child_directed   = false
  name             = %[1]q
  process_behavior = "BUILD"
}

resource "aws_connect_instance" "test" {
  identity_management_type = "CONNECT_MANAGED"
  inbound_calls_enabled    = true
  instance_alias           = %[2]q
  outbound_calls_enabled   = true
}
  `, rName, rName2)
}

func testAccBotV1AssociationConfigBasic(rName string, rName2 string) string {
	return acctest.ConfigCompose(
		testAccBotV1AssociationConfigBase(rName, rName2),
		`
data "aws_region" "current" {}

resource "aws_connect_bot_association" "test" {
  instance_id = aws_connect_instance.test.id
  bot_name    = aws_lex_bot.test.name
  lex_region  = data.aws_region.current.name
}
`)
}
