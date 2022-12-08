package mirrosa

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"go.uber.org/zap"
)

const instanceDescription = `A ROSA cluster must have the followings
- 3 masters running
- at least 2 infras running for single-AZ, 3 infras for multi-AZ`

var _ Component = &Instances{}

type Instances struct {
	log       *zap.SugaredLogger
	InfraName string
	MultiAZ   bool

	Ec2Client Ec2AwsApi
}

func (c *Client) NewInstances() Instances {
	return Instances{
		log:       c.log,
		InfraName: c.ClusterInfo.InfraName,
		MultiAZ:   c.Cluster.MultiAZ(),
		Ec2Client: ec2.NewFromConfig(c.AwsConfig),
	}
}

func (i Instances) Validate(ctx context.Context) error {
	i.log.Info("running ec2 instance validations")
	in := &ec2.DescribeInstancesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String(fmt.Sprintf("tag:kubernetes.io/cluster/%s", i.InfraName)),
				Values: []string{"owned"},
			},
		},
	}

	var instances []types.Instance
	for {
		out, err := i.Ec2Client.DescribeInstances(ctx, in)
		if err != nil {
			return err
		}
		for _, res := range out.Reservations {
			instances = append(instances, res.Instances...)
		}
		if out.NextToken == nil {
			break
		}
		in.NextToken = out.NextToken
	}

	// MASTER NODES VALIDATIONS
	i.log.Info("validating cluster's master nodes")
	var masters []types.Instance
	masterPattern := fmt.Sprintf("%s-master", i.InfraName)
	for _, v := range instances {
		for _, tag := range v.Tags {
			if strings.Contains(*tag.Value, masterPattern) {
				masters = append(masters, v)
			}
		}
	}

	// Each cluster has 3 master nodes by default - immutable
	if len(masters) != 3 {
		return fmt.Errorf("there should be 3 masters belong to the cluster")
	}

	// Check if masters are running
	for _, v := range masters {
		if v.State.Name != types.InstanceStateNameRunning {
			return fmt.Errorf("found non running master instance: %s", *v.InstanceId)
		}
	}

	// INFRA NODES VALIDATIONS
	i.log.Info("validating cluster's infra nodes")
	var infraNodes []types.Instance
	infraPattern := fmt.Sprintf("%s-infra", i.InfraName)
	for _, v := range instances {
		for _, tag := range v.Tags {
			if strings.Contains(*tag.Value, infraPattern) {
				infraNodes = append(infraNodes, v)
			}
		}
	}

	if i.MultiAZ && len(infraNodes) < 3 {
		return fmt.Errorf("there should be at least 3 infra nodes for multi-AZ clusters")
	}

	if !i.MultiAZ && len(infraNodes) < 2 {
		return fmt.Errorf("there should be at least 2 infra nodes for single-AZ clusters")
	}

	// Check if infras are running
	for _, v := range infraNodes {
		if v.State.Name != types.InstanceStateNameRunning {
			return fmt.Errorf("found non running infra node: %s", *v.InstanceId)
		}
	}

	// WORKER NODES VALIDATIONS
	i.log.Info("validating cluster's worker nodes")
	var workerNodes []types.Instance
	workerPattern := fmt.Sprintf("%s-worker", i.InfraName)
	for _, v := range instances {
		for _, tag := range v.Tags {
			if strings.Contains(*tag.Value, workerPattern) {
				workerNodes = append(workerNodes, v)
			}
		}
	}

	// Check if there are any worker nodes provisioned
	if len(workerNodes) == 0 {
		return fmt.Errorf("there should be at least 1 worker node running, otherwise CU workloads wouldn't be able to be schedulable")
	}

	// Check if worker are running
	for _, v := range workerNodes {
		if v.State.Name != types.InstanceStateNameRunning {
			i.log.Infof("[error but not blocker]: found non running worker nodes: %s", *v.InstanceId)
		}
	}

	return nil
}

func (i Instances) Documentation() string {
	return instanceDescription
}

func (i Instances) FilterValue() string {
	return "instance validation service"
}
