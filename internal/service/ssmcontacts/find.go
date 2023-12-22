// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package ssmcontacts

import (
	"context"
	"errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssmcontacts"
	"github.com/aws/aws-sdk-go-v2/service/ssmcontacts/types"
	awstypes "github.com/aws/aws-sdk-go-v2/service/ssmcontacts/types"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/hashicorp/terraform-provider-aws/internal/errs"
	"github.com/hashicorp/terraform-provider-aws/internal/tfresource"
)

func findContactByID(ctx context.Context, conn *ssmcontacts.Client, id string) (*ssmcontacts.GetContactOutput, error) {
	in := &ssmcontacts.GetContactInput{
		ContactId: aws.String(id),
	}
	out, err := conn.GetContact(ctx, in)
	if err != nil {
		var nfe *types.ResourceNotFoundException
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

func findContactChannelByID(ctx context.Context, conn *ssmcontacts.Client, id string) (*ssmcontacts.GetContactChannelOutput, error) {
	in := &ssmcontacts.GetContactChannelInput{
		ContactChannelId: aws.String(id),
	}
	out, err := conn.GetContactChannel(ctx, in)
	if err != nil {
		var nfe *types.ResourceNotFoundException
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

func FindRotationByID(ctx context.Context, conn *ssmcontacts.Client, id string) (*ssmcontacts.GetRotationOutput, error) {
	in := &ssmcontacts.GetRotationInput{
		RotationId: aws.String(id),
	}
	out, err := conn.GetRotation(ctx, in)

	if errs.IsA[*awstypes.ResourceNotFoundException](err) {
		return nil, &retry.NotFoundError{
			LastError:   err,
			LastRequest: in,
		}
	}

	if err != nil {
		return nil, err
	}

	if out == nil {
		return nil, tfresource.NewEmptyResultError(in)
	}

	return out, nil
}
