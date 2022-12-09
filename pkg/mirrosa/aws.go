package mirrosa

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

type Ec2AwsApi interface {
	DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
	ec2.DescribeSecurityGroupsAPIClient
	ec2.DescribeSecurityGroupRulesAPIClient
	ec2.DescribeSubnetsAPIClient
	ec2.DescribeVpcsAPIClient
}
