package rosa

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/route53"
)

type RosaAWSClient interface {
	// EC2 Functions
	DescribeSubnets(ctx context.Context, params *ec2.DescribeSubnetsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error)
	DescribeVpcAttribute(ctx context.Context, params *ec2.DescribeVpcAttributeInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcAttributeOutput, error)
	// Route53 Functions
	GetHostedZone(ctx context.Context, params *route53.GetHostedZoneInput, optFns ...func(*route53.Options)) (*route53.GetHostedZoneOutput, error)
	ListHostedZonesByName(ctx context.Context, params *route53.ListHostedZonesByNameInput, optFns ...func(*route53.Options)) (*route53.ListHostedZonesByNameOutput, error)
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

	//test := route53.NewFromConfig(cfg)
	//test.ListHostedZonesByVPC()

	return &RosaClient{
		Ec2Client:     ec2.NewFromConfig(cfg),
		Route53Client: route53.NewFromConfig(cfg),
	}, nil
}
