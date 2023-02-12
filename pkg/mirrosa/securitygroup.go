package mirrosa

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"go.uber.org/zap"
)

const securityGroupDescription = "Security groups act as a virtual firewall for Elastic Network Interfaces (ENIs)." +
	" For ROSA clusters, the control plane and worker security groups restrict network traffic to the clusters nodes and" +
	" must not be modified. Here are some important required security group rules:" +
	"\n  - Control Plane: Inbound TCP port 6443 from the cluster's machine CIDR for the Kubernetes API server" +
	"\n  - Control Plane: Inbound TCP port 22623 from the cluster's machine CIDR for etcd"

//"\n  - Inbound master 10257 kube-controller-manager from worker" +
//"\n  - Inbound master 10259 kube-scheduler from worker" +
//"\n  - Inbound master 10250 kubelet from worker"

type securityGroupRule struct {
	CidrIpv4   string
	IpProtocol types.Protocol
	ToPort     int32
	FromPort   int32
	IsEgress   bool
}

var _ Component = &SecurityGroup{}

type SecurityGroup struct {
	log         *zap.SugaredLogger
	InfraName   string
	MachineCIDR string

	Ec2Client Ec2AwsApi
}

func (c *Client) NewSecurityGroup() SecurityGroup {
	return SecurityGroup{
		log:         c.log,
		InfraName:   c.ClusterInfo.InfraName,
		MachineCIDR: c.Cluster.Network().MachineCIDR(),
		Ec2Client:   ec2.NewFromConfig(c.AwsConfig),
	}
}

func (s SecurityGroup) Validate(ctx context.Context) error {
	var (
		masterGroup = fmt.Sprintf("%s-master-sg", s.InfraName)
		workerGroup = fmt.Sprintf("%s-worker-sg", s.InfraName)
	)

	expectedGroups := map[string]string{
		masterGroup: "",
		workerGroup: "",
	}

	for group := range expectedGroups {
		s.log.Infof("searching for security group: %s", group)
		resp, err := s.Ec2Client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
			Filters: []types.Filter{
				{
					Name:   aws.String("tag:Name"),
					Values: []string{group},
				},
				{
					Name:   aws.String(fmt.Sprintf("tag:kubernetes.io/cluster/%s", s.InfraName)),
					Values: []string{"owned"},
				},
			},
		})
		if err != nil {
			return err
		}

		switch len(resp.SecurityGroups) {
		case 0:
			return fmt.Errorf("security group: %s not found", group)
		case 1:
			s.log.Infof("found security group: %s", *resp.SecurityGroups[0].GroupId)
			expectedGroups[group] = *resp.SecurityGroups[0].GroupId
		default:
			return fmt.Errorf("multiple matches found for security group: %s", group)
		}
	}

	s.log.Info("validating security group rules inside master security group")
	resp, err := s.Ec2Client.DescribeSecurityGroupRules(ctx, &ec2.DescribeSecurityGroupRulesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("group-id"),
				Values: []string{expectedGroups[masterGroup]},
			},
		},
	})
	if err != nil {
		return err
	}

	// TODO: Validate more rules?
	expectedMasterRules := map[string]securityGroupRule{
		"etcd": {
			CidrIpv4:   s.MachineCIDR,
			IpProtocol: types.ProtocolTcp,
			FromPort:   22623,
			ToPort:     22623,
			IsEgress:   false,
		},
		"kube-apiserver": {
			CidrIpv4:   s.MachineCIDR,
			IpProtocol: types.ProtocolTcp,
			FromPort:   6443,
			ToPort:     6443,
			IsEgress:   false,
		},
	}

	for _, rule := range resp.SecurityGroupRules {
		// If we've found all the required security group rules, stop
		if len(expectedMasterRules) == 0 {
			break
		}

		s.log.Debugf("found security group rule %s", *rule.SecurityGroupRuleId)
		for k, expectedRule := range expectedMasterRules {
			if compareSecurityGroupRules(expectedRule, rule) {
				s.log.Infof("security group rule validated for %s: %+v", k, expectedRule)
				delete(expectedMasterRules, k)
			}
		}
	}

	if len(expectedMasterRules) > 0 {
		return fmt.Errorf("missing required rules in master security group %v", expectedMasterRules)
	}

	return nil
}

func (s SecurityGroup) Description() string {
	return securityGroupDescription
}

func (s SecurityGroup) FilterValue() string {
	return "Security Group"
}

func (s SecurityGroup) Title() string {
	return "Security Group"
}

func compareSecurityGroupRules(expected securityGroupRule, actual types.SecurityGroupRule) bool {
	if actual.CidrIpv4 != nil {
		if string(expected.IpProtocol) == *actual.IpProtocol &&
			expected.CidrIpv4 == *actual.CidrIpv4 &&
			expected.FromPort == *actual.FromPort &&
			expected.ToPort == *actual.ToPort &&
			expected.IsEgress == *actual.IsEgress {
			return true
		}
	}

	return false
}
