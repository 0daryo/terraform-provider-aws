// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package amp

import (
	"context"
	"errors"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/amp"
	"github.com/aws/aws-sdk-go-v2/service/amp/types"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/prometheusservice"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/hashicorp/terraform-provider-aws/internal/enum"
	"github.com/hashicorp/terraform-provider-aws/internal/tfresource"
)

const (
	// Maximum amount of time to wait for a Workspace to be created, updated, or deleted
	workspaceTimeout = 5 * time.Minute
)

func waitRuleGroupNamespaceDeleted(ctx context.Context, conn *prometheusservice.PrometheusService, id string) (*prometheusservice.RuleGroupsNamespaceDescription, error) {
	stateConf := &retry.StateChangeConf{
		Pending: []string{prometheusservice.RuleGroupsNamespaceStatusCodeDeleting},
		Target:  []string{},
		Refresh: statusRuleGroupNamespace(ctx, conn, id),
		Timeout: workspaceTimeout,
	}

	outputRaw, err := stateConf.WaitForStateContext(ctx)

	if output, ok := outputRaw.(*prometheusservice.RuleGroupsNamespaceDescription); ok {
		return output, err
	}

	return nil, err
}

func waitRuleGroupNamespaceCreated(ctx context.Context, conn *prometheusservice.PrometheusService, id string) (*prometheusservice.RuleGroupsNamespaceDescription, error) {
	stateConf := &retry.StateChangeConf{
		Pending: []string{prometheusservice.RuleGroupsNamespaceStatusCodeCreating},
		Target:  []string{prometheusservice.RuleGroupsNamespaceStatusCodeActive},
		Refresh: statusRuleGroupNamespace(ctx, conn, id),
		Timeout: workspaceTimeout,
	}

	outputRaw, err := stateConf.WaitForStateContext(ctx)

	if output, ok := outputRaw.(*prometheusservice.RuleGroupsNamespaceDescription); ok {
		return output, err
	}

	return nil, err
}

func waitRuleGroupNamespaceUpdated(ctx context.Context, conn *prometheusservice.PrometheusService, id string) (*prometheusservice.RuleGroupsNamespaceDescription, error) {
	stateConf := &retry.StateChangeConf{
		Pending: []string{prometheusservice.RuleGroupsNamespaceStatusCodeUpdating},
		Target:  []string{prometheusservice.RuleGroupsNamespaceStatusCodeActive},
		Refresh: statusRuleGroupNamespace(ctx, conn, id),
		Timeout: workspaceTimeout,
	}

	outputRaw, err := stateConf.WaitForStateContext(ctx)

	if output, ok := outputRaw.(*prometheusservice.RuleGroupsNamespaceDescription); ok {
		return output, err
	}

	return nil, err
}

func waitScraperCreated(ctx context.Context, conn *amp.Client, id string, timeout time.Duration) (*types.ScraperDescription, error) {
	stateConf := &retry.StateChangeConf{
		Pending: enum.Slice(types.ScraperStatusCodeCreating),
		Target:  enum.Slice(types.ScraperStatusCodeActive),
		Refresh: statusScraper(ctx, conn, id),
		Timeout: timeout,
	}

	outputRaw, err := stateConf.WaitForStateContext(ctx)

	if out, ok := outputRaw.(*types.ScraperDescription); ok {
		return out, err
	}

	return nil, err
}

func waitScraperDeleted(ctx context.Context, conn *amp.Client, id string, timeout time.Duration) (*types.ScraperDescription, error) {
	stateConf := &retry.StateChangeConf{
		Pending: enum.Slice(types.ScraperStatusCodeActive, types.ScraperStatusCodeDeleting),
		Target:  []string{},
		Refresh: statusScraper(ctx, conn, id),
		Timeout: timeout,
	}

	outputRaw, err := stateConf.WaitForStateContext(ctx)

	if output, ok := outputRaw.(*types.ScraperDescription); ok {
		return output, err
	}

	return nil, err
}

func waitWorkspaceCreated(ctx context.Context, conn *prometheusservice.PrometheusService, id string) (*prometheusservice.WorkspaceDescription, error) {
	stateConf := &retry.StateChangeConf{
		Pending: []string{prometheusservice.WorkspaceStatusCodeCreating},
		Target:  []string{prometheusservice.WorkspaceStatusCodeActive},
		Refresh: statusWorkspace(ctx, conn, id),
		Timeout: workspaceTimeout,
	}

	outputRaw, err := stateConf.WaitForStateContext(ctx)

	if output, ok := outputRaw.(*prometheusservice.WorkspaceDescription); ok {
		return output, err
	}

	return nil, err
}

func waitWorkspaceDeleted(ctx context.Context, conn *prometheusservice.PrometheusService, id string) (*prometheusservice.WorkspaceDescription, error) {
	stateConf := &retry.StateChangeConf{
		Pending: []string{prometheusservice.WorkspaceStatusCodeDeleting},
		Target:  []string{},
		Refresh: statusWorkspace(ctx, conn, id),
		Timeout: workspaceTimeout,
	}

	outputRaw, err := stateConf.WaitForStateContext(ctx)

	if output, ok := outputRaw.(*prometheusservice.WorkspaceDescription); ok {
		return output, err
	}

	return nil, err
}

func waitWorkspaceUpdated(ctx context.Context, conn *prometheusservice.PrometheusService, id string) (*prometheusservice.WorkspaceDescription, error) {
	stateConf := &retry.StateChangeConf{
		Pending: []string{prometheusservice.WorkspaceStatusCodeUpdating},
		Target:  []string{prometheusservice.WorkspaceStatusCodeActive},
		Refresh: statusWorkspace(ctx, conn, id),
		Timeout: workspaceTimeout,
	}

	outputRaw, err := stateConf.WaitForStateContext(ctx)

	if output, ok := outputRaw.(*prometheusservice.WorkspaceDescription); ok {
		return output, err
	}

	return nil, err
}

func waitLoggingConfigurationCreated(ctx context.Context, conn *prometheusservice.PrometheusService, workspaceID string) (*prometheusservice.LoggingConfigurationMetadata, error) { //nolint:unparam
	stateConf := &retry.StateChangeConf{
		Pending: []string{prometheusservice.LoggingConfigurationStatusCodeCreating},
		Target:  []string{prometheusservice.LoggingConfigurationStatusCodeActive},
		Refresh: statusLoggingConfiguration(ctx, conn, workspaceID),
		Timeout: workspaceTimeout,
	}

	outputRaw, err := stateConf.WaitForStateContext(ctx)

	if output, ok := outputRaw.(*prometheusservice.LoggingConfigurationMetadata); ok {
		if statusCode := aws.StringValue(output.Status.StatusCode); statusCode == prometheusservice.LoggingConfigurationStatusCodeCreationFailed {
			tfresource.SetLastError(err, errors.New(aws.StringValue(output.Status.StatusReason)))
		}

		return output, err
	}

	return nil, err
}

func waitLoggingConfigurationDeleted(ctx context.Context, conn *prometheusservice.PrometheusService, workspaceID string) (*prometheusservice.LoggingConfigurationMetadata, error) {
	stateConf := &retry.StateChangeConf{
		Pending: []string{prometheusservice.LoggingConfigurationStatusCodeDeleting},
		Target:  []string{},
		Refresh: statusLoggingConfiguration(ctx, conn, workspaceID),
		Timeout: workspaceTimeout,
	}

	outputRaw, err := stateConf.WaitForStateContext(ctx)

	if output, ok := outputRaw.(*prometheusservice.LoggingConfigurationMetadata); ok {
		return output, err
	}

	return nil, err
}

func waitLoggingConfigurationUpdated(ctx context.Context, conn *prometheusservice.PrometheusService, workspaceID string) (*prometheusservice.LoggingConfigurationMetadata, error) {
	stateConf := &retry.StateChangeConf{
		Pending: []string{prometheusservice.LoggingConfigurationStatusCodeUpdating},
		Target:  []string{prometheusservice.LoggingConfigurationStatusCodeActive},
		Refresh: statusLoggingConfiguration(ctx, conn, workspaceID),
		Timeout: workspaceTimeout,
	}

	outputRaw, err := stateConf.WaitForStateContext(ctx)

	if output, ok := outputRaw.(*prometheusservice.LoggingConfigurationMetadata); ok {
		if statusCode := aws.StringValue(output.Status.StatusCode); statusCode == prometheusservice.LoggingConfigurationStatusCodeUpdateFailed {
			tfresource.SetLastError(err, errors.New(aws.StringValue(output.Status.StatusReason)))
		}

		return output, err
	}

	return nil, err
}
