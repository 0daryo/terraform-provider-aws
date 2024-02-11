// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package m2

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/m2"
	awstypes "github.com/aws/aws-sdk-go-v2/service/m2/types"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-provider-aws/internal/create"
	"github.com/hashicorp/terraform-provider-aws/internal/errs"
	"github.com/hashicorp/terraform-provider-aws/internal/framework"
	"github.com/hashicorp/terraform-provider-aws/internal/framework/flex"
	fwtypes "github.com/hashicorp/terraform-provider-aws/internal/framework/types"
	tftags "github.com/hashicorp/terraform-provider-aws/internal/tags"
	"github.com/hashicorp/terraform-provider-aws/internal/tfresource"
	"github.com/hashicorp/terraform-provider-aws/names"
)

// @FrameworkResource(name="M2 Application")
// @Tags(identifierAttribute="arn")
func newResourceApplication(context.Context) (resource.ResourceWithConfigure, error) {
	r := &resourceApplication{}
	r.SetDefaultCreateTimeout(40 * time.Minute)
	r.SetDefaultUpdateTimeout(80 * time.Minute)
	r.SetDefaultDeleteTimeout(40 * time.Minute)
	return r, nil
}

const (
	ResNameApplication = "M2 Application"
)

type resourceApplication struct {
	framework.ResourceWithConfigure
	framework.WithTimeouts
}

func (r *resourceApplication) Metadata(_ context.Context, request resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = "aws_m2_application"
}

func (r *resourceApplication) Schema(ctx context.Context, request resource.SchemaRequest, response *resource.SchemaResponse) {
	s := schema.Schema{
		Attributes: map[string]schema.Attribute{
			"arn":            framework.ARNAttributeComputedOnly(),
			"application_id": framework.IDAttribute(),
			"application_version": schema.Int64Attribute{
				Computed: true,
			},
			"client_token": schema.StringAttribute{
				Optional: true,
			},
			"description": schema.StringAttribute{
				Optional: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"engine_type": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			// "engine_type": schema.StringAttribute{
			// 	CustomType: fwtypes.StringEnumType[awstypes.EngineType](),
			// 	Required:   true,
			// 	PlanModifiers: []planmodifier.String{
			// 		stringplanmodifier.RequiresReplace(),
			// 	},
			// },
			"id": framework.IDAttribute(),
			"kms_key_id": schema.StringAttribute{
				Optional: true,
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
			"role_arn": schema.StringAttribute{
				Optional: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			names.AttrTags:    tftags.TagsAttribute(),
			names.AttrTagsAll: tftags.TagsAttributeComputedOnly(),
		},
		Blocks: map[string]schema.Block{
			"definition": schema.ListNestedBlock{
				CustomType: fwtypes.NewListNestedObjectTypeOf[definition](ctx),
				Validators: []validator.List{
					listvalidator.IsRequired(),
					listvalidator.SizeAtMost(1),
				},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"content": schema.StringAttribute{
							Optional: true,
							Validators: []validator.String{
								stringvalidator.ConflictsWith(
									path.MatchRelative().AtParent().AtName("s3_location"),
								),
								stringvalidator.ExactlyOneOf(
									path.MatchRelative().AtParent().AtName("s3_location"),
								),
							},
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
						},
						"s3_location": schema.StringAttribute{
							Optional: true,
							Validators: []validator.String{
								stringvalidator.ConflictsWith(
									path.MatchRelative().AtParent().AtName("content"),
								),
								stringvalidator.ExactlyOneOf(
									path.MatchRelative().AtParent().AtName("content"),
								),
							},
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
						},
					},
				},
			},
		},
	}

	if s.Blocks == nil {
		s.Blocks = make(map[string]schema.Block)
	}
	s.Blocks["timeouts"] = timeouts.Block(ctx, timeouts.Opts{
		Create: true,
		Update: true,
		Delete: true,
	})

	response.Schema = s
}

func (r *resourceApplication) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	conn := r.Meta().M2Client(ctx)

	var data resourceApplicationData
	response.Diagnostics.Append(request.Plan.Get(ctx, &data)...)
	if response.Diagnostics.HasError() {
		return
	}

	definition, diags := data.Definition.ToPtr(ctx)
	response.Diagnostics.Append(diags...)
	if response.Diagnostics.HasError() {
		return
	}

	engineType := awstypes.EngineType(*flex.StringFromFramework(ctx, data.EngineType))
	s3location := &awstypes.DefinitionMemberS3Location{
		Value: *flex.StringFromFramework(ctx, definition.S3Location),
	}

	input := &m2.CreateApplicationInput{
		Definition: s3location,
		//EngineType: data.EngineType.ValueEnum(),
		EngineType: engineType,
		Name:       data.Name.ValueStringPointer(),
		Tags:       getTagsIn(ctx),
	}

	output, err := conn.CreateApplication(ctx, input)
	if err != nil {
		response.Diagnostics.AddError(
			create.ProblemStandardMessage(names.M2, create.ErrActionCreating, ResNameApplication, data.ApplicationId.ValueString(), err),
			err.Error(),
		)
		return
	}

	state := data
	state.ID = flex.StringToFramework(ctx, output.ApplicationId)
	state.ApplicationId = flex.StringToFramework(ctx, output.ApplicationId)
	state.ARN = flex.StringToFramework(ctx, output.ApplicationArn)
	state.ApplicationVersion = flex.Int32ToFramework(ctx, output.ApplicationVersion)
	createTimeout := r.CreateTimeout(ctx, data.Timeouts)
	out, err := waitApplicationCreated(ctx, conn, state.ID.ValueString(), createTimeout)

	if err != nil {
		response.Diagnostics.AddError(
			create.ProblemStandardMessage(names.M2, create.ErrActionWaitingForCreation, ResNameEnvironment, data.Name.ValueString(), err),
			err.Error(),
		)
		return
	}
	response.Diagnostics.Append(flex.Flatten(ctx, out, &state)...)

	if response.Diagnostics.HasError() {
		return
	}

	response.Diagnostics.Append(response.State.Set(ctx, &state)...)
}

// Read implements resource.ResourceWithConfigure.
func (r *resourceApplication) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {

	conn := r.Meta().M2Client(ctx)
	var data resourceApplicationData
	response.Diagnostics.Append(request.State.Get(ctx, &data)...)

	if response.Diagnostics.HasError() {
		return
	}

	out, err := FindAppByID(ctx, conn, data.ApplicationId.ValueString())

	if tfresource.NotFound(err) {
		response.State.RemoveResource(ctx)
		return
	}

	if err != nil {
		response.Diagnostics.AddError(
			create.ProblemStandardMessage(names.M2, create.ErrActionSetting, ResNameApplication, data.ApplicationId.ValueString(), err),
			err.Error(),
		)
		return
	}

	//version, err := findApplicationVersion(ctx, conn, data.ID.ValueString(), *out.LatestVersion.ApplicationVersion)

	if tfresource.NotFound(err) {
		response.State.RemoveResource(ctx)
		return
	}

	if err != nil {
		response.Diagnostics.AddError(
			create.ProblemStandardMessage(names.M2, create.ErrActionSetting, ResNameApplication, data.ApplicationId.ValueString(), err),
			err.Error(),
		)
		return
	}

	response.Diagnostics.Append(data.refreshFromOutput(ctx, out)...)
	// data.Definition = fwtypes.NewListNestedObjectValueOfPtr(ctx, &definition{
	// 	Content:    flex.StringValueToFramework(ctx, *version.DefinitionContent),
	// 	S3Location: types.StringNull(),
	// })

	if response.Diagnostics.HasError() {
		return
	}

	response.Diagnostics.Append(response.State.Set(ctx, &data)...)

}

// Delete implements resource.ResourceWithConfigure.
func (r *resourceApplication) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	conn := r.Meta().M2Client(ctx)
	var state resourceApplicationData

	response.Diagnostics.Append(request.State.Get(ctx, &state)...)

	if response.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "deleting M2 Application", map[string]interface{}{
		"id": state.ApplicationId.ValueString(),
	})

	input := &m2.DeleteApplicationInput{
		ApplicationId: flex.StringFromFramework(ctx, state.ApplicationId),
	}

	_, err := tfresource.RetryWhenAWSErrCodeEquals(ctx, 5*time.Minute, func() (interface{}, error) {
		return conn.DeleteApplication(ctx, input)
	}, "DependencyViolation")

	if errs.IsA[*awstypes.ResourceNotFoundException](err) {
		return
	}

	if err != nil {
		response.Diagnostics.AddError(
			create.ProblemStandardMessage(names.M2, create.ErrActionDeleting, ResNameApplication, state.ApplicationId.ValueString(), err),
			err.Error(),
		)
		return
	}

	deleteTimeout := r.DeleteTimeout(ctx, state.Timeouts)
	_, err = waitApplicationDeleted(ctx, conn, state.ID.ValueString(), deleteTimeout)
	if err != nil {
		response.Diagnostics.AddError(
			create.ProblemStandardMessage(names.M2, create.ErrActionWaitingForDeletion, ResNameApplication, state.ID.String(), err),
			err.Error(),
		)
		return
	}

}

// Update implements resource.ResourceWithConfigure.
func (r *resourceApplication) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
	conn := r.Meta().M2Client(ctx)
	var state, plan resourceApplicationData

	response.Diagnostics.Append(request.State.Get(ctx, &state)...)

	if response.Diagnostics.HasError() {
		return
	}

	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)

	if response.Diagnostics.HasError() {
		return
	}

	if applicationHasChanges(ctx, plan, state) {
		input := &m2.UpdateApplicationInput{}
		response.Diagnostics.Append(flex.Expand(ctx, plan, input)...)

		if response.Diagnostics.HasError() {
			return
		}

		_, err := conn.UpdateApplication(ctx, input)

		if err != nil {
			response.Diagnostics.AddError(
				create.ProblemStandardMessage(names.M2, create.ErrActionUpdating, ResNameApplication, state.ApplicationId.ValueString(), err),
				err.Error(),
			)
			return
		}

		response.Diagnostics.Append(response.State.Set(ctx, &plan)...)

	}

	response.Diagnostics.Append(response.State.Set(ctx, &plan)...)

}
func (r *resourceApplication) ImportState(ctx context.Context, request resource.ImportStateRequest, response *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("application_id"), request, response)
}

func (r *resourceApplication) ModifyPlan(ctx context.Context, request resource.ModifyPlanRequest, response *resource.ModifyPlanResponse) {
	r.SetTagsAll(ctx, request, response)
}

type resourceApplicationData struct {
	ARN                types.String                                `tfsdk:"arn"`
	ApplicationId      types.String                                `tfsdk:"application_id"`
	ApplicationVersion types.Int64                                 `tfsdk:"application_version"`
	ClientToken        types.String                                `tfsdk:"client_token"`
	Definition         fwtypes.ListNestedObjectValueOf[definition] `tfsdk:"definition"`
	Description        types.String                                `tfsdk:"description"`
	//EngineType         fwtypes.StringEnum[awstypes.EngineType]     `tfsdk:"engine_type"`
	EngineType types.String   `tfsdk:"engine_type"`
	ID         types.String   `tfsdk:"id"`
	KmsKeyId   types.String   `tfsdk:"kms_key_id"`
	RoleARN    types.String   `tfsdk:"role_arn"`
	Name       types.String   `tfsdk:"name"`
	Tags       types.Map      `tfsdk:"tags"`
	TagsAll    types.Map      `tfsdk:"tags_all"`
	Timeouts   timeouts.Value `tfsdk:"timeouts"`
}

type definition struct {
	Content    types.String `tfsdk:"content"`
	S3Location types.String `tfsdk:"s3_location"`
}

func applicationHasChanges(_ context.Context, plan, state resourceApplicationData) bool {
	return !plan.EngineType.Equal(state.EngineType) ||
		!plan.Description.Equal(state.Description) ||
		!plan.KmsKeyId.Equal(state.KmsKeyId) ||
		!plan.Name.Equal(state.Name) ||
		!plan.RoleARN.Equal(state.RoleARN) ||
		!plan.Definition.Equal(state.Definition)

}

func (r *resourceApplicationData) refreshFromOutput(ctx context.Context, app *m2.GetApplicationOutput) diag.Diagnostics {
	var diags diag.Diagnostics

	//diags.Append(flex.Flatten(ctx, app, r)...)
	r.ARN = flex.StringToFramework(ctx, app.ApplicationArn)
	r.ID = flex.StringToFramework(ctx, app.ApplicationId)
	r.ApplicationVersion = flex.Int32ToFramework(ctx, app.LatestVersion.ApplicationVersion)
	r.Name = flex.StringToFramework(ctx, app.Name)
	r.EngineType = flex.StringToFramework(ctx, (*string)(&app.EngineType))
	return diags
}
