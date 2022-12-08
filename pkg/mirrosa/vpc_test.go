package mirrosa

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"go.uber.org/zap/zaptest"
)

type mockMirrosaVpcAPI func(ctx context.Context, params *ec2.DescribeVpcAttributeInput, optFns ...func(options *ec2.Options)) (*ec2.DescribeVpcAttributeOutput, error)

func (m mockMirrosaVpcAPI) DescribeVpcAttribute(ctx context.Context, params *ec2.DescribeVpcAttributeInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcAttributeOutput, error) {
	return m(ctx, params, optFns...)
}

func TestVpc_Validate(t *testing.T) {
	tests := []struct {
		name    string
		client  func(t *testing.T) MirrosaVpcAPIClient
		wantErr bool
	}{
		{
			name: "enableDnsSupport and enableDnsHostnames true",
			client: func(t *testing.T) MirrosaVpcAPIClient {
				return mockMirrosaVpcAPI(func(ctx context.Context, params *ec2.DescribeVpcAttributeInput, optFns ...func(options *ec2.Options)) (*ec2.DescribeVpcAttributeOutput, error) {
					t.Helper()
					if params.Attribute == types.VpcAttributeNameEnableDnsHostnames {
						return &ec2.DescribeVpcAttributeOutput{
							EnableDnsHostnames: &types.AttributeBooleanValue{Value: aws.Bool(true)},
							VpcId:              params.VpcId,
						}, nil
					}

					if params.Attribute == types.VpcAttributeNameEnableDnsSupport {
						return &ec2.DescribeVpcAttributeOutput{
							EnableDnsSupport: &types.AttributeBooleanValue{Value: aws.Bool(true)},
							VpcId:            params.VpcId,
						}, nil
					}

					return nil, errors.New("unsupported attribute")
				})
			},
			wantErr: false,
		},
		{
			name: "enableDnsSupport false",
			client: func(t *testing.T) MirrosaVpcAPIClient {
				return mockMirrosaVpcAPI(func(ctx context.Context, params *ec2.DescribeVpcAttributeInput, optFns ...func(options *ec2.Options)) (*ec2.DescribeVpcAttributeOutput, error) {
					t.Helper()
					if params.Attribute == types.VpcAttributeNameEnableDnsHostnames {
						return &ec2.DescribeVpcAttributeOutput{
							EnableDnsHostnames: &types.AttributeBooleanValue{Value: aws.Bool(true)},
							VpcId:              params.VpcId,
						}, nil
					}

					if params.Attribute == types.VpcAttributeNameEnableDnsSupport {
						return &ec2.DescribeVpcAttributeOutput{
							EnableDnsSupport: &types.AttributeBooleanValue{Value: aws.Bool(false)},
							VpcId:            params.VpcId,
						}, nil
					}

					return nil, errors.New("unsupported attribute")
				})
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			v := Vpc{
				log:       zaptest.NewLogger(t).Sugar(),
				Id:        "id",
				Ec2Client: test.client(t),
			}
			if err := v.Validate(context.TODO()); (err != nil) != test.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}
