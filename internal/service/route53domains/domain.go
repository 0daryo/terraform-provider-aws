// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package route53domains

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53domains"
	awstypes "github.com/aws/aws-sdk-go-v2/service/route53domains/types"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-provider-aws/internal/errs"
	"github.com/hashicorp/terraform-provider-aws/internal/errs/fwdiag"
	"github.com/hashicorp/terraform-provider-aws/internal/framework"
	fwflex "github.com/hashicorp/terraform-provider-aws/internal/framework/flex"
	fwtypes "github.com/hashicorp/terraform-provider-aws/internal/framework/types"
	tftags "github.com/hashicorp/terraform-provider-aws/internal/tags"
	"github.com/hashicorp/terraform-provider-aws/internal/tfresource"
	"github.com/hashicorp/terraform-provider-aws/names"
)

// @FrameworkResource("aws_route53domains_domain", name="Domain")
// @Tags(identifierAttribute="domain_name")
func newDomainResource(context.Context) (resource.ResourceWithConfigure, error) {
	r := &domainResource{}

	r.SetDefaultCreateTimeout(30 * time.Minute)
	r.SetDefaultUpdateTimeout(30 * time.Minute)
	r.SetDefaultDeleteTimeout(30 * time.Minute)

	return r, nil
}

type domainResource struct {
	framework.ResourceWithConfigure
	framework.WithTimeouts
}

func (*domainResource) Metadata(_ context.Context, request resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = "aws_route53domains_domain"
}

func (r *domainResource) Schema(ctx context.Context, request resource.SchemaRequest, response *resource.SchemaResponse) {
	response.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"abuse_contact_email": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"abuse_contact_phone": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"admin_privacy": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(true),
			},
			"auto_renew": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(true),
			},
			"billing_contact": framework.ResourceOptionalComputedListOfObjectAttribute[contactDetailModel](ctx),
			"billing_privacy": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(true),
			},
			"creation_date": schema.StringAttribute{
				CustomType: timetypes.RFC3339Type{},
				Computed:   true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			names.AttrDomainName: schema.StringAttribute{
				CustomType: fwtypes.CaseInsensitiveStringType,
				Required:   true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"duration_in_years": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				Default:  int64default.StaticInt64(1),
				Validators: []validator.Int64{
					int64validator.Between(1, 10),
				},
			},
			"expiration_date": schema.StringAttribute{
				CustomType: timetypes.RFC3339Type{},
				Computed:   true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name_server": schema.ListAttribute{
				CustomType: fwtypes.NewListNestedObjectTypeOf[nameserverModel](ctx),
				Optional:   true,
				Computed:   true,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.List{
					listvalidator.SizeAtMost(6),
				},
				ElementType: types.ObjectType{
					AttrTypes: fwtypes.AttributeTypesMust[nameserverModel](ctx),
				},
			},
			"registrant_privacy": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(true),
			},
			"registrar_name": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"registrar_url": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"reseller": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"status_list": schema.ListAttribute{
				CustomType:  fwtypes.ListOfStringType,
				Computed:    true,
				ElementType: types.StringType,
			},
			names.AttrTags:    tftags.TagsAttribute(),
			names.AttrTagsAll: tftags.TagsAttributeComputedOnly(),
			"tech_privacy": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(true),
			},
			"transfer_lock": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(true),
			},
			"updated_date": schema.StringAttribute{
				CustomType: timetypes.RFC3339Type{},
				Computed:   true,
			},
			"whois_server": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
		Blocks: map[string]schema.Block{
			"admin_contact":      contactDetailBlock(ctx),
			"registrant_contact": contactDetailBlock(ctx),
			"tech_contact":       contactDetailBlock(ctx),
			names.AttrTimeouts: timeouts.Block(ctx, timeouts.Opts{
				Create: true,
				Update: true,
				Delete: true,
			}),
		},
	}
}

func contactDetailBlock(ctx context.Context) schema.Block {
	block := schema.ListNestedBlock{
		CustomType: fwtypes.NewListNestedObjectTypeOf[contactDetailModel](ctx),
		NestedObject: schema.NestedBlockObject{
			Attributes: map[string]schema.Attribute{
				"address_line_1": schema.StringAttribute{
					Optional: true,
					Validators: []validator.String{
						stringvalidator.LengthAtMost(255),
					},
				},
				"address_line_2": schema.StringAttribute{
					Optional: true,
					Validators: []validator.String{
						stringvalidator.LengthAtMost(255),
					},
				},
				"city": schema.StringAttribute{
					Optional: true,
					Validators: []validator.String{
						stringvalidator.LengthAtMost(255),
					},
				},
				"contact_type": schema.StringAttribute{
					CustomType: fwtypes.StringEnumType[awstypes.ContactType](),
					Optional:   true,
				},
				"country_code": schema.StringAttribute{
					CustomType: fwtypes.StringEnumType[awstypes.CountryCode](),
					Optional:   true,
				},
				names.AttrEmail: schema.StringAttribute{
					Optional: true,
					Validators: []validator.String{
						stringvalidator.LengthAtMost(254),
					},
				},
				"extra_params": schema.MapAttribute{
					CustomType:  fwtypes.MapOfStringType,
					Optional:    true,
					ElementType: types.StringType,
				},
				"fax": schema.StringAttribute{
					Optional: true,
					Validators: []validator.String{
						stringvalidator.LengthAtMost(30),
					},
				},
				"first_name": schema.StringAttribute{
					Optional: true,
					Validators: []validator.String{
						stringvalidator.LengthAtMost(255),
					},
				},
				"last_name": schema.StringAttribute{
					Optional: true,
					Validators: []validator.String{
						stringvalidator.LengthAtMost(255),
					},
				},
				"organization_name": schema.StringAttribute{
					Optional: true,
					Validators: []validator.String{
						stringvalidator.LengthAtMost(255),
					},
				},
				"phone_number": schema.StringAttribute{
					Optional: true,
					Validators: []validator.String{
						stringvalidator.LengthAtMost(30),
					},
				},
				"state": schema.StringAttribute{
					Optional: true,
					Validators: []validator.String{
						stringvalidator.LengthAtMost(255),
					},
				},
				"zip_code": schema.StringAttribute{
					Optional: true,
					Validators: []validator.String{
						stringvalidator.LengthAtMost(255),
					},
				},
			},
		},
		Validators: []validator.List{
			listvalidator.IsRequired(),
			listvalidator.SizeAtLeast(1),
			listvalidator.SizeAtMost(1),
		},
	}

	return block
}

func (r *domainResource) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	var data domainResourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &data)...)
	if response.Diagnostics.HasError() {
		return
	}

	conn := r.Meta().Route53DomainsClient(ctx)

	domainName := fwflex.StringValueFromFramework(ctx, data.DomainName)
	input := &route53domains.RegisterDomainInput{}
	response.Diagnostics.Append(fwflex.Expand(ctx, data, input)...)
	if response.Diagnostics.HasError() {
		return
	}

	input.PrivacyProtectAdminContact = fwflex.BoolFromFramework(ctx, data.AdminPrivacy)
	input.PrivacyProtectBillingContact = fwflex.BoolFromFramework(ctx, data.BillingPrivacy)
	input.PrivacyProtectRegistrantContact = fwflex.BoolFromFramework(ctx, data.RegistrantPrivacy)
	input.PrivacyProtectTechContact = fwflex.BoolFromFramework(ctx, data.TechPrivacy)

	output, err := conn.RegisterDomain(ctx, input)

	if err != nil {
		response.Diagnostics.AddError(fmt.Sprintf("creating Route 53 Domains Domain (%s)", domainName), err.Error())

		return
	}

	if _, err := waitOperationSucceeded(ctx, conn, aws.ToString(output.OperationId), r.CreateTimeout(ctx, data.Timeouts)); err != nil {
		response.State.SetAttribute(ctx, path.Root(names.AttrID), data.DomainName) // Set 'id' so as to taint the resource.
		response.Diagnostics.AddError(fmt.Sprintf("waiting for Route 53 Domains Domain (%s) create", domainName), err.Error())

		return
	}

	if err := createTags(ctx, conn, domainName, getTagsIn(ctx)); err != nil {
		response.State.SetAttribute(ctx, path.Root(names.AttrID), data.DomainName) // Set 'id' so as to taint the resource.
		response.Diagnostics.AddError(fmt.Sprintf("setting Route 53 Domains Domain (%s) tags", domainName), err.Error())

		return
	}

	// Set values for unknowns.
	domainDetail, err := findDomainDetailByName(ctx, conn, domainName)

	if err != nil {
		response.Diagnostics.AddError(fmt.Sprintf("reading Route 53 Domains Domain (%s)", domainName), err.Error())

		return
	}

	response.Diagnostics.Append(fwflex.Flatten(ctx, domainDetail, &data)...)
	if response.Diagnostics.HasError() {
		return
	}

	response.Diagnostics.Append(response.State.Set(ctx, data)...)
}

func (r *domainResource) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	var data domainResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &data)...)
	if response.Diagnostics.HasError() {
		return
	}

	conn := r.Meta().Route53DomainsClient(ctx)

	domainName := fwflex.StringValueFromFramework(ctx, data.DomainName)
	domainDetail, err := findDomainDetailByName(ctx, conn, domainName)

	if tfresource.NotFound(err) {
		response.Diagnostics.Append(fwdiag.NewResourceNotFoundWarningDiagnostic(err))
		response.State.RemoveResource(ctx)

		return
	}

	if err != nil {
		response.Diagnostics.AddError(fmt.Sprintf("reading Route 53 Domains Domain (%s)", domainName), err.Error())

		return
	}

	fixupContactDetail(domainDetail.AdminContact)
	fixupContactDetail(domainDetail.BillingContact)
	fixupContactDetail(domainDetail.RegistrantContact)
	fixupContactDetail(domainDetail.TechContact)

	// Set attributes for import.
	response.Diagnostics.Append(fwflex.Flatten(ctx, domainDetail, &data)...)
	if response.Diagnostics.HasError() {
		return
	}

	tfObject, d := flattenExtraParamsFramework(ctx, data.AdminContact, domainDetail.AdminContact)
	response.Diagnostics.Append(d...)
	if response.Diagnostics.HasError() {
		return
	}
	data.AdminContact = tfObject

	tfObject, d = flattenExtraParamsFramework(ctx, data.BillingContact, domainDetail.BillingContact)
	response.Diagnostics.Append(d...)
	if response.Diagnostics.HasError() {
		return
	}
	data.BillingContact = tfObject

	tfObject, d = flattenExtraParamsFramework(ctx, data.RegistrantContact, domainDetail.RegistrantContact)
	response.Diagnostics.Append(d...)
	if response.Diagnostics.HasError() {
		return
	}
	data.RegistrantContact = tfObject

	tfObject, d = flattenExtraParamsFramework(ctx, data.TechContact, domainDetail.TechContact)
	response.Diagnostics.Append(d...)
	if response.Diagnostics.HasError() {
		return
	}
	data.TechContact = tfObject

	response.Diagnostics.Append(response.State.Set(ctx, &data)...)
}

func (r *domainResource) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
	var old, new domainResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &old)...)
	if response.Diagnostics.HasError() {
		return
	}
	response.Diagnostics.Append(request.Plan.Get(ctx, &new)...)
	if response.Diagnostics.HasError() {
		return
	}

	response.Diagnostics.Append(response.State.Set(ctx, &new)...)
}

func (r *domainResource) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	var data domainResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &data)...)
	if response.Diagnostics.HasError() {
		return
	}

	conn := r.Meta().Route53DomainsClient(ctx)

	domainName := fwflex.StringValueFromFramework(ctx, data.DomainName)
	input := &route53domains.DeleteDomainInput{
		DomainName: aws.String(domainName),
	}

	output, err := conn.DeleteDomain(ctx, input)

	if errs.IsAErrorMessageContains[*awstypes.InvalidInput](err, "not found") {
		return
	}

	if err != nil {
		response.Diagnostics.AddError(fmt.Sprintf("deleting Route 53 Domains Domain (%s)", domainName), err.Error())

		return
	}

	if _, err := waitOperationSucceeded(ctx, conn, aws.ToString(output.OperationId), r.DeleteTimeout(ctx, data.Timeouts)); err != nil {
		response.Diagnostics.AddError(fmt.Sprintf("waiting for Route 53 Domains Domain (%s) delete", domainName), err.Error())

		return
	}

	// TODO Delete associated hosted zone:
	// % aws route53 list-hosted-zones
	// {
	// 	"HostedZones": [
	// 		{
	// 			"Id": "/hostedzone/Z04259161CEX7GET1UCX1",
	// 			"Name": "tf-acc-test-002.click.",
	// 			"CallerReference": "RISWorkflow-RD:0c5f36fb-7c78-4c40-8fb3-310d5961bdbe",
	// 			"Config": {
	// 				"Comment": "HostedZone created by Route53 Registrar",
	// 				"PrivateZone": false
	// 			},
	// 			"ResourceRecordSetCount": 2
	// 		}
	// 	]
	// }
}

func (r *domainResource) ImportState(ctx context.Context, request resource.ImportStateRequest, response *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root(names.AttrDomainName), request, response)
}

func (r *domainResource) ModifyPlan(ctx context.Context, request resource.ModifyPlanRequest, response *resource.ModifyPlanResponse) {
	r.SetTagsAll(ctx, request, response)
}

type domainResourceModel struct {
	AbuseContactEmail types.String                                        `tfsdk:"abuse_contact_email"`
	AbuseContactPhone types.String                                        `tfsdk:"abuse_contact_phone"`
	AdminContact      fwtypes.ListNestedObjectValueOf[contactDetailModel] `tfsdk:"admin_contact"`
	AdminPrivacy      types.Bool                                          `tfsdk:"admin_privacy"`
	AutoRenew         types.Bool                                          `tfsdk:"auto_renew"`
	BillingContact    fwtypes.ListNestedObjectValueOf[contactDetailModel] `tfsdk:"billing_contact"`
	BillingPrivacy    types.Bool                                          `tfsdk:"billing_privacy"`
	CreationDate      timetypes.RFC3339                                   `tfsdk:"creation_date"`
	DomainName        fwtypes.CaseInsensitiveString                       `tfsdk:"domain_name"`
	DurationInYears   types.Int64                                         `tfsdk:"duration_in_years"`
	ExpirationDate    timetypes.RFC3339                                   `tfsdk:"expiration_date"`
	NameServers       fwtypes.ListNestedObjectValueOf[nameserverModel]    `tfsdk:"name_server"`
	RegistrantContact fwtypes.ListNestedObjectValueOf[contactDetailModel] `tfsdk:"registrant_contact"`
	RegistrantPrivacy types.Bool                                          `tfsdk:"registrant_privacy"`
	RegistrarName     types.String                                        `tfsdk:"registrar_name"`
	RegistrarURL      types.String                                        `tfsdk:"registrar_url"`
	Reseller          types.String                                        `tfsdk:"reseller"`
	StatusList        fwtypes.ListOfString                                `tfsdk:"status_list"`
	Tags              tftags.Map                                          `tfsdk:"tags"`
	TagsAll           tftags.Map                                          `tfsdk:"tags_all"`
	TechContact       fwtypes.ListNestedObjectValueOf[contactDetailModel] `tfsdk:"tech_contact"`
	TechPrivacy       types.Bool                                          `tfsdk:"tech_privacy"`
	Timeouts          timeouts.Value                                      `tfsdk:"timeouts"`
	TransferLock      types.Bool                                          `tfsdk:"transfer_lock"`
	UpdatedDate       timetypes.RFC3339                                   `tfsdk:"updated_date"`
	WhoIsServer       types.String                                        `tfsdk:"whois_server"`
}

type contactDetailModel struct {
	AddressLine1     types.String                             `tfsdk:"address_line_1"`
	AddressLine2     types.String                             `tfsdk:"address_line_2"`
	City             types.String                             `tfsdk:"city"`
	ContactType      fwtypes.StringEnum[awstypes.ContactType] `tfsdk:"contact_type"`
	CountryCode      fwtypes.StringEnum[awstypes.CountryCode] `tfsdk:"country_code"`
	Email            types.String                             `tfsdk:"email"`
	ExtraParams      fwtypes.MapOfString                      `tfsdk:"extra_params" autoflex:"-"`
	Fax              types.String                             `tfsdk:"fax"`
	FirstName        types.String                             `tfsdk:"first_name"`
	LastName         types.String                             `tfsdk:"last_name"`
	OrganizationName types.String                             `tfsdk:"organization_name"`
	PhoneNumber      types.String                             `tfsdk:"phone_number"`
	State            types.String                             `tfsdk:"state"`
	ZipCode          types.String                             `tfsdk:"zip_code"`
}

type nameserverModel struct {
	GlueIPs fwtypes.SetOfString `tfsdk:"glue_ips"`
	Name    types.String        `tfsdk:"name"`
}

// API response empty contact detail strings are converted to nil.
func fixupContactDetail(apiObject *awstypes.ContactDetail) {
	if apiObject == nil {
		return
	}

	if aws.ToString(apiObject.AddressLine2) == "" {
		apiObject.AddressLine2 = nil
	}
}

func flattenExtraParamsFramework(ctx context.Context, tfObject fwtypes.ListNestedObjectValueOf[contactDetailModel], apiObject *awstypes.ContactDetail) (fwtypes.ListNestedObjectValueOf[contactDetailModel], diag.Diagnostics) {
	var diags diag.Diagnostics

	if apiObject == nil {
		return fwtypes.NewListNestedObjectValueOfNull[contactDetailModel](ctx), diags
	}

	if len(apiObject.ExtraParams) == 0 {
		return tfObject, diags
	}

	data, d := tfObject.ToPtr(ctx)
	diags = append(diags, d...)
	if diags.HasError() {
		return fwtypes.NewListNestedObjectValueOfNull[contactDetailModel](ctx), diags
	}

	elements := make(map[string]attr.Value, len(apiObject.ExtraParams))
	for _, apiObject := range apiObject.ExtraParams {
		elements[string(apiObject.Name)] = fwflex.StringToFramework(ctx, apiObject.Value)
	}

	m, d := fwtypes.NewMapValueOf[types.String](ctx, elements)
	diags = append(diags, d...)
	if diags.HasError() {
		return fwtypes.NewListNestedObjectValueOfNull[contactDetailModel](ctx), diags
	}

	data.ExtraParams = m

	tfObject, d = fwtypes.NewListNestedObjectValueOfPtr(ctx, data)
	diags = append(diags, d...)
	if diags.HasError() {
		return fwtypes.NewListNestedObjectValueOfNull[contactDetailModel](ctx), diags
	}

	return tfObject, diags
}
