package mirrosa

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"go.uber.org/zap/zaptest"
	"testing"
)

type mockMirrosaDhcpOptionsAPIClient struct {
	describeDhcpOptionsResp *ec2.DescribeDhcpOptionsOutput
	describeVpcsResp        *ec2.DescribeVpcsOutput
}

func (m mockMirrosaDhcpOptionsAPIClient) DescribeVpcs(ctx context.Context, params *ec2.DescribeVpcsInput, optFns ...func(options *ec2.Options)) (*ec2.DescribeVpcsOutput, error) {
	return m.describeVpcsResp, nil
}

func (m mockMirrosaDhcpOptionsAPIClient) DescribeDhcpOptions(ctx context.Context, params *ec2.DescribeDhcpOptionsInput, optFns ...func(options *ec2.Options)) (*ec2.DescribeDhcpOptionsOutput, error) {
	return m.describeDhcpOptionsResp, nil
}

func TestDhcpOptions_Validate(t *testing.T) {
	tests := []struct {
		name        string
		dhcpOptions *DhcpOptions
		expectErr   bool
	}{
		{
			name: "all lowercase",
			dhcpOptions: &DhcpOptions{
				log:   zaptest.NewLogger(t).Sugar(),
				VpcId: "id",
				Ec2Client: &mockMirrosaDhcpOptionsAPIClient{
					describeDhcpOptionsResp: &ec2.DescribeDhcpOptionsOutput{
						DhcpOptions: []types.DhcpOptions{
							{
								DhcpConfigurations: []types.DhcpConfiguration{
									{
										Key: aws.String("domain-name"),
										Values: []types.AttributeValue{
											{
												Value: aws.String("ec2.internal"),
											},
										},
									},
								},
								DhcpOptionsId: aws.String("dhcp-id"),
							},
						},
					},
					describeVpcsResp: &ec2.DescribeVpcsOutput{
						Vpcs: []types.Vpc{
							{
								DhcpOptionsId: aws.String("dhcp-id"),
								VpcId:         aws.String("id"),
							},
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name: "contains uppercase",
			dhcpOptions: &DhcpOptions{
				log:   zaptest.NewLogger(t).Sugar(),
				VpcId: "id",
				Ec2Client: &mockMirrosaDhcpOptionsAPIClient{
					describeDhcpOptionsResp: &ec2.DescribeDhcpOptionsOutput{
						DhcpOptions: []types.DhcpOptions{
							{
								DhcpConfigurations: []types.DhcpConfiguration{
									{
										Key: aws.String("domain-name"),
										Values: []types.AttributeValue{
											{
												Value: aws.String("My.cUsTom.DoMaIn"),
											},
										},
									},
								},
								DhcpOptionsId: aws.String("dhcp-id"),
							},
						},
					},
					describeVpcsResp: &ec2.DescribeVpcsOutput{
						Vpcs: []types.Vpc{
							{
								DhcpOptionsId: aws.String("dhcp-id"),
								VpcId:         aws.String("id"),
							},
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "contains space",
			dhcpOptions: &DhcpOptions{
				log:   zaptest.NewLogger(t).Sugar(),
				VpcId: "id",
				Ec2Client: &mockMirrosaDhcpOptionsAPIClient{
					describeDhcpOptionsResp: &ec2.DescribeDhcpOptionsOutput{
						DhcpOptions: []types.DhcpOptions{
							{
								DhcpConfigurations: []types.DhcpConfiguration{
									{
										Key: aws.String("domain-name"),
										Values: []types.AttributeValue{
											{
												Value: aws.String("www.example.com example.com"),
											},
										},
									},
								},
								DhcpOptionsId: aws.String("dhcp-id"),
							},
						},
					},
					describeVpcsResp: &ec2.DescribeVpcsOutput{
						Vpcs: []types.Vpc{
							{
								DhcpOptionsId: aws.String("dhcp-id"),
								VpcId:         aws.String("id"),
							},
						},
					},
				},
			},
			expectErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.dhcpOptions.Validate(context.TODO())
			if err != nil {
				if !test.expectErr {
					t.Errorf("expected no err, got %v", err)
				}
			} else {
				if test.expectErr {
					t.Error("expected err, got nil")
				}
			}
		})
	}
}
