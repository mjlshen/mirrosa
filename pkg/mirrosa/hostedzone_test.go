package mirrosa

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/aws/smithy-go/middleware"
)

type mockMirrosaHostedZoneAPIClient struct {
	getHostedZoneResp          *route53.GetHostedZoneOutput
	listHostedZonesByNameResp  *route53.ListHostedZonesByNameOutput
	listResourceRecordSetsResp *route53.ListResourceRecordSetsOutput
}

func (m mockMirrosaHostedZoneAPIClient) GetHostedZone(ctx context.Context, params *route53.GetHostedZoneInput, optFns ...func(*route53.Options)) (*route53.GetHostedZoneOutput, error) {
	return m.getHostedZoneResp, nil
}

func (m mockMirrosaHostedZoneAPIClient) ListHostedZonesByName(ctx context.Context, params *route53.ListHostedZonesByNameInput, optFns ...func(*route53.Options)) (*route53.ListHostedZonesByNameOutput, error) {
	return m.listHostedZonesByNameResp, nil
}

func (m mockMirrosaHostedZoneAPIClient) ListResourceRecordSets(ctx context.Context, params *route53.ListResourceRecordSetsInput, optFns ...func(*route53.Options)) (*route53.ListResourceRecordSetsOutput, error) {
	return m.listResourceRecordSetsResp, nil
}

func TestPublicHostedZone_Validate(t *testing.T) {
	tests := []struct {
		name      string
		resp      *route53.ListHostedZonesByNameOutput
		expectErr bool
	}{
		{
			name: "public hosted zone",
			resp: &route53.ListHostedZonesByNameOutput{
				HostedZones: []types.HostedZone{
					{
						Id: aws.String("HZ_public"),
						Config: &types.HostedZoneConfig{
							PrivateZone: false,
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name: "private hosted zone",
			resp: &route53.ListHostedZonesByNameOutput{
				HostedZones: []types.HostedZone{
					{
						Id: aws.String("HZ_private"),
						Config: &types.HostedZoneConfig{
							PrivateZone: true,
						},
					},
				},
			},
			expectErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := PublicHostedZone{
				log:         slog.New(slog.NewTextHandler(os.Stdout, nil)),
				BaseDomain:  "",
				PrivateLink: false,
				Route53Client: &mockMirrosaHostedZoneAPIClient{
					listHostedZonesByNameResp: test.resp,
				},
			}

			err := client.Validate(context.TODO())
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

func TestPrivateHostedZone_Validate(t *testing.T) {
	const (
		mockCluster = "cluster"
		mockDomain  = "domain"
		mockVpcId   = "vpc"
		mockHzId    = "hz"
	)
	tests := []struct {
		name      string
		lhzResp   *route53.ListHostedZonesByNameOutput
		ghzResp   *route53.GetHostedZoneOutput
		lrrsResp  *route53.ListResourceRecordSetsOutput
		expectErr bool
	}{
		{
			name: "missing private hosted zone",
			lhzResp: &route53.ListHostedZonesByNameOutput{
				HostedZones: []types.HostedZone{
					{
						Id: aws.String("HZ_public"),
						Config: &types.HostedZoneConfig{
							PrivateZone: false,
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "private hosted zone with no records",
			ghzResp: &route53.GetHostedZoneOutput{
				HostedZone: &types.HostedZone{
					Id: aws.String(mockHzId),
				},
				VPCs: []types.VPC{
					{
						VPCId: aws.String(mockVpcId),
					},
				},
				ResultMetadata: middleware.Metadata{},
			},
			lhzResp: &route53.ListHostedZonesByNameOutput{
				HostedZones: []types.HostedZone{
					{
						Id:   aws.String(mockHzId),
						Name: aws.String(fmt.Sprintf("%s.%s.", mockCluster, mockDomain)),
						Config: &types.HostedZoneConfig{
							PrivateZone: true,
						},
					},
				},
			},
			lrrsResp: &route53.ListResourceRecordSetsOutput{
				ResourceRecordSets: []types.ResourceRecordSet{},
			},
			expectErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := PrivateHostedZone{
				log:         slog.New(slog.NewTextHandler(os.Stdout, nil)),
				ClusterName: mockCluster,
				BaseDomain:  mockDomain,
				Region:      "",
				VpcId:       mockVpcId,
				Route53Client: &mockMirrosaHostedZoneAPIClient{
					getHostedZoneResp:          test.ghzResp,
					listHostedZonesByNameResp:  test.lhzResp,
					listResourceRecordSetsResp: test.lrrsResp,
				},
			}

			err := client.Validate(context.TODO())
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
