package rosa

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/route53"
)

type RosaAWSClient interface {
	DescribeSubnets(ctx context.Context, params *ec2.DescribeSubnetsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error)
	DescribeVpcAttribute(ctx context.Context, params *ec2.DescribeVpcAttributeInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcAttributeOutput, error)
}

type RosaClient struct {
	Ec2Client     *ec2.Client
	Route53Client *route53.Client
}

func NewClient(ctx context.Context, optFns ...func(*config.LoadOptions) error) (*RosaClient, error) {
	cfg, err := config.LoadDefaultConfig(ctx, optFns...)
	if err != nil {
		return nil, err
	}

	//test := ec2.NewFromConfig(cfg)

	return &RosaClient{
		Ec2Client:     ec2.NewFromConfig(cfg),
		Route53Client: route53.NewFromConfig(cfg),
	}, nil
}

// GetVpcIdFromSubnetId returns the VPC ID associated with a provided subnetId
func (c *RosaClient) GetVpcIdFromSubnetId(ctx context.Context, subnetId string) (*string, error) {
	subnet, err := c.Ec2Client.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{
		SubnetIds: []string{subnetId},
	})
	if err != nil {
		return nil, err
	}

	if len(subnet.Subnets) == 0 {
		return nil, fmt.Errorf("no subnet found with id: %s", subnetId)
	}

	return subnet.Subnets[0].VpcId, nil
}

// ValidateVpcAttributes will inspect a provided vpcId and ensure that
// "enableDnsHostnames" and "enableDnsSupport" are true
func (c *RosaClient) ValidateVpcAttributes(ctx context.Context, vpcId string) error {
	dnsHostnames, err := c.Ec2Client.DescribeVpcAttribute(ctx, &ec2.DescribeVpcAttributeInput{
		Attribute: ec2Types.VpcAttributeNameEnableDnsHostnames,
		VpcId:     aws.String(vpcId),
	})
	if err != nil {
		return err
	}

	dnsSupport, err := c.Ec2Client.DescribeVpcAttribute(ctx, &ec2.DescribeVpcAttributeInput{
		Attribute: ec2Types.VpcAttributeNameEnableDnsSupport,
		VpcId:     aws.String(vpcId),
	})
	if err != nil {
		return err
	}

	// Make sure dnsHostname's enableDnsHostnames attribute is true
	if !*dnsHostnames.EnableDnsHostnames.Value {
		return fmt.Errorf("enableDnsHostnames is false for VPC: %s", vpcId)
	}

	// Repeat for enableDnsSupport
	if !*dnsSupport.EnableDnsSupport.Value {
		return fmt.Errorf("enableDnsSupport is false for VPC: %s", vpcId)
	}

	return nil
}

// ValidatePublicRoute53HostedZone
// We can get baseDomain from `ocm describe cluster $CLUSTER_ID --json`
func (c *RosaClient) ValidatePublicRoute53HostedZoneExists(ctx context.Context, baseDomain string) error {
	// Do stuff

	return nil
}

// ValidateAnotherThing
