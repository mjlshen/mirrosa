package mirrosa

import (
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	elb "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancing"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancing/types"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"go.uber.org/zap"
)

const (
	defaultIngressControllerLoadBalancerTagKey      = "kubernetes.io/service-name"
	defaultIngressControllerLoadBalancerTagValue    = "openshift-ingress/router-default"
	defaultIngressControllerLoadBalancerDescription = "TODO"
)

var _ Component = &DefaultIngressControllerLoadBalancer{}

type DefaultIngressControllerLoadBalancer struct {
	log       *zap.SugaredLogger
	InfraName string

	Ec2Client   Ec2AwsApi
	ElbClient   *elb.Client
	ElbV2Client NetworkLoadBalancerAPIClient
}

func (c *Client) NewIngressControllerLoadBalancer() DefaultIngressControllerLoadBalancer {
	return DefaultIngressControllerLoadBalancer{
		log:         c.log,
		InfraName:   c.ClusterInfo.InfraName,
		Ec2Client:   ec2.NewFromConfig(c.AwsConfig),
		ElbClient:   elb.NewFromConfig(c.AwsConfig),
		ElbV2Client: elbv2.NewFromConfig(c.AwsConfig),
	}
}

func (d DefaultIngressControllerLoadBalancer) Validate(ctx context.Context) error {
	d.log.Info("searching for default ingress controller load balancer")

	d.log.Info("searching classic load balancers")
	clb, err := d.searchForCLB(ctx)
	if err != nil {
		d.log.Info("failed to find classic load balancer for default ingress controller")
		// TODO: Search NLBs
	}

	if len(clb.SecurityGroups) != 1 {
		return fmt.Errorf("expected 1 security group attached to the default ingress controller load balancer, found %d", len(clb.SecurityGroups))
	}

	resp, err := d.Ec2Client.DescribeSecurityGroupRules(ctx, &ec2.DescribeSecurityGroupRulesInput{
		Filters: []ec2Types.Filter{
			{
				Name:   aws.String("group-id"),
				Values: []string{clb.SecurityGroups[0]},
			},
		},
	})
	if err != nil {
		return err
	}

	expectedRules := map[string]securityGroupRule{
		"http": {
			CidrIpv4:   "0.0.0.0/0",
			IpProtocol: ec2Types.ProtocolTcp,
			FromPort:   80,
			ToPort:     80,
			IsEgress:   false,
		},
		"https": {
			CidrIpv4:   "0.0.0.0/0",
			IpProtocol: ec2Types.ProtocolTcp,
			FromPort:   443,
			ToPort:     443,
			IsEgress:   false,
		},
		"icmp": {
			CidrIpv4:   "0.0.0.0/0",
			IpProtocol: "icmp",
			FromPort:   3,
			ToPort:     4,
			IsEgress:   false,
		},
		"egress": {
			CidrIpv4:   "0.0.0.0/0",
			IpProtocol: "-1",
			FromPort:   -1,
			ToPort:     -1,
			IsEgress:   true,
		},
	}

	for _, rule := range resp.SecurityGroupRules {
		// If we've found all the required security group rules, stop
		if len(expectedRules) == 0 {
			break
		}

		d.log.Debugf("found security group rule %s", *rule.SecurityGroupRuleId)
		for k, expectedRule := range expectedRules {
			if compareSecurityGroupRules(expectedRule, rule) {
				d.log.Infof("security group rule validated for %s: %+v", k, expectedRule)
				delete(expectedRules, k)
			}
		}
	}

	if len(expectedRules) > 0 {
		return fmt.Errorf("missing required rules in default ingress controller load balancer security group %v", expectedRules)
	}

	return nil
}

func (d DefaultIngressControllerLoadBalancer) Documentation() string {
	return defaultIngressControllerLoadBalancerDescription
}

func (d DefaultIngressControllerLoadBalancer) FilterValue() string {
	return "Default Ingress Controller Load Balancer"
}

func (d DefaultIngressControllerLoadBalancer) searchForCLB(ctx context.Context) (*types.LoadBalancerDescription, error) {
	resp, err := d.ElbClient.DescribeLoadBalancers(ctx, &elb.DescribeLoadBalancersInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to describe CLBs: %w", err)
	}

	for _, clb := range resp.LoadBalancerDescriptions {
		expectedTags := map[string]string{
			defaultIngressControllerLoadBalancerTagKey:           defaultIngressControllerLoadBalancerTagValue,
			fmt.Sprintf("kubernetes.io/cluster/%s", d.InfraName): "owned",
		}

		tagsResp, err := d.ElbClient.DescribeTags(ctx, &elb.DescribeTagsInput{
			LoadBalancerNames: []string{*clb.LoadBalancerName},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to describe tags of CLB %s:%w", *clb.LoadBalancerName, err)
		}

		for _, tag := range tagsResp.TagDescriptions[0].Tags {
			if len(expectedTags) == 0 {
				return &clb, nil
			}

			if v, ok := expectedTags[*tag.Key]; ok {
				if v == *tag.Value {
					d.log.Infof("found match for tag %s:%s", *tag.Key, *tag.Value)
					delete(expectedTags, *tag.Key)
				}
			}
		}
	}

	return nil, errors.New("no matching CLB found")
}
