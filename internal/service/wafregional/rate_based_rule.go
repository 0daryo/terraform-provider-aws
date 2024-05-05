// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package wafregional

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/wafregional"
	awstypes "github.com/aws/aws-sdk-go-v2/service/wafregional/types"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	"github.com/hashicorp/terraform-provider-aws/internal/enum"
	"github.com/hashicorp/terraform-provider-aws/internal/errs"
	"github.com/hashicorp/terraform-provider-aws/internal/errs/sdkdiag"
	tftags "github.com/hashicorp/terraform-provider-aws/internal/tags"
	"github.com/hashicorp/terraform-provider-aws/internal/verify"
	"github.com/hashicorp/terraform-provider-aws/names"
)

// @SDKResource("aws_wafregional_rate_based_rule", name="Rate Based Rule")
// @Tags(identifierAttribute="arn")
func resourceRateBasedRule() *schema.Resource {
	return &schema.Resource{
		CreateWithoutTimeout: resourceRateBasedRuleCreate,
		ReadWithoutTimeout:   resourceRateBasedRuleRead,
		UpdateWithoutTimeout: resourceRateBasedRuleUpdate,
		DeleteWithoutTimeout: resourceRateBasedRuleDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"metric_name": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validMetricName,
			},
			"predicate": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"negated": {
							Type:     schema.TypeBool,
							Required: true,
						},
						"data_id": {
							Type:         schema.TypeString,
							Required:     true,
							ValidateFunc: validation.StringLenBetween(0, 128),
						},
						"type": {
							Type:             schema.TypeString,
							Required:         true,
							ValidateDiagFunc: enum.Validate[awstypes.PredicateType](),
						},
					},
				},
			},
			"rate_key": {
				Type:     schema.TypeString,
				Required: true,
			},
			"rate_limit": {
				Type:         schema.TypeInt,
				Required:     true,
				ValidateFunc: validation.IntAtLeast(100),
			},
			names.AttrTags:    tftags.TagsSchema(),
			names.AttrTagsAll: tftags.TagsSchemaComputed(),
			"arn": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},

		CustomizeDiff: verify.SetTagsDiff,
	}
}

func resourceRateBasedRuleCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).WAFRegionalClient(ctx)
	region := meta.(*conns.AWSClient).Region

	name := d.Get("name").(string)
	outputRaw, err := NewRetryer(conn, region).RetryWithToken(ctx, func(token *string) (interface{}, error) {
		input := &wafregional.CreateRateBasedRuleInput{
			ChangeToken: token,
			MetricName:  aws.String(d.Get("metric_name").(string)),
			Name:        aws.String(name),
			RateKey:     awstypes.RateKey(d.Get("rate_key").(string)),
			RateLimit:   aws.Int64(int64(d.Get("rate_limit").(int))),
			Tags:        getTagsIn(ctx),
		}

		return conn.CreateRateBasedRule(ctx, input)
	})

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "creating WAF Regional Rate Based Rule (%s): %s", name, err)
	}

	d.SetId(aws.ToString(outputRaw.(*wafregional.CreateRateBasedRuleOutput).Rule.RuleId))

	if newPredicates := d.Get("predicate").(*schema.Set).List(); len(newPredicates) > 0 {
		var oldPredicates []interface{}

		if err := updateRateBasedRuleResourceWR(ctx, conn, region, d.Id(), d.Get("rate_limit").(int), oldPredicates, newPredicates); err != nil {
			return sdkdiag.AppendErrorf(diags, "updating WAF Regional Rate Based Rule (%s): %s", d.Id(), err)
		}
	}

	return append(diags, resourceRateBasedRuleRead(ctx, d, meta)...)
}

func resourceRateBasedRuleRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).WAFRegionalClient(ctx)

	params := &wafregional.GetRateBasedRuleInput{
		RuleId: aws.String(d.Id()),
	}

	resp, err := conn.GetRateBasedRule(ctx, params)
	if !d.IsNewResource() && errs.IsA[*awstypes.WAFNonexistentItemException](err) {
		log.Printf("[WARN] WAF Regional Rate Based Rule (%s) not found, removing from state", d.Get("name").(string))
		d.SetId("")
		return diags
	}
	if err != nil {
		return sdkdiag.AppendErrorf(diags, "getting WAF Regional Rate Based Rule (%s): %s", d.Get("name").(string), err)
	}

	var predicates []map[string]interface{}

	for _, predicateSet := range resp.Rule.MatchPredicates {
		predicates = append(predicates, map[string]interface{}{
			"negated": *predicateSet.Negated,
			"type":    predicateSet.Type,
			"data_id": *predicateSet.DataId,
		})
	}

	arn := arn.ARN{
		AccountID: meta.(*conns.AWSClient).AccountID,
		Partition: meta.(*conns.AWSClient).Partition,
		Region:    meta.(*conns.AWSClient).Region,
		Resource:  fmt.Sprintf("ratebasedrule/%s", d.Id()),
		Service:   "waf-regional",
	}.String()
	d.Set("arn", arn)
	d.Set("predicate", predicates)
	d.Set("name", resp.Rule.Name)
	d.Set("metric_name", resp.Rule.MetricName)
	d.Set("rate_key", resp.Rule.RateKey)
	d.Set("rate_limit", resp.Rule.RateLimit)

	return diags
}

func resourceRateBasedRuleUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).WAFRegionalClient(ctx)
	region := meta.(*conns.AWSClient).Region

	if d.HasChanges("predicate", "rate_limit") {
		o, n := d.GetChange("predicate")
		oldPredicates, newPredicates := o.(*schema.Set).List(), n.(*schema.Set).List()

		if err := updateRateBasedRuleResourceWR(ctx, conn, region, d.Id(), d.Get("rate_limit").(int), oldPredicates, newPredicates); err != nil {
			return sdkdiag.AppendErrorf(diags, "updating WAF Regional Rate Based Rule (%s): %s", d.Id(), err)
		}
	}

	return append(diags, resourceRateBasedRuleRead(ctx, d, meta)...)
}

func resourceRateBasedRuleDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).WAFRegionalClient(ctx)
	region := meta.(*conns.AWSClient).Region

	if oldPredicates := d.Get("predicate").(*schema.Set).List(); len(oldPredicates) > 0 {
		var newPredicates []interface{}
		err := updateRateBasedRuleResourceWR(ctx, conn, region, d.Id(), d.Get("rate_limit").(int), oldPredicates, newPredicates)
		if err != nil {
			if !errs.IsA[*awstypes.WAFNonexistentItemException](err) && !errs.IsA[*awstypes.WAFNonexistentContainerException](err) {
				return sdkdiag.AppendErrorf(diags, "updating WAF Regional Rate Based Rule (%s): %s", d.Id(), err)
			}
		}
	}

	_, err := NewRetryer(conn, region).RetryWithToken(ctx, func(token *string) (interface{}, error) {
		req := &wafregional.DeleteRateBasedRuleInput{
			ChangeToken: token,
			RuleId:      aws.String(d.Id()),
		}
		log.Printf("[INFO] Deleting WAF Regional Rate Based Rule")
		return conn.DeleteRateBasedRule(ctx, req)
	})

	if errs.IsA[*awstypes.WAFNonexistentItemException](err) {
		return diags
	}

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "deleting WAF Regional Rate Based Rule (%s): %s", d.Id(), err)
	}

	return diags
}

func updateRateBasedRuleResourceWR(ctx context.Context, conn *wafregional.Client, region, ruleID string, rateLimit int, oldP, newP []interface{}) error {
	_, err := NewRetryer(conn, region).RetryWithToken(ctx, func(token *string) (interface{}, error) {
		input := &wafregional.UpdateRateBasedRuleInput{
			ChangeToken: token,
			RateLimit:   aws.Int64(int64(rateLimit)),
			RuleId:      aws.String(ruleID),
			Updates:     DiffRulePredicates(oldP, newP),
		}

		return conn.UpdateRateBasedRule(ctx, input)
	})

	return err
}
