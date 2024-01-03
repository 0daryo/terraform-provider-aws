// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package amp

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/amp"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/prometheusservice"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/hashicorp/terraform-provider-aws/internal/tfresource"
)

func statusScraper(ctx context.Context, conn *amp.Client, id string) retry.StateRefreshFunc {
	return func() (interface{}, string, error) {
		output, err := FindScraperByID(ctx, conn, id)

		if tfresource.NotFound(err) {
			return nil, "", nil
		}

		if err != nil {
			return nil, "", err
		}

		return output, string(output.Status.StatusCode), nil
	}
}

func statusWorkspace(ctx context.Context, conn *prometheusservice.PrometheusService, id string) retry.StateRefreshFunc {
	return func() (interface{}, string, error) {
		output, err := FindWorkspaceByID(ctx, conn, id)

		if tfresource.NotFound(err) {
			return nil, "", nil
		}

		if err != nil {
			return nil, "", err
		}

		return output, aws.StringValue(output.Status.StatusCode), nil
	}
}

func statusLoggingConfiguration(ctx context.Context, conn *prometheusservice.PrometheusService, workspaceID string) retry.StateRefreshFunc {
	return func() (interface{}, string, error) {
		output, err := FindLoggingConfigurationByWorkspaceID(ctx, conn, workspaceID)

		if tfresource.NotFound(err) {
			return nil, "", nil
		}

		if err != nil {
			return nil, "", err
		}

		return output, aws.StringValue(output.Status.StatusCode), nil
	}
}
