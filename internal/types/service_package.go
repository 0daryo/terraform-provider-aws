// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package types

import (
	"context"
	"unique"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// ServicePackageResourceRegion represents resource-level Region information.
type ServicePackageResourceRegion struct {
	IsOverrideEnabled             bool // Is per-resource Region override supported?
	IsValidateOverrideInPartition bool // Is the per-resource Region override value validated againt the configured partition?
}

// ResourceRegionDefault returns the default resource region configuration.
// The default is to enable per-resource Region override and validate the override value.
func ResourceRegionDefault() ServicePackageResourceRegion {
	return ServicePackageResourceRegion{
		IsOverrideEnabled:             true,
		IsValidateOverrideInPartition: true,
	}
}

// ResourceRegionDisabled returns the resource region configuration indicating that there is no per-resource Region override.
func ResourceRegionDisabled() ServicePackageResourceRegion {
	return ServicePackageResourceRegion{}
}

// ServicePackageResourceTags represents resource-level tagging information.
type ServicePackageResourceTags struct {
	IdentifierAttribute string // The attribute for the identifier for UpdateTags etc.
	ResourceType        string // Extra resourceType parameter value for UpdateTags etc.
}

// ServicePackageEphemeralResource represents a Terraform Plugin Framework ephemeral resource
// implemented by a service package.
type ServicePackageEphemeralResource struct {
	Factory  func(context.Context) (ephemeral.EphemeralResourceWithConfigure, error)
	TypeName string
	Name     string
	Region   unique.Handle[ServicePackageResourceRegion]
}

// ServicePackageFrameworkDataSource represents a Terraform Plugin Framework data source
// implemented by a service package.
type ServicePackageFrameworkDataSource struct {
	Factory  func(context.Context) (datasource.DataSourceWithConfigure, error)
	TypeName string
	Name     string
	Tags     unique.Handle[ServicePackageResourceTags]
	Region   unique.Handle[ServicePackageResourceRegion]
}

// ServicePackageFrameworkResource represents a Terraform Plugin Framework resource
// implemented by a service package.
type ServicePackageFrameworkResource struct {
	Factory  func(context.Context) (resource.ResourceWithConfigure, error)
	TypeName string
	Name     string
	Tags     unique.Handle[ServicePackageResourceTags]
	Region   unique.Handle[ServicePackageResourceRegion]
	Identity Identity
}

// ServicePackageSDKDataSource represents a Terraform Plugin SDK data source
// implemented by a service package.
type ServicePackageSDKDataSource struct {
	Factory  func() *schema.Resource
	TypeName string
	Name     string
	Tags     unique.Handle[ServicePackageResourceTags]
	Region   unique.Handle[ServicePackageResourceRegion]
}

// ServicePackageSDKResource represents a Terraform Plugin SDK resource
// implemented by a service package.
type ServicePackageSDKResource struct {
	Factory  func() *schema.Resource
	TypeName string
	Name     string
	Tags     unique.Handle[ServicePackageResourceTags]
	Region   unique.Handle[ServicePackageResourceRegion]
	Identity Identity
	Import   Import
}

type Identity struct {
	Global       bool
	Singleton    bool
	ARN          bool
	ARNAttribute string
}

func RegionalSingletonIdentity() Identity {
	return Identity{
		Global:    false,
		Singleton: true,
	}
}

func GlobalARNIdentity() Identity {
	return arnIdentity(true, "arn")
}

func RegionalARNIdentity() Identity {
	return RegionalARNIdentityNamed("arn")
}

func RegionalARNIdentityNamed(name string) Identity {
	return arnIdentity(false, name)
}

func arnIdentity(isGlobal bool, name string) Identity {
	return Identity{
		Global:       isGlobal,
		ARN:          true,
		ARNAttribute: name,
	}
}

type Import struct {
	WrappedImport bool
}
