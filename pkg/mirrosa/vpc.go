package mirrosa

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"go.uber.org/zap"
)

const vpcDescription = "A VPC is a logically isolated network in AWS and a ROSA cluster can be installed " +
	"into an existing VPC (BYOVPC) or the installer can create one for the end-user (non-BYOVPC). " +
	"Regardless, the 'enableDnsSupport' and 'enableDnsHostnames' settings must be enabled on the VPC so that the cluster can use the " +
	"private Route 53 Hosted Zones attached to the VPC to resolve internal DNS records [1]." +
	"\n\nnon-BYOVPC's must not be modified and must contain the resources exactly documented in [2], while BYOVPC's allow " +
	"for more flexibility and the only requirement is that the required network egresses are resolvable and routable [3], " +
	"which can be validated by osd-network-verifier [4]." +
	"\n\nReferences:\n" +
	"1. https://docs.aws.amazon.com/vpc/latest/userguide/vpc-dns.html#vpc-dns-support\n" +
	"2. https://docs.openshift.com/rosa/rosa_planning/rosa-sts-aws-prereqs.html#rosa-vpc_rosa-sts-aws-prereqs\n" +
	"3. https://docs.openshift.com/rosa/rosa_planning/rosa-sts-aws-prereqs.html#osd-aws-privatelink-firewall-prerequisites_rosa-sts-aws-prereqs\n" +
	"4. https://github.com/openshift/osd-network-verifier"

// Ensure Vpc implements Component
var _ Component = &Vpc{}

// MirrosaVpcAPIClient is a client that implements what's needed to validate a Vpc
type MirrosaVpcAPIClient interface {
	DescribeVpcAttribute(ctx context.Context, params *ec2.DescribeVpcAttributeInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcAttributeOutput, error)
}

type Vpc struct {
	log *zap.SugaredLogger
	Id  string

	Ec2Client MirrosaVpcAPIClient
}

func (c *Client) NewVpc() Vpc {
	return Vpc{
		log:       c.log,
		Id:        c.ClusterInfo.VpcId,
		Ec2Client: ec2.NewFromConfig(c.AwsConfig),
	}
}

func (v Vpc) Validate(ctx context.Context) error {
	v.log.Infof("validating vpc: %s", v.Id)

	v.log.Debugf("validating that enableDnsHostnames is true for vpc: %s", v.Id)
	dnsHostnames, err := v.Ec2Client.DescribeVpcAttribute(ctx, &ec2.DescribeVpcAttributeInput{
		Attribute: types.VpcAttributeNameEnableDnsHostnames,
		VpcId:     aws.String(v.Id),
	})
	if err != nil {
		return err
	}

	if !*dnsHostnames.EnableDnsHostnames.Value {
		return fmt.Errorf("enableDnsHostnames is false for VPC: %s", v.Id)
	}

	v.log.Debugf("validating that enableDnsSupport is true for vpc: %s", v.Id)
	dnsSupport, err := v.Ec2Client.DescribeVpcAttribute(ctx, &ec2.DescribeVpcAttributeInput{
		Attribute: types.VpcAttributeNameEnableDnsSupport,
		VpcId:     aws.String(v.Id),
	})
	if err != nil {
		return err
	}

	if !*dnsSupport.EnableDnsSupport.Value {
		return fmt.Errorf("enableDnsSupport is false for VPC: %s", v.Id)
	}

	return nil
}

func (v Vpc) Description() string {
	return vpcDescription
}

func (v Vpc) FilterValue() string {
	return v.Title()
}

func (v Vpc) Title() string {
	return "VPC"
}
