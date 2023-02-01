package mirrosa

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"go.uber.org/zap/zaptest"
)

type mockMirrosaInstancesAPIClient struct {
	describeInstancesResp *ec2.DescribeInstancesOutput
}

func (m mockMirrosaInstancesAPIClient) DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(options *ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	return m.describeInstancesResp, nil
}

func TestInstances_Validate(t *testing.T) {
	tests := []struct {
		name      string
		instances *Instances
		expectErr bool
	}{
		{
			name: "no instances",
			instances: &Instances{
				log: zaptest.NewLogger(t).Sugar(),
				Ec2Client: &mockMirrosaInstancesAPIClient{
					describeInstancesResp: &ec2.DescribeInstancesOutput{
						Reservations: []types.Reservation{},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "healthy multi-az",
			instances: &Instances{
				log:       zaptest.NewLogger(t).Sugar(),
				InfraName: "mock",
				MultiAZ:   true,
				Ec2Client: &mockMirrosaInstancesAPIClient{
					describeInstancesResp: &ec2.DescribeInstancesOutput{
						Reservations: []types.Reservation{
							{
								Groups: nil,
								Instances: []types.Instance{
									{
										SecurityGroups: []types.GroupIdentifier{
											{},
										},
										State: &types.InstanceState{Name: types.InstanceStateNameRunning},
										Tags: []types.Tag{
											{Key: aws.String("Name"), Value: aws.String("mock-master1")},
										},
									},
									{
										SecurityGroups: []types.GroupIdentifier{
											{},
										},
										State: &types.InstanceState{Name: types.InstanceStateNameRunning},
										Tags: []types.Tag{
											{Key: aws.String("Name"), Value: aws.String("mock-master2")},
										},
									},
									{
										SecurityGroups: []types.GroupIdentifier{
											{},
										},
										State: &types.InstanceState{Name: types.InstanceStateNameRunning},
										Tags: []types.Tag{
											{Key: aws.String("Name"), Value: aws.String("mock-master3")},
										},
									},
									{
										SecurityGroups: []types.GroupIdentifier{
											{},
										},
										State: &types.InstanceState{Name: types.InstanceStateNameRunning},
										Tags: []types.Tag{
											{Key: aws.String("Name"), Value: aws.String("mock-infra1")},
										},
									},
									{
										SecurityGroups: []types.GroupIdentifier{
											{},
										},
										State: &types.InstanceState{Name: types.InstanceStateNameRunning},
										Tags: []types.Tag{
											{Key: aws.String("Name"), Value: aws.String("mock-infra2")},
										},
									},
									{
										SecurityGroups: []types.GroupIdentifier{
											{},
										},
										State: &types.InstanceState{Name: types.InstanceStateNameRunning},
										Tags: []types.Tag{
											{Key: aws.String("Name"), Value: aws.String("mock-infra3")},
										},
									},
									{
										SecurityGroups: []types.GroupIdentifier{
											{},
										},
										State: &types.InstanceState{Name: types.InstanceStateNameRunning},
										Tags: []types.Tag{
											{Key: aws.String("Name"), Value: aws.String("mock-worker1")},
										},
									},
									{
										SecurityGroups: []types.GroupIdentifier{
											{},
										},
										State: &types.InstanceState{Name: types.InstanceStateNameRunning},
										Tags: []types.Tag{
											{Key: aws.String("Name"), Value: aws.String("mock-worker2")},
										},
									},
								},
							},
						},
					},
				},
			},
			expectErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.instances.Validate(context.TODO())
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
