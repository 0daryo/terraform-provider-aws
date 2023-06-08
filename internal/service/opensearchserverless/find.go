package opensearchserverless

import (
	"context"
	"errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/opensearchserverless"
	"github.com/aws/aws-sdk-go-v2/service/opensearchserverless/types"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/hashicorp/terraform-provider-aws/internal/tfresource"
)

func FindSecurityPolicyByNameAndType(ctx context.Context, conn *opensearchserverless.Client, name, policyType string) (*types.SecurityPolicyDetail, error) {
	in := &opensearchserverless.GetSecurityPolicyInput{
		Name: aws.String(name),
		Type: types.SecurityPolicyType(policyType),
	}
	out, err := conn.GetSecurityPolicy(ctx, in)
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

	if out == nil || out.SecurityPolicyDetail == nil {
		return nil, tfresource.NewEmptyResultError(in)
	}

	return out.SecurityPolicyDetail, nil
}
