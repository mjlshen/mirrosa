package mirrosa

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"go.uber.org/zap/zaptest"
)

type mockMirrosaVpcEndpointServiceAPIClient struct {
	describeVpcEndpointServicesResp    *ec2.DescribeVpcEndpointServicesOutput
	describeVpcEndpointConnectionsResp *ec2.DescribeVpcEndpointConnectionsOutput
}

func (m mockMirrosaVpcEndpointServiceAPIClient) DescribeVpcEndpointServices(ctx context.Context, params *ec2.DescribeVpcEndpointServicesInput, optFns ...func(options *ec2.Options)) (*ec2.DescribeVpcEndpointServicesOutput, error) {
	return m.describeVpcEndpointServicesResp, nil
}

func (m mockMirrosaVpcEndpointServiceAPIClient) DescribeVpcEndpointConnections(ctx context.Context, params *ec2.DescribeVpcEndpointConnectionsInput, optFns ...func(options *ec2.Options)) (*ec2.DescribeVpcEndpointConnectionsOutput, error) {
	return m.describeVpcEndpointConnectionsResp, nil
}

func TestVpcEndpointService_Validate(t *testing.T) {
	tests := []struct {
		name        string
		privatelink bool
		mock        *mockMirrosaVpcEndpointServiceAPIClient
		wantErr     bool
	}{
		{
			name:        "non-PrivateLink",
			privatelink: false,
			wantErr:     false,
		},
		{
			name:        "PrivateLink, no VPCE Service",
			privatelink: true,
			mock: &mockMirrosaVpcEndpointServiceAPIClient{
				describeVpcEndpointServicesResp: &ec2.DescribeVpcEndpointServicesOutput{
					ServiceDetails: []types.ServiceDetail{},
				},
			},
			wantErr: true,
		},
		{
			name:        "Healthy PrivateLink",
			privatelink: true,
			mock: &mockMirrosaVpcEndpointServiceAPIClient{
				describeVpcEndpointServicesResp: &ec2.DescribeVpcEndpointServicesOutput{
					ServiceDetails: []types.ServiceDetail{
						{
							ServiceId: aws.String("vpce-mock"),
						},
					},
				},
				describeVpcEndpointConnectionsResp: &ec2.DescribeVpcEndpointConnectionsOutput{
					VpcEndpointConnections: []types.VpcEndpointConnection{
						{},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			v := VpcEndpointService{
				log:         zaptest.NewLogger(t).Sugar(),
				PrivateLink: test.privatelink,
				Ec2Client:   test.mock,
			}
			if err := v.Validate(context.TODO()); (err != nil) != test.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}
