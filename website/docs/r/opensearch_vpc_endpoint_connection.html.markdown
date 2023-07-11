---
subcategory: "OpenSearch"
layout: "aws"
page_title: "AWS: aws_opensearch_vpc_endpoint_connection"
description: |-
  Terraform resource for managing an AWS OpenSearch VPC Endpoint connection.
---

# Resource: aws_opensearch_vpc_endpoint_connection

Manages an [AWS Opensearch VPC Endpoint Connection](https://docs.aws.amazon.com/opensearch-service/latest/APIReference/API_CreateVpcEndpoint.html). Creates an Amazon OpenSearch Service-managed VPC endpoint..

## Example Usage

### Basic Usage



## Argument Reference

The following arguments are supported:

* `domain_arn` - (Required, Forces new resource) Specifies the Amazon Resource Name (ARN) of the domain to create the endpoint for
* `vpc_options` - (Optional) Options to specify the subnets and security groups for the endpoint.

### vpc_options

* `security_group_ids` - (Optional) The list of security group IDs associated with the VPC endpoints for the domain. If you do not provide a security group ID, OpenSearch Service uses the default security group for the VPC.
* `subnet_ids` - (Optional) A list of subnet IDs associated with the VPC endpoints for the domain. If your domain uses multiple Availability Zones, you need to provide two subnet IDs, one per zone. Otherwise, provide only one.

## Attributes Reference

In addition to all arguments above, the following attributes are exported:

* `id` - The connection endpoint ID for connecting to the domain.
* `connection_status` - The current status of the endpoint.

## Import

AWS Opensearch VPC Endpoint Connection can be imported by using the VPC Endpoint Connection ID, e.g.,

```
$ terraform import aws_opensearch_vpc_endpoint_connection.foo endpoint-id
```
