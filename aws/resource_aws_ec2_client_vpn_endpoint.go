package aws

import (
	"bytes"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/hashicorp/terraform/helper/hashcode"
	"github.com/hashicorp/terraform/helper/resource"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/helper/validation"
)

func resourceAwsEc2ClientVpnEndpoint() *schema.Resource {
	return &schema.Resource{
		Create: resourceAwsEc2ClientVpnEndpointCreate,
		Read:   resourceAwsEc2ClientVpnEndpointRead,
		Delete: resourceAwsEc2ClientVpnEndpointDelete,
		Update: resourceAwsEc2ClientVpnEndpointUpdate,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"description": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"client_cidr_block": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"dns_servers": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			"server_certificate_arn": {
				Type:     schema.TypeString,
				Required: true,
			},
			"split_tunnel": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			"transport_protocol": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
				Default:  ec2.TransportProtocolUdp,
				ValidateFunc: validation.StringInSlice([]string{
					ec2.TransportProtocolTcp,
					ec2.TransportProtocolUdp,
				}, false),
			},
			"authentication_options": {
				Type:     schema.TypeList,
				Required: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"type": {
							Type:     schema.TypeString,
							Required: true,
							ForceNew: true,
							ValidateFunc: validation.StringInSlice([]string{
								ec2.ClientVpnAuthenticationTypeCertificateAuthentication,
								ec2.ClientVpnAuthenticationTypeDirectoryServiceAuthentication,
							}, false),
						},
						"active_directory_id": {
							Type:     schema.TypeString,
							Optional: true,
							ForceNew: true,
						},
						"root_certificate_chain_arn": {
							Type:     schema.TypeString,
							Optional: true,
							ForceNew: true,
						},
					},
				},
			},
			"connection_log_options": {
				Type:     schema.TypeList,
				Required: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"cloudwatch_log_group": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"cloudwatch_log_stream": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"enabled": {
							Type:     schema.TypeBool,
							Required: true,
						},
					},
				},
			},
			"network_association": {
				Type:     schema.TypeSet,
				Optional: true,
				Set:      resourceAwsNetAssocHash,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"subnet_id": {
							Type:     schema.TypeString,
							Required: true,
						},
						"security_groups": {
							Type:     schema.TypeSet,
							Optional: true,
							Computed: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
						"association_id": {
							Type:     schema.TypeString,
							Computed: true,
						},
					},
				},
			},
			"authorization_rule": {
				Type:     schema.TypeSet,
				Optional: true,
				Set:      resourceAwsAuthRuleHash,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"description": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"target_network_cidr": {
							Type:     schema.TypeString,
							Required: true,
						},
						"access_group_id": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"authorize_all_groups": {
							Type:     schema.TypeBool,
							Optional: true,
							Default:  true,
						},
					},
				},
			},
			"route": {
				Type:     schema.TypeSet,
				Optional: true,
				Set:      resourceAwsRouteHash,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"description": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"destination_network_cidr": {
							Type:     schema.TypeString,
							Required: true,
						},
						"subnet_id": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
			"dns_name": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"tags": tagsSchema(),
		},
	}
}

func resourceAwsEc2ClientVpnEndpointCreate(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).ec2conn

	req := &ec2.CreateClientVpnEndpointInput{
		ClientCidrBlock:      aws.String(d.Get("client_cidr_block").(string)),
		ServerCertificateArn: aws.String(d.Get("server_certificate_arn").(string)),
		TransportProtocol:    aws.String(d.Get("transport_protocol").(string)),
		SplitTunnel:          aws.Bool(d.Get("split_tunnel").(bool)),
		TagSpecifications:    ec2TagSpecificationsFromMap(d.Get("tags").(map[string]interface{}), ec2.ResourceTypeClientVpnEndpoint),
	}

	if v, ok := d.GetOk("description"); ok {
		req.Description = aws.String(v.(string))
	}

	if v, ok := d.GetOk("dns_servers"); ok {
		req.DnsServers = expandStringList(v.(*schema.Set).List())
	}

	if v, ok := d.GetOk("authentication_options"); ok {
		authOptsSet := v.([]interface{})
		attrs := authOptsSet[0].(map[string]interface{})

		authOptsReq := &ec2.ClientVpnAuthenticationRequest{
			Type: aws.String(attrs["type"].(string)),
		}

		if attrs["type"].(string) == "certificate-authentication" {
			authOptsReq.MutualAuthentication = &ec2.CertificateAuthenticationRequest{
				ClientRootCertificateChainArn: aws.String(attrs["root_certificate_chain_arn"].(string)),
			}
		}

		if attrs["type"].(string) == "directory-service-authentication" {
			authOptsReq.ActiveDirectory = &ec2.DirectoryServiceAuthenticationRequest{
				DirectoryId: aws.String(attrs["active_directory_id"].(string)),
			}
		}

		req.AuthenticationOptions = []*ec2.ClientVpnAuthenticationRequest{authOptsReq}
	}

	if v, ok := d.GetOk("connection_log_options"); ok {
		connLogSet := v.([]interface{})
		attrs := connLogSet[0].(map[string]interface{})

		connLogReq := &ec2.ConnectionLogOptions{
			Enabled: aws.Bool(attrs["enabled"].(bool)),
		}

		if attrs["enabled"].(bool) && attrs["cloudwatch_log_group"].(string) != "" {
			connLogReq.CloudwatchLogGroup = aws.String(attrs["cloudwatch_log_group"].(string))
		}

		if attrs["enabled"].(bool) && attrs["cloudwatch_log_stream"].(string) != "" {
			connLogReq.CloudwatchLogStream = aws.String(attrs["cloudwatch_log_stream"].(string))
		}

		req.ConnectionLogOptions = connLogReq
	}

	resp, err := conn.CreateClientVpnEndpoint(req)

	if err != nil {
		return fmt.Errorf("Error creating Client VPN endpoint: %s", err)
	}

	d.SetId(*resp.ClientVpnEndpointId)

	if _, ok := d.GetOk("network_association"); ok {
		networks, securityGroups := addNetworkAssociation(d.Id(), d.Get("network_association").(*schema.Set).List())

		for i, n := range networks {
			netResp, err := conn.AssociateClientVpnTargetNetwork(n)
			if err != nil {
				return fmt.Errorf("Failure adding new Client VPN authorization rules: %s", err)
			}

			stateConf := &resource.StateChangeConf{
				Pending: []string{ec2.AssociationStatusCodeAssociating},
				Target:  []string{ec2.AssociationStatusCodeAssociated},
				Refresh: clientVpnNetworkAssociationRefresh(conn, aws.StringValue(netResp.AssociationId), d.Id()),
				Timeout: d.Timeout(schema.TimeoutCreate),
			}

			log.Printf("[DEBUG] Waiting for Client VPN endpoint to associate with target network: %s", aws.StringValue(netResp.AssociationId))
			targetNetworkRaw, err := stateConf.WaitForState()
			if err != nil {
				return fmt.Errorf("Error waiting for Client VPN endpoint to associate with target network: %s", err)
			}

			targetNetwork := targetNetworkRaw.(*ec2.TargetNetwork)

			if len(securityGroups[i]) > 0 {
				sgReq := &ec2.ApplySecurityGroupsToClientVpnTargetNetworkInput{
					ClientVpnEndpointId: aws.String(d.Id()),
					VpcId:               targetNetwork.VpcId,
					SecurityGroupIds:    securityGroups[i],
				}

				_, err := conn.ApplySecurityGroupsToClientVpnTargetNetwork(sgReq)
				if err != nil {
					return fmt.Errorf("Error applying security groups to Client VPN network association: %s", err)
				}
			}
		}
	}

	if _, ok := d.GetOk("authorization_rule"); ok {
		rules := addAuthorizationRules(d.Id(), d.Get("authorization_rule").(*schema.Set).List())

		for _, r := range rules {
			_, err := conn.AuthorizeClientVpnIngress(r)
			if err != nil {
				return fmt.Errorf("Failure adding new Client VPN authorization rules: %s", err)
			}
		}
	}

	if _, ok := d.GetOk("route"); ok {
		rules := addRoutes(d.Id(), d.Get("route").(*schema.Set).List())

		for _, r := range rules {
			_, err := conn.CreateClientVpnRoute(r)
			if err != nil {
				return fmt.Errorf("Failure adding new Client VPN routes: %s", err)
			}
		}
	}

	return resourceAwsEc2ClientVpnEndpointRead(d, meta)
}

func resourceAwsEc2ClientVpnEndpointRead(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).ec2conn
	var err error

	result, err := conn.DescribeClientVpnEndpoints(&ec2.DescribeClientVpnEndpointsInput{
		ClientVpnEndpointIds: []*string{aws.String(d.Id())},
	})

	if err != nil {
		return fmt.Errorf("Error reading Client VPN endpoint: %s", err)
	}

	if result == nil || len(result.ClientVpnEndpoints) == 0 || result.ClientVpnEndpoints[0] == nil {
		log.Printf("[WARN] EC2 Client VPN Endpoint (%s) not found, removing from state", d.Id())
		d.SetId("")
		return nil
	}

	if result.ClientVpnEndpoints[0].Status != nil && aws.StringValue(result.ClientVpnEndpoints[0].Status.Code) == ec2.ClientVpnEndpointStatusCodeDeleted {
		log.Printf("[WARN] EC2 Client VPN Endpoint (%s) not found, removing from state", d.Id())
		d.SetId("")
		return nil
	}

	d.Set("description", result.ClientVpnEndpoints[0].Description)
	d.Set("client_cidr_block", result.ClientVpnEndpoints[0].ClientCidrBlock)
	d.Set("server_certificate_arn", result.ClientVpnEndpoints[0].ServerCertificateArn)
	d.Set("transport_protocol", result.ClientVpnEndpoints[0].TransportProtocol)
	d.Set("dns_name", result.ClientVpnEndpoints[0].DnsName)
	d.Set("status", result.ClientVpnEndpoints[0].Status)
	d.Set("split_tunnel", result.ClientVpnEndpoints[0].SplitTunnel)

	err = d.Set("authentication_options", flattenAuthOptsConfig(result.ClientVpnEndpoints[0].AuthenticationOptions))
	if err != nil {
		return fmt.Errorf("error setting authentication_options: %s", err)
	}

	err = d.Set("connection_log_options", flattenConnLoggingConfig(result.ClientVpnEndpoints[0].ConnectionLogOptions))
	if err != nil {
		return fmt.Errorf("error setting connection_log_options: %s", err)
	}

	err = d.Set("tags", tagsToMap(result.ClientVpnEndpoints[0].Tags))
	if err != nil {
		return fmt.Errorf("error setting tags: %s", err)
	}

	netAssocResult, err := conn.DescribeClientVpnTargetNetworks(&ec2.DescribeClientVpnTargetNetworksInput{
		ClientVpnEndpointId: aws.String(d.Id()),
	})
	if err != nil {
		return err
	}

	d.Set("network_association", flattenNetAssoc(netAssocResult.ClientVpnTargetNetworks))

	authRuleResult, err := conn.DescribeClientVpnAuthorizationRules(&ec2.DescribeClientVpnAuthorizationRulesInput{
		ClientVpnEndpointId: aws.String(d.Id()),
	})
	if err != nil {
		return err
	}

	d.Set("authorization_rule", flattenAuthRules(authRuleResult.AuthorizationRules))

	routeResult, err := conn.DescribeClientVpnRoutes(&ec2.DescribeClientVpnRoutesInput{
		ClientVpnEndpointId: aws.String(d.Id()),
	})
	if err != nil {
		return err
	}

	d.Set("route", flattenRoutes(routeResult.Routes))

	return nil
}

func resourceAwsEc2ClientVpnEndpointDelete(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).ec2conn

	netAssocResult, err := conn.DescribeClientVpnTargetNetworks(&ec2.DescribeClientVpnTargetNetworksInput{
		ClientVpnEndpointId: aws.String(d.Id()),
	})
	if err != nil {
		return err
	}

	for _, n := range netAssocResult.ClientVpnTargetNetworks {
		network := &ec2.DisassociateClientVpnTargetNetworkInput{}

		network.ClientVpnEndpointId = aws.String(d.Id())
		network.AssociationId = aws.String(*n.AssociationId)

		log.Printf("[DEBUG] Client VPN network association opts: %s", n)
		_, err := conn.DisassociateClientVpnTargetNetwork(network)
		if err != nil {
			return fmt.Errorf("D Failure removing Client VPN network associations: %s \n %s", err, n)
		}

		stateConf := &resource.StateChangeConf{
			Pending: []string{ec2.AssociationStatusCodeDisassociating},
			Target:  []string{ec2.AssociationStatusCodeDisassociated},
			Refresh: clientVpnNetworkAssociationRefresh(conn, aws.StringValue(n.AssociationId), d.Id()),
			Timeout: d.Timeout(schema.TimeoutDelete),
		}

		log.Printf("[DEBUG] Waiting for Client VPN endpoint to disassociate with target network: %s", aws.StringValue(n.AssociationId))
		_, err = stateConf.WaitForState()
		if err != nil {
			return fmt.Errorf("Error waiting for Client VPN endpoint to disassociate with target network: %s", err)
		}
	}

	_, err = conn.DeleteClientVpnEndpoint(&ec2.DeleteClientVpnEndpointInput{
		ClientVpnEndpointId: aws.String(d.Id()),
	})
	if err != nil {
		return fmt.Errorf("Error deleting Client VPN endpoint: %s", err)
	}

	return nil
}

func resourceAwsEc2ClientVpnEndpointUpdate(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).ec2conn

	d.Partial(true)

	req := &ec2.ModifyClientVpnEndpointInput{
		ClientVpnEndpointId: aws.String(d.Id()),
	}

	if d.HasChange("description") {
		req.Description = aws.String(d.Get("description").(string))
	}

	if d.HasChange("dns_servers") {
		dnsValue := expandStringList(d.Get("dns_servers").(*schema.Set).List())
		var enabledValue *bool

		if len(dnsValue) > 0 {
			enabledValue = aws.Bool(true)
		} else {
			enabledValue = aws.Bool(false)
		}

		dnsMod := &ec2.DnsServersOptionsModifyStructure{
			CustomDnsServers: dnsValue,
			Enabled:          enabledValue,
		}
		req.DnsServers = dnsMod
	}

	if d.HasChange("server_certificate_arn") {
		req.ServerCertificateArn = aws.String(d.Get("server_certificate_arn").(string))
	}

	if d.HasChange("split_tunnel") {
		req.SplitTunnel = aws.Bool(d.Get("split_tunnel").(bool))
	}

	if d.HasChange("connection_log_options") {
		if v, ok := d.GetOk("connection_log_options"); ok {
			connSet := v.([]interface{})
			attrs := connSet[0].(map[string]interface{})

			connReq := &ec2.ConnectionLogOptions{
				Enabled: aws.Bool(attrs["enabled"].(bool)),
			}

			if attrs["enabled"].(bool) && attrs["cloudwatch_log_group"].(string) != "" {
				connReq.CloudwatchLogGroup = aws.String(attrs["cloudwatch_log_group"].(string))
			}

			if attrs["enabled"].(bool) && attrs["cloudwatch_log_stream"].(string) != "" {
				connReq.CloudwatchLogStream = aws.String(attrs["cloudwatch_log_stream"].(string))
			}

			req.ConnectionLogOptions = connReq
		}
	}

	if _, err := conn.ModifyClientVpnEndpoint(req); err != nil {
		return fmt.Errorf("Error modifying Client VPN endpoint: %s", err)
	}

	if d.HasChange("network_association") {
		o, n := d.GetChange("network_association")
		os := o.(*schema.Set)
		ns := n.(*schema.Set)

		removeNets := removeNetworkAssociation(d.Id(), os.Difference(ns).List())
		addNets, addSGs := addNetworkAssociation(d.Id(), ns.Difference(os).List())

		if len(removeNets) > 0 {
			for _, r := range removeNets {
				log.Printf("[DEBUG] Client VPN network authorization opts: %s", r)
				_, err := conn.DisassociateClientVpnTargetNetwork(r)
				if err != nil {
					return fmt.Errorf("Failure removing outdated Client VPN network authorizations: %s", err)
				}

				stateConf := &resource.StateChangeConf{
					Pending: []string{ec2.AssociationStatusCodeDisassociating},
					Target:  []string{ec2.AssociationStatusCodeDisassociated},
					Refresh: clientVpnNetworkAssociationRefresh(conn, aws.StringValue(r.AssociationId), d.Id()),
					Timeout: d.Timeout(schema.TimeoutDelete),
				}

				log.Printf("[DEBUG] Waiting for Client VPN endpoint to disassociate with target network: %s", aws.StringValue(r.AssociationId))
				_, err = stateConf.WaitForState()
				if err != nil {
					return fmt.Errorf("Error waiting for Client VPN endpoint to disassociate with target network: %s", err)
				}
			}
		}

		if len(addNets) > 0 {
			for i, a := range addNets {
				log.Printf("[DEBUG] Client VPN network authorization opts: %s", a)
				addResp, err := conn.AssociateClientVpnTargetNetwork(a)
				if err != nil {
					return fmt.Errorf("Failure adding new Client VPN network authorizations: %s", err)
				}

				stateConf := &resource.StateChangeConf{
					Pending: []string{ec2.AssociationStatusCodeAssociating},
					Target:  []string{ec2.AssociationStatusCodeAssociated},
					Refresh: clientVpnNetworkAssociationRefresh(conn, aws.StringValue(addResp.AssociationId), d.Id()),
					Timeout: d.Timeout(schema.TimeoutCreate),
				}

				log.Printf("[DEBUG] Waiting for Client VPN endpoint to associate with target network: %s", aws.StringValue(addResp.AssociationId))
				targetNetworkRaw, err := stateConf.WaitForState()
				if err != nil {
					return fmt.Errorf("Error waiting for Client VPN endpoint to associate with target network: %s", err)
				}

				targetNetwork := targetNetworkRaw.(*ec2.TargetNetwork)

				if len(addSGs[i]) > 0 {
					sgReq := &ec2.ApplySecurityGroupsToClientVpnTargetNetworkInput{
						ClientVpnEndpointId: aws.String(d.Id()),
						VpcId:               targetNetwork.VpcId,
						SecurityGroupIds:    addSGs[i],
					}

					_, err := conn.ApplySecurityGroupsToClientVpnTargetNetwork(sgReq)
					if err != nil {
						return fmt.Errorf("Error applying security groups to Client VPN network association: %s", err)
					}
				}
			}
		}
	}

	if d.HasChange("authorization_rule") {
		o, n := d.GetChange("authorization_rule")
		os := o.(*schema.Set)
		ns := n.(*schema.Set)

		remove := removeAuthorizationRules(d.Id(), os.Difference(ns).List())
		add := addAuthorizationRules(d.Id(), ns.Difference(os).List())

		if len(remove) > 0 {
			for _, r := range remove {
				log.Printf("[DEBUG] Client VPN authorization rule opts: %s", r)
				_, err := conn.RevokeClientVpnIngress(r)
				if err != nil {
					return fmt.Errorf("Failure removing outdated Client VPN authorization rules: %s", err)
				}
			}
		}

		if len(add) > 0 {
			for _, r := range add {
				log.Printf("[DEBUG] Client VPN authorization rule opts: %s", r)
				_, err := conn.AuthorizeClientVpnIngress(r)
				if err != nil {
					return fmt.Errorf("Failure adding new Client VPN authorization rules: %s", err)
				}
			}
		}
	}

	if d.HasChange("route") {
		o, n := d.GetChange("route")
		os := o.(*schema.Set)
		ns := n.(*schema.Set)

		remove := removeRoutes(d.Id(), os.Difference(ns).List())
		add := addRoutes(d.Id(), ns.Difference(os).List())

		if len(remove) > 0 {
			for _, r := range remove {
				log.Printf("[DEBUG] Client VPN authorization route opts: %s", r)
				_, err := conn.DeleteClientVpnRoute(r)
				if err != nil {
					return fmt.Errorf("Failure removing outdated Client VPN routes: %s", err)
				}
			}
		}

		if len(add) > 0 {
			for _, r := range add {
				log.Printf("[DEBUG] Client VPN authorization route opts: %s", r)
				_, err := conn.CreateClientVpnRoute(r)
				if err != nil {
					return fmt.Errorf("Failure adding new Client VPN authorization routes: %s", err)
				}
			}
		}
	}

	if err := setTags(conn, d); err != nil {
		return err
	}
	d.SetPartial("tags")

	d.Partial(false)
	return resourceAwsEc2ClientVpnEndpointRead(d, meta)
}

func clientVpnNetworkAssociationRefresh(conn *ec2.EC2, cvnaID string, cvepID string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		resp, err := conn.DescribeClientVpnTargetNetworks(&ec2.DescribeClientVpnTargetNetworksInput{
			ClientVpnEndpointId: aws.String(cvepID),
			AssociationIds:      []*string{aws.String(cvnaID)},
		})

		if isAWSErr(err, "InvalidClientVpnAssociationId.NotFound", "") || isAWSErr(err, "InvalidClientVpnEndpointId.NotFound", "") {
			return 42, ec2.AssociationStatusCodeDisassociated, nil
		}

		if err != nil {
			return nil, "", err
		}

		if resp == nil || len(resp.ClientVpnTargetNetworks) == 0 || resp.ClientVpnTargetNetworks[0] == nil {
			return 42, ec2.AssociationStatusCodeDisassociated, nil
		}

		return resp.ClientVpnTargetNetworks[0], aws.StringValue(resp.ClientVpnTargetNetworks[0].Status.Code), nil
	}
}

func flattenConnLoggingConfig(lopts *ec2.ConnectionLogResponseOptions) []map[string]interface{} {
	m := make(map[string]interface{})
	if lopts.CloudwatchLogGroup != nil {
		m["cloudwatch_log_group"] = *lopts.CloudwatchLogGroup
	}
	if lopts.CloudwatchLogStream != nil {
		m["cloudwatch_log_stream"] = *lopts.CloudwatchLogStream
	}
	m["enabled"] = *lopts.Enabled
	return []map[string]interface{}{m}
}

func flattenAuthOptsConfig(aopts []*ec2.ClientVpnAuthentication) []map[string]interface{} {
	m := make(map[string]interface{})
	if aopts[0].MutualAuthentication != nil {
		m["root_certificate_chain_arn"] = *aopts[0].MutualAuthentication.ClientRootCertificateChain
	}
	if aopts[0].ActiveDirectory != nil {
		m["active_directory_id"] = *aopts[0].ActiveDirectory.DirectoryId
	}
	m["type"] = *aopts[0].Type
	return []map[string]interface{}{m}
}

func flattenNetAssoc(list []*ec2.TargetNetwork) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(list))
	for _, i := range list {
		l := map[string]interface{}{
			"association_id": *i.AssociationId,
		}

		if i.SecurityGroups != nil {
			l["security_groups"] = i.SecurityGroups
		}

		result = append(result, l)
	}
	return result
}

func addNetworkAssociation(eid string, configured []interface{}) ([]*ec2.AssociateClientVpnTargetNetworkInput, [][]*string) {
	networks := make([]*ec2.AssociateClientVpnTargetNetworkInput, 0, len(configured))
	securityGroups := make([][]*string, 0, len(configured))

	for _, i := range configured {
		item := i.(map[string]interface{})
		network := &ec2.AssociateClientVpnTargetNetworkInput{}

		network.ClientVpnEndpointId = aws.String(eid)
		network.SubnetId = aws.String(item["subnet_id"].(string))

		networks = append(networks, network)
		securityGroups = append(securityGroups, expandStringSet(item["security_groups"].(*schema.Set)))
	}

	return networks, securityGroups
}

func removeNetworkAssociation(eid string, configured []interface{}) []*ec2.DisassociateClientVpnTargetNetworkInput {
	networks := make([]*ec2.DisassociateClientVpnTargetNetworkInput, 0, len(configured))

	for _, i := range configured {
		item := i.(map[string]interface{})
		network := &ec2.DisassociateClientVpnTargetNetworkInput{}

		network.ClientVpnEndpointId = aws.String(eid)
		network.AssociationId = aws.String(item["association_id"].(string))

		networks = append(networks, network)
	}

	return networks
}

func resourceAwsNetAssocHash(v interface{}) int {
	var buf bytes.Buffer
	m := v.(map[string]interface{})

	if v, ok := m["subnet_id"]; ok {
		buf.WriteString(fmt.Sprintf("%s-", v.(string)))
	}

	if v, ok := m["security_groups"]; ok {
		buf.WriteString(fmt.Sprintf("%v-", v.(*schema.Set).List()))
	}

	return hashcode.String(buf.String())
}

func flattenAuthRules(list []*ec2.AuthorizationRule) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(list))
	for _, i := range list {
		l := map[string]interface{}{
			"target_network_cidr": *i.DestinationCidr,
		}

		if i.Description != nil {
			l["description"] = aws.String(*i.Description)
		}
		if i.GroupId != nil {
			l["access_group_id"] = aws.String(*i.GroupId)
		}
		if i.AccessAll != nil {
			l["authorize_all_groups"] = aws.Bool(*i.AccessAll)
		}

		result = append(result, l)
	}
	return result
}

func addAuthorizationRules(eid string, configured []interface{}) []*ec2.AuthorizeClientVpnIngressInput {
	rules := make([]*ec2.AuthorizeClientVpnIngressInput, 0, len(configured))

	for _, i := range configured {
		item := i.(map[string]interface{})
		rule := &ec2.AuthorizeClientVpnIngressInput{}

		rule.ClientVpnEndpointId = aws.String(eid)
		rule.TargetNetworkCidr = aws.String(item["target_network_cidr"].(string))
		rule.AuthorizeAllGroups = aws.Bool(item["authorize_all_groups"].(bool))

		if item["description"].(string) != "" {
			rule.Description = aws.String(item["description"].(string))
		}

		if item["access_group_id"].(string) != "" {
			rule.AccessGroupId = aws.String(item["access_group_id"].(string))
		}

		rules = append(rules, rule)
	}

	return rules
}

func removeAuthorizationRules(eid string, configured []interface{}) []*ec2.RevokeClientVpnIngressInput {
	rules := make([]*ec2.RevokeClientVpnIngressInput, 0, len(configured))

	for _, i := range configured {
		item := i.(map[string]interface{})
		rule := &ec2.RevokeClientVpnIngressInput{}

		rule.ClientVpnEndpointId = aws.String(eid)
		rule.TargetNetworkCidr = aws.String(item["target_network_cidr"].(string))

		rules = append(rules, rule)
	}

	return rules
}

func resourceAwsAuthRuleHash(v interface{}) int {
	var buf bytes.Buffer
	m := v.(map[string]interface{})

	if v, ok := m["description"]; ok {
		buf.WriteString(fmt.Sprintf("%s-", v.(string)))
	}

	if v, ok := m["target_network_cidr"]; ok {
		buf.WriteString(fmt.Sprintf("%s-", v.(string)))
	}

	if v, ok := m["access_group_id"]; ok {
		buf.WriteString(fmt.Sprintf("%s-", v.(string)))
	}

	if v, ok := m["authorize_all_groups"]; ok {
		buf.WriteString(fmt.Sprintf("%v-", v.(bool)))
	}

	return hashcode.String(buf.String())
}

func flattenRoutes(list []*ec2.ClientVpnRoute) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(list))
	for _, i := range list {
		l := map[string]interface{}{
			"destination_network_cidr": *i.DestinationCidr,
			"subnet_id":                *i.TargetSubnet,
		}

		if i.Description != nil {
			l["description"] = aws.String(*i.Description)
		}

		result = append(result, l)
	}
	return result
}

func addRoutes(eid string, configured []interface{}) []*ec2.CreateClientVpnRouteInput {
	routes := make([]*ec2.CreateClientVpnRouteInput, 0, len(configured))

	for _, i := range configured {
		item := i.(map[string]interface{})
		route := &ec2.CreateClientVpnRouteInput{}

		route.ClientVpnEndpointId = aws.String(eid)
		route.DestinationCidrBlock = aws.String(item["destination_network_cidr"].(string))
		route.TargetVpcSubnetId = aws.String(item["subnet_id"].(string))

		if item["description"].(string) != "" {
			route.Description = aws.String(item["description"].(string))
		}

		routes = append(routes, route)
	}

	return routes
}

func removeRoutes(eid string, configured []interface{}) []*ec2.DeleteClientVpnRouteInput {
	routes := make([]*ec2.DeleteClientVpnRouteInput, 0, len(configured))

	for _, i := range configured {
		item := i.(map[string]interface{})
		route := &ec2.DeleteClientVpnRouteInput{}

		route.ClientVpnEndpointId = aws.String(eid)
		route.DestinationCidrBlock = aws.String(item["destination_network_cidr"].(string))
		route.TargetVpcSubnetId = aws.String(item["subnet_id"].(string))

		routes = append(routes, route)
	}

	return routes
}

func resourceAwsRouteHash(v interface{}) int {
	var buf bytes.Buffer
	m := v.(map[string]interface{})

	if v, ok := m["description"]; ok {
		buf.WriteString(fmt.Sprintf("%s-", v.(string)))
	}

	if v, ok := m["destination_network_cidr"]; ok {
		buf.WriteString(fmt.Sprintf("%s-", v.(string)))
	}

	if v, ok := m["subnet_id"]; ok {
		buf.WriteString(fmt.Sprintf("%s-", v.(string)))
	}

	return hashcode.String(buf.String())
}
