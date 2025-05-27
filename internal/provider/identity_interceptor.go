// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	"github.com/hashicorp/terraform-provider-aws/internal/errs/sdkdiag"
	"github.com/hashicorp/terraform-provider-aws/internal/provider/importer"
	tfslices "github.com/hashicorp/terraform-provider-aws/internal/slices"
	inttypes "github.com/hashicorp/terraform-provider-aws/internal/types"
	"github.com/hashicorp/terraform-provider-aws/names"
)

var _ crudInterceptor = identityInterceptor{}

type identityInterceptor struct {
	attributes []string
}

func (r identityInterceptor) run(ctx context.Context, opts crudInterceptorOptions) diag.Diagnostics {
	var diags diag.Diagnostics
	awsClient := opts.c

	switch d, when, why := opts.d, opts.when, opts.why; when {
	case After:
		switch why {
		case Create:
			identity, err := d.Identity()
			if err != nil {
				return sdkdiag.AppendFromErr(diags, err)
			}

			for _, attr := range r.attributes {
				switch attr {
				case names.AttrAccountID:
					if err := identity.Set(attr, awsClient.AccountID(ctx)); err != nil {
						return sdkdiag.AppendFromErr(diags, err)
					}

				case names.AttrRegion:
					if err := identity.Set(attr, awsClient.Region(ctx)); err != nil {
						return sdkdiag.AppendFromErr(diags, err)
					}

				default:
					val, ok := getAttributeOk(d, attr)
					if !ok {
						continue
					}
					if err := identity.Set(attr, val); err != nil {
						return sdkdiag.AppendFromErr(diags, err)
					}
				}
			}
		}
	}

	return diags
}

func getAttributeOk(d schemaResourceData, name string) (string, bool) {
	if name == "id" {
		return d.Id(), true
	}
	v, ok := d.GetOk(name)
	return v.(string), ok
}

func newIdentityInterceptor(attributes []inttypes.IdentityAttribute) interceptorInvocation {
	return interceptorInvocation{
		when: After,
		why:  Create, // TODO: probably need to do this after Read and Update as well
		interceptor: identityInterceptor{
			attributes: tfslices.ApplyToAll(attributes, func(v inttypes.IdentityAttribute) string {
				return v.Name
			}),
		},
	}
}

func newResourceIdentity(v inttypes.Identity) *schema.ResourceIdentity {
	return &schema.ResourceIdentity{
		SchemaFunc: func() map[string]*schema.Schema {
			return newIdentitySchema(v.Attributes)
		},
	}
}

func newIdentitySchema(attributes []inttypes.IdentityAttribute) map[string]*schema.Schema {
	identitySchema := make(map[string]*schema.Schema, len(attributes))
	for _, attr := range attributes {
		identitySchema[attr.Name] = newIdentityAttribute(attr)
	}
	return identitySchema
}

func newIdentityAttribute(attribute inttypes.IdentityAttribute) *schema.Schema {
	attr := &schema.Schema{
		Type: schema.TypeString,
	}
	if attribute.Required {
		attr.RequiredForImport = true
	} else {
		attr.OptionalForImport = true
	}
	return attr
}

func newParameterizedIdentityImporter(v inttypes.Identity) *schema.ResourceImporter {
	// if v.Singleton {
	// 	if v.Global {
	// 		return &schema.ResourceImporter{
	// 			StateContext: globalSingletonImporter,
	// 		}
	// 	} else {
	// 		return &schema.ResourceImporter{
	// 			StateContext: regionalSingletonImporter,
	// 		}
	// 	}
	// }

	importer := &schema.ResourceImporter{
		StateContext: func(ctx context.Context, rd *schema.ResourceData, meta any) ([]*schema.ResourceData, error) {
			if rd.Id() != "" {
				if v.IDAttrShadowsAttr != "id" {
					rd.Set(v.IDAttrShadowsAttr, rd.Id())
				}
				return []*schema.ResourceData{rd}, nil
			}

			identity, err := rd.Identity()
			if err != nil {
				return nil, err
			}

			for _, attr := range v.Attributes {
				var val string
				switch attr.Name {
				case names.AttrAccountID:
					accountIDRaw, ok := identity.GetOk(names.AttrAccountID)
					if ok {
						accountID, ok := accountIDRaw.(string)
						if !ok {
							return nil, fmt.Errorf("identity attribute %q: expected string, got %T", names.AttrAccountID, accountIDRaw)
						}
						client := meta.(*conns.AWSClient)
						if accountID != client.AccountID(ctx) {
							return nil, fmt.Errorf("Unable to import\n\nidentity attribute %q: Provider configured with Account ID %q, got %q", names.AttrAccountID, client.AccountID(ctx), accountID)
						}
					}

				case names.AttrRegion:
					regionRaw, ok := identity.GetOk(names.AttrRegion)
					if ok {
						val, ok = regionRaw.(string)
						if !ok {
							return nil, fmt.Errorf("identity attribute %q: expected string, got %T", names.AttrRegion, regionRaw)
						}
						rd.Set(names.AttrRegion, val)
					}

				default:
					valRaw, ok := identity.GetOk(attr.Name)
					if attr.Required && !ok {
						return nil, fmt.Errorf("identity attribute %q is required", attr.Name)
					}
					val, ok = valRaw.(string)
					if !ok {
						return nil, fmt.Errorf("identity attribute %q: expected string, got %T", attr.Name, valRaw)
					}
					setAttribute(rd, attr.Name, val)
				}

				if attr.Name == v.IDAttrShadowsAttr {
					rd.SetId(val)
				}
			}

			return []*schema.ResourceData{rd}, nil
		},
	}
	return importer
}

// func globalSingletonImporter(ctx context.Context, rd *schema.ResourceData, meta any) ([]*schema.ResourceData, error) {
// 	if rd.Id() != "" {
// 		rd.Set(names.AttrAccountID, rd.Id())
// 		return []*schema.ResourceData{rd}, nil
// 	}

// 	identity, err := rd.Identity()
// 	if err != nil {
// 		return nil, err
// 	}

// 	client := meta.(*conns.AWSClient)

// 	accountIDRaw, ok := identity.GetOk(names.AttrAccountID)
// 	var accountID string
// 	if ok {
// 		accountID, ok = accountIDRaw.(string)
// 		if !ok {
// 			return nil, fmt.Errorf("identity attribute %q: expected string, got %T", names.AttrAccountID, accountIDRaw)
// 		}
// 		if accountID != client.AccountID(ctx) {
// 			return nil, fmt.Errorf("Unable to import\n\nidentity attribute %q: Provider configured with Account ID %q, got %q", names.AttrAccountID, client.AccountID(ctx), accountID)
// 		}
// 	} else {
// 		accountID = client.AccountID(ctx)
// 	}
// 	rd.Set(names.AttrAccountID, accountID)
// 	rd.SetId(accountID)

// 	return []*schema.ResourceData{rd}, nil
// }

// func regionalSingletonImporter(ctx context.Context, rd *schema.ResourceData, meta any) ([]*schema.ResourceData, error) {
// 	if rd.Id() != "" {
// 		rd.Set("region", rd.Id())
// 		return []*schema.ResourceData{rd}, nil
// 	}

// 	identity, err := rd.Identity()
// 	if err != nil {
// 		return nil, err
// 	}

// 	client := meta.(*conns.AWSClient)

// 	accountIDRaw, ok := identity.GetOk(names.AttrAccountID)
// 	var accountID string
// 	if ok {
// 		accountID, ok = accountIDRaw.(string)
// 		if !ok {
// 			return nil, fmt.Errorf("identity attribute %q: expected string, got %T", names.AttrAccountID, accountIDRaw)
// 		}
// 		if accountID != client.AccountID(ctx) {
// 			return nil, fmt.Errorf("Unable to import\n\nidentity attribute %q: Provider configured with Account ID %q, got %q", names.AttrAccountID, client.AccountID(ctx), accountID)
// 		}
// 	}

// 	regionRaw, ok := identity.GetOk("region")
// 	var region string
// 	if ok {
// 		region, ok = regionRaw.(string)
// 		if !ok {
// 			return nil, fmt.Errorf("identity attribute %q: expected string, got %T", "region", regionRaw)
// 		}
// 	} else {
// 		region = client.Region(ctx)
// 	}
// 	rd.Set("region", region)
// 	rd.SetId(region)

// 	return []*schema.ResourceData{rd}, nil
// }

func setAttribute(d *schema.ResourceData, name, value string) {
	if name == "id" {
		d.SetId(value)
	} else {
		d.Set(name, value)
	}
}

func arnIdentityResourceImporter(attrName string, isGlobal bool) *schema.ResourceImporter {
	if isGlobal {
		return &schema.ResourceImporter{
			StateContext: func(ctx context.Context, rd *schema.ResourceData, meta any) ([]*schema.ResourceData, error) {
				if err := importer.GlobalARN(ctx, rd, attrName); err != nil {
					return nil, err
				}

				return []*schema.ResourceData{rd}, nil
			},
		}
	} else {
		return &schema.ResourceImporter{
			StateContext: func(ctx context.Context, rd *schema.ResourceData, meta any) ([]*schema.ResourceData, error) {
				if err := importer.RegionalARN(ctx, rd, attrName); err != nil {
					return nil, err
				}

				return []*schema.ResourceData{rd}, nil
			},
		}
	}
}

func singletonIdentityResourceImporter(isGlobal bool) *schema.ResourceImporter {
	if isGlobal {
		// Historically, we haven't validated *any* Import ID value for Global Singletons
		return &schema.ResourceImporter{
			StateContext: func(ctx context.Context, rd *schema.ResourceData, meta any) ([]*schema.ResourceData, error) {
				if err := importer.GlobalSingleton(ctx, rd, meta); err != nil {
					return nil, err
				}

				return []*schema.ResourceData{rd}, nil
			},
		}
	} else {
		return &schema.ResourceImporter{
			StateContext: func(ctx context.Context, rd *schema.ResourceData, meta any) ([]*schema.ResourceData, error) {
				if err := importer.RegionalSingleton(ctx, rd, meta); err != nil {
					return nil, err
				}

				return []*schema.ResourceData{rd}, nil
			},
		}
	}
}
