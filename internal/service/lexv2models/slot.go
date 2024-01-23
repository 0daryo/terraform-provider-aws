// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package lexv2models

import (
	"context"
	"errors"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lexmodelsv2"
	awstypes "github.com/aws/aws-sdk-go-v2/service/lexmodelsv2/types"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/hashicorp/terraform-provider-aws/internal/create"
	intflex "github.com/hashicorp/terraform-provider-aws/internal/flex"
	"github.com/hashicorp/terraform-provider-aws/internal/framework"
	"github.com/hashicorp/terraform-provider-aws/internal/framework/flex"
	fwtypes "github.com/hashicorp/terraform-provider-aws/internal/framework/types"
	lexschema "github.com/hashicorp/terraform-provider-aws/internal/service/lexv2models/schema"
	"github.com/hashicorp/terraform-provider-aws/internal/tfresource"
	"github.com/hashicorp/terraform-provider-aws/names"
)

// @FrameworkResource(name="Slot")
func newResourceSlot(_ context.Context) (resource.ResourceWithConfigure, error) {
	r := &resourceSlot{}

	r.SetDefaultCreateTimeout(30 * time.Minute)
	r.SetDefaultUpdateTimeout(30 * time.Minute)
	r.SetDefaultDeleteTimeout(30 * time.Minute)

	return r, nil
}

const (
	ResNameSlot = "Slot"

	slotIDPartCount = 5
)

type resourceSlot struct {
	framework.ResourceWithConfigure
	framework.WithTimeouts
}

func (r *resourceSlot) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "aws_lexv2models_slot"
}

func (r *resourceSlot) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"bot_id": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"bot_version": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				Optional: true,
			},
			"id": framework.IDAttribute(),
			"intent_id": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"locale_id": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"slot_type_id": schema.StringAttribute{
				Optional: true,
			},
		},
		Blocks: map[string]schema.Block{
			"multiple_values_setting":   lexschema.MultipleValuesSettingBlock(ctx),
			"obfuscation_setting":       lexschema.ObfuscationSettingBlock(ctx),
			"value_elicitation_setting": lexschema.ValueElicitationSettingBlock(ctx),
			//sub_slot_setting
			"timeouts": timeouts.Block(ctx, timeouts.Opts{
				Create: true,
				Update: true,
				Delete: true,
			}),
		},
	}
}

func (r *resourceSlot) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	conn := r.Meta().LexV2ModelsClient(ctx)

	var plan resourceSlotData
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	in := &lexmodelsv2.CreateSlotInput{
		SlotName: aws.String(plan.Name.ValueString()),
	}

	resp.Diagnostics.Append(flex.Expand(ctx, plan, &in)...)
	if resp.Diagnostics.HasError() {
		return
	}

	out, err := conn.CreateSlot(ctx, in)
	if err != nil {
		resp.Diagnostics.AddError(
			create.ProblemStandardMessage(names.LexV2Models, create.ErrActionCreating, ResNameSlot, plan.Name.String(), err),
			err.Error(),
		)
		return
	}
	if out == nil {
		resp.Diagnostics.AddError(
			create.ProblemStandardMessage(names.LexV2Models, create.ErrActionCreating, ResNameSlot, plan.Name.String(), nil),
			errors.New("empty output").Error(),
		)
		return
	}

	idParts := []string{
		aws.ToString(out.BotId),
		aws.ToString(out.BotVersion),
		aws.ToString(out.IntentId),
		aws.ToString(out.LocaleId),
		aws.ToString(out.SlotId),
	}
	id, err := intflex.FlattenResourceId(idParts, slotIDPartCount, false)
	if err != nil {
		resp.Diagnostics.AddError(
			create.ProblemStandardMessage(names.LexV2Models, create.ErrActionCreating, ResNameSlot, plan.Name.String(), err),
			err.Error(),
		)
		return
	}

	plan.ID = types.StringValue(id)

	resp.Diagnostics.Append(flex.Flatten(ctx, out, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *resourceSlot) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	conn := r.Meta().LexV2ModelsClient(ctx)

	var state resourceSlotData
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	out, err := findSlotByID(ctx, conn, state.ID.ValueString())
	if tfresource.NotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError(
			create.ProblemStandardMessage(names.LexV2Models, create.ErrActionSetting, ResNameSlot, state.ID.String(), err),
			err.Error(),
		)
		return
	}

	resp.Diagnostics.Append(flex.Flatten(ctx, out, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *resourceSlot) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	conn := r.Meta().LexV2ModelsClient(ctx)

	var plan, state resourceSlotData
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if slotHasChanges(ctx, plan, state) {
		input := &lexmodelsv2.UpdateSlotInput{}

		// TODO: expand here, or check for updatable arguments individually?

		resp.Diagnostics.Append(flex.Expand(context.WithValue(ctx, flex.ResourcePrefix, ResNameSlot), &plan, input)...)
		if resp.Diagnostics.HasError() {
			return
		}

		out, err := conn.UpdateSlot(ctx, input)
		if err != nil {
			resp.Diagnostics.AddError(
				create.ProblemStandardMessage(names.LexV2Models, create.ErrActionUpdating, ResNameSlot, plan.ID.String(), err),
				err.Error(),
			)
			return
		}
		if out == nil {
			resp.Diagnostics.AddError(
				create.ProblemStandardMessage(names.LexV2Models, create.ErrActionUpdating, ResNameSlot, plan.ID.String(), nil),
				errors.New("empty output").Error(),
			)
			return
		}

		// resp.Diagnostics.Append(flex.Flatten(ctx, out, &plan)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *resourceSlot) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	conn := r.Meta().LexV2ModelsClient(ctx)

	var state resourceSlotData
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	in := &lexmodelsv2.DeleteSlotInput{
		BotId:      aws.String(state.ID.ValueString()),
		BotVersion: aws.String(state.ID.ValueString()),
		IntentId:   aws.String(state.ID.ValueString()),
		LocaleId:   aws.String(state.ID.ValueString()),
		SlotId:     aws.String(state.ID.ValueString()),
	}

	_, err := conn.DeleteSlot(ctx, in)
	if err != nil {
		var nfe *awstypes.ResourceNotFoundException
		if errors.As(err, &nfe) {
			return
		}
		resp.Diagnostics.AddError(
			create.ProblemStandardMessage(names.LexV2Models, create.ErrActionDeleting, ResNameSlot, state.ID.String(), err),
			err.Error(),
		)
		return
	}
}

func (r *resourceSlot) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func findSlotByID(ctx context.Context, conn *lexmodelsv2.Client, id string) (*lexmodelsv2.DescribeSlotOutput, error) {
	parts, err := intflex.ExpandResourceId(id, slotIDPartCount, false)
	if err != nil {
		return nil, err
	}

	in := &lexmodelsv2.DescribeSlotInput{
		BotId:      aws.String(parts[0]),
		BotVersion: aws.String(parts[1]),
		IntentId:   aws.String(parts[2]),
		LocaleId:   aws.String(parts[3]),
		SlotId:     aws.String(parts[4]),
	}

	out, err := conn.DescribeSlot(ctx, in)
	if err != nil {
		var nfe *awstypes.ResourceNotFoundException
		if errors.As(err, &nfe) {
			return nil, &retry.NotFoundError{
				LastError:   err,
				LastRequest: in,
			}
		}

		return nil, err
	}

	if out == nil {
		return nil, tfresource.NewEmptyResultError(in)
	}

	return out, nil
}

type resourceSlotData struct {
	BotID                    types.String                                                           `tfsdk:"bot_id"`
	BotVersion               types.String                                                           `tfsdk:"bot_version"`
	Description              types.String                                                           `tfsdk:"description"`
	ID                       types.String                                                           `tfsdk:"id"`
	IntentID                 types.String                                                           `tfsdk:"intent_id"`
	LocaleID                 types.String                                                           `tfsdk:"locale_id"`
	MultipleValuesSetting    fwtypes.ListNestedObjectValueOf[lexschema.MultipleValuesSettingData]   `tfsdk:"multiple_values_setting"`
	Name                     types.String                                                           `tfsdk:"name"`
	ObfuscationSetting       fwtypes.ListNestedObjectValueOf[lexschema.ObfuscationSettingData]      `tfsdk:"obfuscation_setting"`
	Timeouts                 timeouts.Value                                                         `tfsdk:"timeouts"`
	SlotTypeID               types.String                                                           `tfsdk:"slot_type_id"`
	ValueElicitationSettings fwtypes.ListNestedObjectValueOf[lexschema.ValueElicitationSettingData] `tfsdk:"value_elicitation_settings"`
}

func slotHasChanges(_ context.Context, plan, state resourceSlotData) bool {
	return !plan.Description.Equal(state.Description) ||
		!plan.Name.Equal(state.Name) ||
		!plan.Description.Equal(state.Description) ||
		!plan.SlotTypeID.Equal(state.SlotTypeID)
}
