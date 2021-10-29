package connect

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/connect"
	"github.com/hashicorp/aws-sdk-go-base/tfawserr"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	"github.com/hashicorp/terraform-provider-aws/internal/tfresource"
)

func ResourceBotAssociation() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceBotAssociationCreate,
		ReadContext:   resourceBotAssociationRead,
		UpdateContext: resourceBotAssociationRead,
		DeleteContext: resourceBotAssociationDelete,
		Importer: &schema.ResourceImporter{
			State: func(d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				instanceID, name, region, err := resourceBotV1AssociationParseID(d.Id())
				if err != nil {
					return nil, err
				}
				d.Set("bot_name", name)
				d.Set("instance_id", instanceID)
				d.Set("lex_region", region)
				d.SetId(fmt.Sprintf("%s:%s:%s", instanceID, name, region))

				return []*schema.ResourceData{d}, nil
			},
		},
		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(connectBotAssociationCreateTimeout),
			Delete: schema.DefaultTimeout(connectBotAssociationDeleteTimeout),
		},
		Schema: map[string]*schema.Schema{
			/*
				We would need a schema like this to support a v1/v2 hybrid
				"alias_arn": {
					Type:         schema.TypeString,
					Optional:     true,
					AtLeastOneOf: []string{"bot_name", "alias_arn"},
				},
				"bot_name": {
					Type:         schema.TypeString,
					Optional:     true,
					ValidateFunc: validation.StringLenBetween(2, 50),
					AtLeastOneOf: []string{"bot_name", "alias_arn"},
				},
				"instance_id": {
					Type:     schema.TypeString,
					Required: true,
				},
				"lex_region": {
					Type:         schema.TypeString,
					Optional:     true,
					RequiredWith: []string{"bot_name"},
				},
			*/
			"bot_name": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.StringLenBetween(2, 50),
			},
			"instance_id": {
				Type:     schema.TypeString,
				Required: true,
			},
			"lex_region": {
				Type:     schema.TypeString,
				Required: true,
			},
		},
	}
}

func resourceBotAssociationCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*conns.AWSClient).ConnectConn

	input := &connect.AssociateBotInput{
		InstanceId: aws.String(d.Get("instance_id").(string)),
	}
	lexBotAssociation := &connect.LexBot{
		Name:      aws.String(d.Get("bot_name").(string)),
		LexRegion: aws.String(d.Get("lex_region").(string)),
	}
	input.LexBot = lexBotAssociation

	/*
		We would need something like this and additionally the opposite on the above
		if _, ok := d.GetOk("alias_arn"); ok {
			lexV2BotAssociation := &connect.LexV2Bot{
				AliasArn: aws.String(d.Get("alias_arn").(string)),
			}
			input.LexV2Bot = lexV2BotAssociation
		}
	*/
	// I'm really not sure how we can overload the ID to make it handle V1 and V2
	lbaId := fmt.Sprintf("%s:%s:%s", d.Get("instance_id").(string), d.Get("bot_name").(string), d.Get("lex_region").(string))

	log.Printf("[DEBUG] Creating Connect Bot V1 Association %s", input)
	err := resource.Retry(d.Timeout(schema.TimeoutCreate), func() *resource.RetryError {
		_, err := conn.AssociateBotWithContext(ctx, input)

		// Wait for the bot to finish building until then the connect will not see the bot
		if tfawserr.ErrCodeEquals(err, connect.ErrCodeInvalidRequestException) {
			return resource.RetryableError(err)
		}

		return nil
	})

	if tfresource.TimedOut(err) { // nosemgrep: helper-schema-TimeoutError-check-doesnt-return-output
		// surface the actual error on timeout
		_, err = conn.AssociateBotWithContext(ctx, input)
	}

	if err != nil {
		return diag.FromErr(fmt.Errorf("error creating Connect Bot V1 Association (%s): %s", lbaId, err))
	}

	d.SetId(lbaId)
	return resourceBotAssociationRead(ctx, d, meta)
}

func resourceBotAssociationRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*conns.AWSClient).ConnectConn
	instanceID := d.Get("instance_id")
	name := d.Get("bot_name")

	lexBot, err := FindBotAssociationV1ByNameWithContext(ctx, conn, instanceID.(string), name.(string))

	if tfawserr.ErrMessageContains(err, BotAssociationStatusNotFound, "") || errors.Is(err, tfresource.ErrEmptyResult) {
		log.Printf("[WARN] Connect Bot V1 Association (%s) not found, removing from state", d.Id())
		d.SetId("")
		return nil
	}
	if err != nil {
		return diag.FromErr(fmt.Errorf("error finding Connect Bot V1 Association by name (%s): %w", name, err))
	}

	d.Set("bot_name", lexBot.Name)
	d.Set("instance_id", instanceID)
	d.Set("lex_region", lexBot.LexRegion)

	lbaId := fmt.Sprintf("%s:%s:%s", instanceID, d.Get("bot_name").(string), d.Get("lex_region").(string))
	d.SetId(lbaId)

	/*
		More If v2 statements would be required
		if _, ok := d.GetOk("alias_arn"); ok {
			lexV2Bot, err := finder.V2BotAssociationByAliasArnWithContext(ctx, conn, instanceID.(string), aliasArn.(string))
			if err != nil {
				return diag.FromErr(fmt.Errorf("error finding V2 Bot Association by name (%s): %w", name, err))
			}

			if lexV2Bot == nil {
				return diag.FromErr(fmt.Errorf("error finding V2 Bot Association by name (%s): not found", name))
			}

			d.Set("alias_arn", lexV2Bot.AliasArn)
			d.Set("instance_id", instanceID)
		}
	*/

	return nil
}

func resourceBotAssociationDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*conns.AWSClient).ConnectConn

	instanceID, name, region, err := resourceBotV1AssociationParseID(d.Id())

	if err != nil {
		return diag.FromErr(err)
	}

	input := &connect.DisassociateBotInput{
		InstanceId: aws.String(instanceID),
	}

	lexBotAssociation := &connect.LexBot{
		Name:      aws.String(name),
		LexRegion: aws.String(region),
	}
	input.LexBot = lexBotAssociation

	log.Printf("[DEBUG] Deleting Connect Bot V1 Association %s", d.Id())
	_, dissErr := conn.DisassociateBot(input)

	if dissErr != nil {
		return diag.FromErr(fmt.Errorf("error deleting Connect Bot V1 Association (%s): %s", instanceID, err))
	}
	return nil
}

func resourceBotV1AssociationParseID(id string) (string, string, string, error) {
	parts := strings.SplitN(id, ":", 3)

	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return "", "", "", fmt.Errorf("unexpected format of Connect Bot V1 Association ID (%s), expected instanceID:name:region", id)
	}

	return parts[0], parts[1], parts[2], nil
}
