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

const instanceDescription = `A ROSA cluster must have the following:
- 3 control plane instances running
- at least 2 infra instances running for single-AZ, 3 infra instances for multi-AZ`

var _ Component = &Instances{}

type MirrosaInstancesAPIClient interface {
	ec2.DescribeInstancesAPIClient
}

type Instances struct {
	log       *zap.SugaredLogger
	InfraName string
	MultiAZ   bool

	Ec2Client MirrosaInstancesAPIClient
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
	i.log.Info("validating cluster's control plane instances")
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
		return fmt.Errorf("there should be 3 control plane instances, found %d", len(masters))
	}

	// Check if masters are running
	for _, v := range masters {
		if v.State.Name != types.InstanceStateNameRunning {
			return fmt.Errorf("found non running control plane instance: %s", *v.InstanceId)
		}

		if len(v.SecurityGroups) != 1 {
			return fmt.Errorf("one security group should be attached to %s: (%s-master-sg), got %d", *v.InstanceId, i.InfraName, len(v.SecurityGroups))
		}

		// TODO: Check if the security group is the correct one, with tag "Name: ${infra_name}-master-sg"
	}

	// INFRA NODES VALIDATIONS
	i.log.Info("validating cluster's infra instances")
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
		return fmt.Errorf("there should be at least 3 infra instances for multi-AZ clusters")
	}

	if !i.MultiAZ && len(infraNodes) < 2 {
		return fmt.Errorf("there should be at least 2 infra instances for single-AZ clusters")
	}

	// Check if infras are running
	for _, v := range infraNodes {
		if v.State.Name != types.InstanceStateNameRunning {
			return fmt.Errorf("found non running infra instances: %s", *v.InstanceId)
		}

		if len(v.SecurityGroups) != 1 {
			return fmt.Errorf("one security group should be attached to %s: (%s-worker-sg), got %d", *v.InstanceId, i.InfraName, len(v.SecurityGroups))
		}

		// TODO: Check if the security group is the correct one, with tag "Name: ${infra_name}-worker-sg"
	}

	// WORKER NODES VALIDATIONS
	i.log.Info("validating cluster's worker instances")
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

		if len(v.SecurityGroups) != 1 {
			return fmt.Errorf("one security group should be attached to %s: (%s-worker-sg), got %d", *v.InstanceId, i.InfraName, len(v.SecurityGroups))
		}

		// TODO: Check if the security group is the correct one, with tag "Name: ${infra_name}-worker-sg"
	}

	return nil
}

func (i Instances) Description() string {
	return instanceDescription
}

func (i Instances) FilterValue() string {
	return "EC2 Instance"
}

func (i Instances) Title() string {
	return "EC2 Instance"
}
