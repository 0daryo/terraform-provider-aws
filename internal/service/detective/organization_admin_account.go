package detective

import (
	"context"
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/detective"
	"github.com/hashicorp/aws-sdk-go-base/v2/awsv1shim/v2/tfawserr"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	"github.com/hashicorp/terraform-provider-aws/internal/verify"
)

// @SDKResource("aws_detective_organization_admin_account")
func ResourceOrganizationAdminAccount() *schema.Resource {
	return &schema.Resource{
		CreateWithoutTimeout: resourceOrganizationAdminAccountCreate,
		ReadWithoutTimeout:   resourceOrganizationAdminAccountRead,
		DeleteWithoutTimeout: resourceOrganizationAdminAccountDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"account_id": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: verify.ValidAccountID,
			},
		},
	}
}

func resourceOrganizationAdminAccountCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*conns.AWSClient).DetectiveConn()

	accountID := d.Get("account_id").(string)

	input := &detective.EnableOrganizationAdminAccountInput{
		AccountId: aws.String(accountID),
	}

	_, err := conn.EnableOrganizationAdminAccountWithContext(ctx, input)

	if err != nil {
		return diag.Errorf("error enabling Detective Organization Admin Account (%s): %s", accountID, err)
	}

	d.SetId(accountID)

	if _, err := waitAdminAccountFound(ctx, conn, d.Id()); err != nil {
		return diag.Errorf("error waiting for Detective Organization Admin Account (%s) to enable: %s", d.Id(), err)
	}

	return resourceOrganizationAdminAccountRead(ctx, d, meta)
}

func resourceOrganizationAdminAccountRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*conns.AWSClient).DetectiveConn()

	adminAccount, err := FindAdminAccount(ctx, conn, d.Id())

	if !d.IsNewResource() && tfawserr.ErrCodeEquals(err, detective.ErrCodeResourceNotFoundException) {
		log.Printf("[WARN] Detective Organization Admin Account (%s) not found, removing from state", d.Id())
		d.SetId("")
		return nil
	}

	if err != nil {
		return diag.Errorf("error reading Detective Organization Admin Account (%s): %s", d.Id(), err)
	}

	if adminAccount == nil {
		if d.IsNewResource() {
			return diag.Errorf("error reading Detective Organization Admin Account (%s): %s", d.Id(), err)
		}

		log.Printf("[WARN] Detective Organization Admin Account (%s) not found, removing from state", d.Id())
		d.SetId("")
		return nil
	}

	d.Set("account_id", adminAccount.AccountId)

	return nil
}

func resourceOrganizationAdminAccountDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*conns.AWSClient).DetectiveConn()

	input := &detective.DisableOrganizationAdminAccountInput{}

	_, err := conn.DisableOrganizationAdminAccountWithContext(ctx, input)

	if tfawserr.ErrCodeEquals(err, detective.ErrCodeResourceNotFoundException) {
		return nil
	}

	if err != nil {
		return diag.Errorf("error disabling Detective Organization Admin Account (%s): %s", d.Id(), err)
	}

	if _, err := waitAdminAccountNotFound(ctx, conn, d.Id()); err != nil {
		return diag.Errorf("error waiting for Detective Organization Admin Account (%s) to disable: %s", d.Id(), err)
	}

	return nil
}
