package rosa

import (
	"context"
	"fmt"
	elb "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancing"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"

	"github.com/mjlshen/mirrosa/pkg/mirrosa"
)

const (
	appsLoadBalancerDescription   = "TODO *.apps LB"
	apiLoadBalancerDescription    = "TODO api LB"
	apiIntLoadBalancerDescription = "TODO api-int LB"
	loadBalancerDescription       = "TODO all the LBs"
	appsLoadBalancerPrefix        = "\\052.apps"
	apiLoadBalancerPrefix         = "api"
	apiIntLoadBalancerPrefix      = "api.int"
)

// Ensure LoadBalancer implements mirrosa.Component
var _ mirrosa.Component = &LoadBalancer{}

type LoadBalancer struct {
	DnsName string
	Prefix  string
	VpcId   string

	ElbClient   ElbAwsApi
	ElbV2Client ElbV2AwsApi
}

func (l LoadBalancer) Validate(ctx context.Context) (string, error) {
	lbs, err := l.ElbV2Client.DescribeLoadBalancers(ctx, &elbv2.DescribeLoadBalancersInput{})
	if err != nil {
		return "", nil
	}

	for _, lb := range lbs.LoadBalancers {
		if *lb.DNSName == l.DnsName && *lb.VpcId == l.VpcId {
			return *lb.LoadBalancerArn, nil
		}
	}

	classicLbs, err := l.ElbClient.DescribeLoadBalancers(ctx, &elb.DescribeLoadBalancersInput{})
	if err != nil {
		return "", nil
	}

	for _, lb := range classicLbs.LoadBalancerDescriptions {
		if *lb.DNSName == l.DnsName && *lb.VPCId == l.VpcId {
			if len(lb.SecurityGroups) != 1 {
				return "", fmt.Errorf("%d security groups attached to %s, expected 1", len(lb.SecurityGroups), *lb.DNSName)
			}
			return lb.SecurityGroups[0], nil
		}
	}

	return "", fmt.Errorf("no lb found with DNS name: %s in VPC: %s", l.DnsName, l.VpcId)
}

func (l LoadBalancer) Documentation() string {
	switch l.Prefix {
	case apiLoadBalancerPrefix:
		return apiLoadBalancerDescription
	case apiIntLoadBalancerPrefix:
		return apiIntLoadBalancerDescription
	case appsLoadBalancerPrefix:
		return appsLoadBalancerDescription
	default:
		return loadBalancerDescription
	}
}

func (l LoadBalancer) FilterValue() string {
	name := l.Prefix
	if l.Prefix == appsLoadBalancerPrefix {
		name = "*.apps"
	}

	// TODO: Detect CLB/NLB/ALB
	return fmt.Sprintf("<type> Load Balancer - %s", name)
}

func NewApiLoadBalancer(elbApi ElbAwsApi, elbV2Api ElbV2AwsApi, vpcId, domainName string) LoadBalancer {
	return LoadBalancer{
		DnsName:     domainName,
		Prefix:      apiLoadBalancerPrefix,
		ElbClient:   elbApi,
		ElbV2Client: elbV2Api,
		VpcId:       vpcId,
	}
}

func NewApiIntLoadBalancer(elbApi ElbAwsApi, elbV2Api ElbV2AwsApi, vpcId, domainName string) LoadBalancer {
	return LoadBalancer{
		DnsName:     domainName,
		Prefix:      apiIntLoadBalancerPrefix,
		ElbClient:   elbApi,
		ElbV2Client: elbV2Api,
		VpcId:       vpcId,
	}
}

func NewAppsLoadBalancer(elbApi ElbAwsApi, elbV2Api ElbV2AwsApi, vpcId, domainName string) LoadBalancer {
	return LoadBalancer{
		DnsName:     domainName,
		Prefix:      appsLoadBalancerPrefix,
		ElbClient:   elbApi,
		ElbV2Client: elbV2Api,
		VpcId:       vpcId,
	}
}
