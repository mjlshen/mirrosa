package rosa

import (
	"context"
	"errors"
	"fmt"

	"github.com/mjlshen/mirrosa/pkg/mirrosa"
	"github.com/mjlshen/mirrosa/pkg/ocm"
	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

const vpcDescription = "A ROSA cluster's VPC can be built by the installer or an existing one can be used. " +
	"`enableDnsSupport` and `enableDnsHostnames` must be enabled on the VPC so that the cluster can use the " +
	"private Route 53 Hosted Zones attached to the VPC to resolve internal DNS records."

type VpcAWSApi interface {
	DescribeVpcs(ctx context.Context, params *ec2.DescribeVpcsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error)
	DescribeSubnets(ctx context.Context, params *ec2.DescribeSubnetsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error)
	DescribeVpcAttribute(ctx context.Context, params *ec2.DescribeVpcAttributeInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcAttributeOutput, error)
}

// Ensure Vpc implements mirrosa.Component
var _ mirrosa.Component = &Vpc{}

type Vpc struct {
	InfraName string
	Byovpc    bool
	SubnetIds []string

	Ec2Client VpcAWSApi
}

func NewVpc(cluster *cmv1.Cluster, api VpcAWSApi) Vpc {
	return Vpc{
		// TODO: This doesn't allow the --infra-name override
		InfraName: cluster.InfraID(),
		Byovpc:    ocm.IsClusterByovpc(cluster),
		SubnetIds: cluster.AWS().SubnetIDs(),
		Ec2Client: api,
	}
}

func (v Vpc) Validate(ctx context.Context) (string, error) {
	vpcId, err := v.FindVpcId(ctx)
	if err != nil {
		return "", err
	}

	dnsHostnames, err := v.Ec2Client.DescribeVpcAttribute(ctx, &ec2.DescribeVpcAttributeInput{
		Attribute: types.VpcAttributeNameEnableDnsHostnames,
		VpcId:     aws.String(vpcId),
	})
	if err != nil {
		return "", err
	}

	dnsSupport, err := v.Ec2Client.DescribeVpcAttribute(ctx, &ec2.DescribeVpcAttributeInput{
		Attribute: types.VpcAttributeNameEnableDnsSupport,
		VpcId:     aws.String(vpcId),
	})
	if err != nil {
		return "", err
	}

	// Make sure dnsHostname's enableDnsHostnames attribute is true
	if !*dnsHostnames.EnableDnsHostnames.Value {
		return "", fmt.Errorf("enableDnsHostnames is false for VPC: %s", vpcId)
	}

	// Repeat for enableDnsSupport
	if !*dnsSupport.EnableDnsSupport.Value {
		return "", fmt.Errorf("enableDnsSupport is false for VPC: %s", vpcId)
	}

	return vpcId, nil
}

func (v Vpc) Documentation() string {
	return vpcDescription
}

func (v Vpc) FindVpcId(ctx context.Context) (string, error) {
	if v.Byovpc {
		// For BYOVPC clusters, subnet ids are provided, so determine the VPC from the provided subnets
		if len(v.SubnetIds) == 0 {
			// Shouldn't happen
			return "", errors.New("no subnet ids for BYOVPC cluster")
		}

		resp, err := v.Ec2Client.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{
			SubnetIds: v.SubnetIds,
		})
		if err != nil {
			return "", err
		}

		if len(resp.Subnets) == 0 {
			return "", fmt.Errorf("no subnetes found with ids: %v", v.SubnetIds)
		}

		return *resp.Subnets[0].VpcId, nil
	} else {
		// If this is not a BYOVPC cluster, there are no subnets provided. Instead, go off the cluster name
		// to find the VPC by tag as a best guess
		if v.InfraName == "" {
			return "", errors.New("empty infraName supplied")
		}

		// TODO: Handle pagination
		vpcs, err := v.Ec2Client.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{
			Filters: []types.Filter{
				{
					Name:   aws.String("tag:Name"),
					Values: []string{v.InfraName},
				},
			},
		})
		if err != nil {
			return "", err
		}

		switch len(vpcs.Vpcs) {
		case 0:
			return "", fmt.Errorf("no VPCs found with expected Name tag: %s", v.InfraName)
		case 1:
			return *vpcs.Vpcs[0].VpcId, nil
		default:
			return "", fmt.Errorf("multiple VPCs found with the expected Name tag: %s", v.InfraName)
		}
	}
}
