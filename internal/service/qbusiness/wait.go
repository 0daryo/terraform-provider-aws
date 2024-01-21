// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package qbusiness

import (
	"context"
	"errors"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/qbusiness"
	"github.com/aws/aws-sdk-go-v2/service/qbusiness/types"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/hashicorp/terraform-provider-aws/internal/enum"
	"github.com/hashicorp/terraform-provider-aws/internal/tfresource"
)

func waitApplicationCreated(ctx context.Context, conn *qbusiness.Client, id string, timeout time.Duration) (*qbusiness.GetApplicationOutput, error) {
	stateConf := &retry.StateChangeConf{
		Pending:    enum.Slice(types.ApplicationStatusCreating, types.ApplicationStatusUpdating),
		Target:     enum.Slice(types.ApplicationStatusActive),
		Refresh:    statusAppAvailability(ctx, conn, id),
		Timeout:    timeout,
		MinTimeout: 10 * time.Second,
	}

	outputRaw, err := stateConf.WaitForStateContext(ctx)

	if output, ok := outputRaw.(*qbusiness.GetApplicationOutput); ok {
		tfresource.SetLastError(err, errors.New(string(output.Status)))

		return output, err
	}
	return nil, err
}

func waitApplicationDeleted(ctx context.Context, conn *qbusiness.Client, id string, timeout time.Duration) (*qbusiness.GetApplicationOutput, error) {
	stateConf := &retry.StateChangeConf{
		Pending:    enum.Slice(types.ApplicationStatusActive, types.ApplicationStatusDeleting),
		Target:     []string{},
		Refresh:    statusAppAvailability(ctx, conn, id),
		Timeout:    timeout,
		MinTimeout: 10 * time.Second,
	}

	outputRaw, err := stateConf.WaitForStateContext(ctx)

	if output, ok := outputRaw.(*qbusiness.GetApplicationOutput); ok {
		tfresource.SetLastError(err, errors.New(string(output.Status)))

		return output, err
	}
	return nil, err
}

func waitWebexperienceCreated(ctx context.Context, conn *qbusiness.Client, id string, timeout time.Duration) (*qbusiness.GetWebExperienceOutput, error) {
	stateConf := &retry.StateChangeConf{
		Pending:    enum.Slice(types.WebExperienceStatusCreating),
		Target:     enum.Slice(types.WebExperienceStatusActive, types.WebExperienceStatusPendingAuthConfig),
		Refresh:    statusWebexperienceAvailability(ctx, conn, id),
		Timeout:    timeout,
		MinTimeout: 10 * time.Second,
	}

	outputRaw, err := stateConf.WaitForStateContext(ctx)

	if output, ok := outputRaw.(*qbusiness.GetWebExperienceOutput); ok {
		tfresource.SetLastError(err, errors.New(string(output.Status)))

		return output, err
	}
	return nil, err
}

func waitWebexperienceDeleted(ctx context.Context, conn *qbusiness.Client, id string, timeout time.Duration) (*qbusiness.GetWebExperienceOutput, error) {
	stateConf := &retry.StateChangeConf{
		Pending: enum.Slice(types.WebExperienceStatusActive, types.WebExperienceStatusDeleting,
			types.WebExperienceStatusPendingAuthConfig, types.WebExperienceStatusFailed),
		Target:     []string{},
		Refresh:    statusWebexperienceAvailability(ctx, conn, id),
		Timeout:    timeout,
		MinTimeout: 10 * time.Second,
	}

	outputRaw, err := stateConf.WaitForStateContext(ctx)

	if output, ok := outputRaw.(*qbusiness.GetWebExperienceOutput); ok {
		tfresource.SetLastError(err, errors.New(string(output.Status)))

		return output, err
	}
	return nil, err
}
