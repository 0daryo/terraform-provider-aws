// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package m2

import (
	"context"
	"errors"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/m2"
	awstypes "github.com/aws/aws-sdk-go-v2/service/m2/types"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/id"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/hashicorp/terraform-provider-aws/internal/create"
	"github.com/hashicorp/terraform-provider-aws/internal/enum"
	"github.com/hashicorp/terraform-provider-aws/internal/errs"
	"github.com/hashicorp/terraform-provider-aws/internal/framework"
	"github.com/hashicorp/terraform-provider-aws/internal/framework/flex"
	fwtypes "github.com/hashicorp/terraform-provider-aws/internal/framework/types"
	tftags "github.com/hashicorp/terraform-provider-aws/internal/tags"
	"github.com/hashicorp/terraform-provider-aws/internal/tfresource"
	"github.com/hashicorp/terraform-provider-aws/names"
)

// Function annotations are used for resource registration to the Provider. DO NOT EDIT.
// @FrameworkResource(name="Environment")
// @Tags(identifierAttribute="arn")
func newResourceEnvironment(_ context.Context) (resource.ResourceWithConfigure, error) {
	r := &resourceEnvironment{}

	r.SetDefaultCreateTimeout(30 * time.Minute)
	r.SetDefaultUpdateTimeout(30 * time.Minute)
	r.SetDefaultDeleteTimeout(30 * time.Minute)

	return r, nil
}

const (
	ResNameEnvironment = "Environment"
)

type resourceEnvironment struct {
	framework.ResourceWithConfigure
	framework.WithTimeouts
}

func (r *resourceEnvironment) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "aws_m2_environment"
}

func (r *resourceEnvironment) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"arn": framework.ARNAttributeComputedOnly(),
			"apply_changes_during_maintenance_window": schema.BoolAttribute{
				Optional: true,
			},
			"client_token": schema.StringAttribute{
				Optional: true,
			},
			"description": schema.StringAttribute{
				Optional: true,
			},
			"id": framework.IDAttribute(),
			"engine_type": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"engine_version": schema.StringAttribute{
				Optional: true,
				Computed: true,
			},
			"force_update": schema.BoolAttribute{
				Optional: true,
			},
			"instance_type": schema.StringAttribute{
				Required:   true,
				Validators: []validator.String{},
			},
			"kms_key_id": schema.StringAttribute{
				Optional: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"load_balancer_arn": schema.StringAttribute{
				Computed: true,
			},
			"name": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"preferred_maintenance_window": schema.StringAttribute{
				Optional: true,
				Computed: true,
			},
			"security_groups": schema.SetAttribute{
				Required:    true,
				ElementType: types.StringType,
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
				},
			},
			"subnet_ids": schema.SetAttribute{
				Required:    true,
				ElementType: types.StringType,
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
				},
			},
			names.AttrTags:    tftags.TagsAttribute(),
			names.AttrTagsAll: tftags.TagsAttributeComputedOnly(),
		},
		Blocks: map[string]schema.Block{
			"storage_configuration": schema.ListNestedBlock{
				Validators: []validator.List{
					listvalidator.SizeAtMost(1),
				},
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
				NestedObject: schema.NestedBlockObject{
					Blocks: map[string]schema.Block{
						"efs": schema.ListNestedBlock{
							Validators: []validator.List{
								listvalidator.SizeAtMost(1),
							},
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"file_system_id": schema.StringAttribute{
										Required: true,
									},
									"mount_point": schema.StringAttribute{
										Required: true,
									},
								},
							},
						},
						"fsx": schema.ListNestedBlock{
							Validators: []validator.List{
								listvalidator.SizeAtMost(1),
							},
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"file_system_id": schema.StringAttribute{
										Required: true,
									},
									"mount_point": schema.StringAttribute{
										Required: true,
									},
								},
							},
						},
					},
				},
			},
			"high_availability_config": schema.ListNestedBlock{
				CustomType: fwtypes.NewListNestedObjectTypeOf[haData](ctx),
				Validators: []validator.List{
					listvalidator.SizeAtMost(1),
				},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"desired_capacity": schema.Int64Attribute{
							Required: true,
						},
					},
				},
			},
			"timeouts": timeouts.Block(ctx, timeouts.Opts{
				Create: true,
				Update: true,
				Delete: true,
			}),
		},
	}
}

func (r *resourceEnvironment) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	conn := r.Meta().M2Client(ctx)

	var plan resourceEnvironmentData
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	in := &m2.CreateEnvironmentInput{}

	resp.Diagnostics.Append(flex.Expand(ctx, plan, in)...)

	var clientToken string
	if plan.ClientToken.IsNull() || plan.ClientToken.IsUnknown() {
		clientToken = id.UniqueId()
	} else {
		clientToken = plan.ClientToken.ValueString()
	}

	in.ClientToken = aws.String(clientToken)

	in.Tags = getTagsIn(ctx)

	if !plan.StorageConfiguration.IsNull() {
		var sc []storageData
		resp.Diagnostics.Append(plan.StorageConfiguration.ElementsAs(ctx, &sc, false)...)
		storageConfig, d := expandStorageConfigurations(ctx, sc)
		resp.Diagnostics.Append(d...)
		in.StorageConfigurations = storageConfig
	}

	if resp.Diagnostics.HasError() {
		return
	}

	out, err := conn.CreateEnvironment(ctx, in)
	if err != nil {
		resp.Diagnostics.AddError(
			create.ProblemStandardMessage(names.M2, create.ErrActionCreating, ResNameEnvironment, plan.Name.String(), err),
			err.Error(),
		)
		return
	}
	if out == nil || out.EnvironmentId == nil {
		resp.Diagnostics.AddError(
			create.ProblemStandardMessage(names.M2, create.ErrActionCreating, ResNameEnvironment, plan.Name.String(), nil),
			errors.New("empty output").Error(),
		)
		return
	}

	plan.ID = flex.StringToFramework(ctx, out.EnvironmentId)

	createTimeout := r.CreateTimeout(ctx, plan.Timeouts)
	env, err := waitEnvironmentCreated(ctx, conn, plan.ID.ValueString(), createTimeout)
	if err != nil {
		resp.Diagnostics.AddError(
			create.ProblemStandardMessage(names.M2, create.ErrActionWaitingForCreation, ResNameEnvironment, plan.Name.String(), err),
			err.Error(),
		)
		return
	}

	resp.Diagnostics.Append(plan.refreshFromOutput(ctx, env)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *resourceEnvironment) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	conn := r.Meta().M2Client(ctx)

	var state resourceEnvironmentData
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	out, err := findEnvironmentByID(ctx, conn, state.ID.ValueString())
	if tfresource.NotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError(
			create.ProblemStandardMessage(names.M2, create.ErrActionSetting, ResNameEnvironment, state.ID.String(), err),
			err.Error(),
		)
		return
	}

	resp.Diagnostics.Append(state.refreshFromOutput(ctx, out)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *resourceEnvironment) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	conn := r.Meta().M2Client(ctx)

	var plan, state resourceEnvironmentData
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	in := &m2.UpdateEnvironmentInput{
		EnvironmentId: flex.StringFromFramework(ctx, plan.ID),
	}

	if r.hasChangesForMaintenance(plan, state) {
		in.ApplyDuringMaintenanceWindow = true
		in.EngineVersion = flex.StringFromFramework(ctx, plan.EngineVersion)
	} else if r.hasChanges(plan, state) {
		if !plan.EngineVersion.Equal(state.EngineVersion) {
			in.EngineVersion = flex.StringFromFramework(ctx, plan.EngineVersion)
		}
		if !plan.InstanceType.Equal(state.InstanceType) {
			in.InstanceType = flex.StringFromFramework(ctx, plan.InstanceType)
		}
		if !plan.PreferredMaintenanceWindow.Equal(state.PreferredMaintenanceWindow) {
			in.PreferredMaintenanceWindow = flex.StringFromFramework(ctx, plan.PreferredMaintenanceWindow)
		}

		if !plan.HighAvailabilityConfig.Equal(state.HighAvailabilityConfig) {
			v, d := plan.HighAvailabilityConfig.ToSlice(ctx)
			resp.Diagnostics.Append(d...)
			if len(v) > 0 {
				in.DesiredCapacity = flex.Int32FromFramework(ctx, v[0].DesiredCapacity)
			}
		}
	} else {
		return
	}

	if !plan.ForceUpdate.IsNull() {
		in.ForceUpdate = plan.ForceUpdate.ValueBool()
	}

	out, err := conn.UpdateEnvironment(ctx, in)
	if err != nil {
		resp.Diagnostics.AddError(
			create.ProblemStandardMessage(names.M2, create.ErrActionUpdating, ResNameEnvironment, plan.ID.String(), err),
			err.Error(),
		)
		return
	}
	if out == nil || out.EnvironmentId == nil {
		resp.Diagnostics.AddError(
			create.ProblemStandardMessage(names.M2, create.ErrActionUpdating, ResNameEnvironment, plan.ID.String(), nil),
			errors.New("empty output").Error(),
		)
		return
	}

	plan.ID = flex.StringToFramework(ctx, out.EnvironmentId)

	updateTimeout := r.UpdateTimeout(ctx, plan.Timeouts)
	env, err := waitEnvironmentUpdated(ctx, conn, plan.ID.ValueString(), updateTimeout)
	if err != nil {
		resp.Diagnostics.AddError(
			create.ProblemStandardMessage(names.M2, create.ErrActionWaitingForUpdate, ResNameEnvironment, plan.ID.String(), err),
			err.Error(),
		)
		return
	}

	resp.Diagnostics.Append(flex.Flatten(ctx, env, &plan)...)

	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *resourceEnvironment) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	conn := r.Meta().M2Client(ctx)

	var state resourceEnvironmentData
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	in := &m2.DeleteEnvironmentInput{
		EnvironmentId: aws.String(state.ID.ValueString()),
	}

	_, err := conn.DeleteEnvironment(ctx, in)
	if err != nil {
		if errs.IsA[*awstypes.ResourceNotFoundException](err) {
			return
		}
		resp.Diagnostics.AddError(
			create.ProblemStandardMessage(names.M2, create.ErrActionDeleting, ResNameEnvironment, state.ID.String(), err),
			err.Error(),
		)
		return
	}

	deleteTimeout := r.DeleteTimeout(ctx, state.Timeouts)
	_, err = waitEnvironmentDeleted(ctx, conn, state.ID.ValueString(), deleteTimeout)
	if err != nil {
		resp.Diagnostics.AddError(
			create.ProblemStandardMessage(names.M2, create.ErrActionWaitingForDeletion, ResNameEnvironment, state.ID.String(), err),
			err.Error(),
		)
		return
	}
}

func (r *resourceEnvironment) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func waitEnvironmentCreated(ctx context.Context, conn *m2.Client, id string, timeout time.Duration) (*m2.GetEnvironmentOutput, error) {
	stateConf := &retry.StateChangeConf{
		Pending:                   enum.Slice(awstypes.EnvironmentLifecycleCreating),
		Target:                    enum.Slice(awstypes.EnvironmentLifecycleAvailable),
		Refresh:                   statusEnvironment(ctx, conn, id),
		Timeout:                   timeout,
		NotFoundChecks:            20,
		ContinuousTargetOccurence: 2,
	}

	outputRaw, err := stateConf.WaitForStateContext(ctx)
	if out, ok := outputRaw.(*m2.GetEnvironmentOutput); ok {
		return out, err
	}

	return nil, err
}

func waitEnvironmentUpdated(ctx context.Context, conn *m2.Client, id string, timeout time.Duration) (*m2.GetEnvironmentOutput, error) {
	stateConf := &retry.StateChangeConf{
		Pending:                   enum.Slice(awstypes.EnvironmentLifecycleUpdating),
		Target:                    enum.Slice(awstypes.EnvironmentLifecycleAvailable),
		Refresh:                   statusEnvironment(ctx, conn, id),
		Timeout:                   timeout,
		NotFoundChecks:            20,
		ContinuousTargetOccurence: 2,
	}

	outputRaw, err := stateConf.WaitForStateContext(ctx)
	if out, ok := outputRaw.(*m2.GetEnvironmentOutput); ok {
		return out, err
	}

	return nil, err
}

func waitEnvironmentDeleted(ctx context.Context, conn *m2.Client, id string, timeout time.Duration) (*m2.GetEnvironmentOutput, error) {
	stateConf := &retry.StateChangeConf{
		Pending: enum.Slice(awstypes.EnvironmentLifecycleAvailable, awstypes.EnvironmentLifecycleCreating, awstypes.EnvironmentLifecycleDeleting),
		Target:  []string{},
		Refresh: statusEnvironment(ctx, conn, id),
		Timeout: timeout,
	}

	outputRaw, err := stateConf.WaitForStateContext(ctx)
	if out, ok := outputRaw.(*m2.GetEnvironmentOutput); ok {
		return out, err
	}

	return nil, err
}

func statusEnvironment(ctx context.Context, conn *m2.Client, id string) retry.StateRefreshFunc {
	return func() (interface{}, string, error) {
		out, err := findEnvironmentByID(ctx, conn, id)
		if tfresource.NotFound(err) {
			return nil, "", nil
		}

		if err != nil {
			return nil, "", err
		}

		return out, string(out.Status), nil
	}
}

func findEnvironmentByID(ctx context.Context, conn *m2.Client, id string) (*m2.GetEnvironmentOutput, error) {
	in := &m2.GetEnvironmentInput{
		EnvironmentId: aws.String(id),
	}

	out, err := conn.GetEnvironment(ctx, in)
	if err != nil {
		if errs.IsA[*awstypes.ResourceNotFoundException](err) {
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

func (rd *resourceEnvironmentData) refreshFromOutput(ctx context.Context, out *m2.GetEnvironmentOutput) diag.Diagnostics {
	var diags diag.Diagnostics

	diags.Append(flex.Flatten(ctx, out, rd)...)
	rd.ARN = flex.StringToFramework(ctx, out.EnvironmentArn)
	rd.ID = flex.StringToFramework(ctx, out.EnvironmentId)
	storage, d := flattenStorageConfigurations(ctx, out.StorageConfigurations)
	diags.Append(d...)
	rd.StorageConfiguration = storage

	return diags
}

type resourceEnvironmentData struct {
	ARN                          types.String                            `tfsdk:"arn"`
	ApplyDuringMaintenanceWindow types.Bool                              `tfsdk:"apply_changes_during_maintenance_window"`
	ClientToken                  types.String                            `tfsdk:"client_token"`
	Description                  types.String                            `tfsdk:"description"`
	ID                           types.String                            `tfsdk:"id"`
	EngineType                   types.String                            `tfsdk:"engine_type"`
	EngineVersion                types.String                            `tfsdk:"engine_version"`
	ForceUpdate                  types.Bool                              `tfsdk:"force_update"`
	HighAvailabilityConfig       fwtypes.ListNestedObjectValueOf[haData] `tfsdk:"high_availability_config"`
	InstanceType                 types.String                            `tfsdk:"instance_type"`
	KmsKeyId                     types.String                            `tfsdk:"kms_key_id"`
	LoadBalancerArn              types.String                            `tfsdk:"load_balancer_arn"`
	PreferredMaintenanceWindow   types.String                            `tfsdk:"preferred_maintenance_window"`
	SecurityGroupIds             types.Set                               `tfsdk:"security_groups"`
	StorageConfiguration         types.List                              `tfsdk:"storage_configuration"`
	SubnetIds                    types.Set                               `tfsdk:"subnet_ids"`
	Name                         types.String                            `tfsdk:"name"`
	Tags                         types.Map                               `tfsdk:"tags"`
	TagsAll                      types.Map                               `tfsdk:"tags_all"`
	Timeouts                     timeouts.Value                          `tfsdk:"timeouts"`
}

type storageData struct {
	EFS types.List `tfsdk:"efs"`
	FSX types.List `tfsdk:"fsx"`
}

type efs struct {
	FileSystemId types.String `tfsdk:"file_system_id"`
	MountPoint   types.String `tfsdk:"mount_point"`
}

type fsx struct {
	FileSystemId types.String `tfsdk:"file_system_id"`
	MountPoint   types.String `tfsdk:"mount_point"`
}

type haData struct {
	DesiredCapacity types.Int64 `tfsdk:"desired_capacity"`
}

var (
	storageDataAttrTypes = map[string]attr.Type{
		"efs": types.ListType{ElemType: mountObjectType},
		"fsx": types.ListType{ElemType: mountObjectType},
	}

	mountObjectType = types.ObjectType{AttrTypes: mountAttrTypes}

	mountAttrTypes = map[string]attr.Type{
		"file_system_id": types.StringType,
		"mount_point":    types.StringType,
	}
)

func (r *resourceEnvironment) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	r.SetTagsAll(ctx, req, resp)
}

func expandStorageConfigurations(ctx context.Context, storageConfigurations []storageData) ([]awstypes.StorageConfiguration, diag.Diagnostics) {
	storage := []awstypes.StorageConfiguration{}
	var diags diag.Diagnostics

	for _, mount := range storageConfigurations {
		if !mount.EFS.IsNull() {
			var efsMounts []efs
			diags.Append(mount.EFS.ElementsAs(ctx, &efsMounts, false)...)
			mp := expandEFSMountPoint(ctx, efsMounts)
			storage = append(storage, mp)
		}
		if !mount.FSX.IsNull() {
			var fsxMounts []fsx
			diags.Append(mount.FSX.ElementsAs(ctx, &fsxMounts, false)...)
			mp := expandFsxMountPoint(ctx, fsxMounts)
			storage = append(storage, mp)
		}
	}

	return storage, diags
}

func expandEFSMountPoint(ctx context.Context, efs []efs) *awstypes.StorageConfigurationMemberEfs {
	if len(efs) == 0 {
		return nil
	}
	return &awstypes.StorageConfigurationMemberEfs{
		Value: awstypes.EfsStorageConfiguration{
			FileSystemId: flex.StringFromFramework(ctx, efs[0].FileSystemId),
			MountPoint:   flex.StringFromFramework(ctx, efs[0].MountPoint),
		},
	}
}

func expandFsxMountPoint(ctx context.Context, fsx []fsx) *awstypes.StorageConfigurationMemberFsx {
	if len(fsx) == 0 {
		return nil
	}
	return &awstypes.StorageConfigurationMemberFsx{
		Value: awstypes.FsxStorageConfiguration{
			FileSystemId: flex.StringFromFramework(ctx, fsx[0].FileSystemId),
			MountPoint:   flex.StringFromFramework(ctx, fsx[0].MountPoint),
		},
	}
}

func flattenStorageConfigurations(ctx context.Context, apiObject []awstypes.StorageConfiguration) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	elemType := types.ObjectType{AttrTypes: storageDataAttrTypes}

	elems := []attr.Value{}

	for _, config := range apiObject {
		switch v := config.(type) {
		case *awstypes.StorageConfigurationMemberEfs:
			mountPoint, d := flattenMountPoint(ctx, v.Value.FileSystemId, v.Value.MountPoint, "efs")
			elems = append(elems, mountPoint)
			diags.Append(d...)

		case *awstypes.StorageConfigurationMemberFsx:
			mountPoint, d := flattenMountPoint(ctx, v.Value.FileSystemId, v.Value.MountPoint, "fsx")
			elems = append(elems, mountPoint)
			diags.Append(d...)
		}
	}
	listVal, d := types.ListValue(elemType, elems)
	diags.Append(d...)

	return listVal, diags
}

func flattenMountPoint(ctx context.Context, fileSystemId, mountPoint *string, mountType string) (attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics

	obj := map[string]attr.Value{
		"file_system_id": flex.StringToFramework(ctx, fileSystemId),
		"mount_point":    flex.StringToFramework(ctx, mountPoint),
	}

	mountValue, d := types.ObjectValue(mountAttrTypes, obj)
	diags.Append(d...)
	if diags.HasError() {
		return nil, diags
	}

	mountList := []attr.Value{
		mountValue,
	}

	mountListValue, d := types.ListValue(mountObjectType, mountList)
	diags.Append(d...)
	if diags.HasError() {
		return nil, diags
	}

	configMap := map[string]attr.Value{
		mountType: mountListValue,
	}

	for k := range storageDataAttrTypes {
		if k != mountType {
			configMap[k] = types.ListNull(mountObjectType)
		}
	}

	configValue, d := types.ObjectValue(storageDataAttrTypes, configMap)
	diags.Append(d...)

	return configValue, diags
}

func (r *resourceEnvironment) hasChanges(plan, state resourceEnvironmentData) bool {
	return !plan.HighAvailabilityConfig.Equal(state.HighAvailabilityConfig) ||
		!plan.EngineVersion.Equal(state.EngineVersion) ||
		!plan.InstanceType.Equal(state.EngineType) ||
		!plan.PreferredMaintenanceWindow.Equal(state.PreferredMaintenanceWindow)
}

func (r *resourceEnvironment) hasChangesForMaintenance(plan, state resourceEnvironmentData) bool {
	return plan.ApplyDuringMaintenanceWindow.ValueBool() && !plan.EngineVersion.Equal(state.EngineVersion)
}
