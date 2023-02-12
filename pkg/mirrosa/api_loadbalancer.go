package mirrosa

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"go.uber.org/zap"
)

const (
	// TODO: Handle non-STS, where external can be disabled and switched to internal-only
	stsApiLoadBalancerDescription = "An STS ROSA cluster uses two Network Load Balancer (NLB) to balance traffic to" +
		"its OpenShift and Kubernetes API Servers - neither can be disabled." +
		"\n  - An internal (-int) NLB to balance traffic internal to the cluster." +
		"\n  - An external (-ext) NLB to balance traffic external to the cluster."
	privateLinkApiLoadBalancerDescription = "A PrivateLink ROSA cluster uses one Network Load Balancer (NLB) to " +
		"balance traffic to its OpenShift and Kubernetes API Servers - it cannot be disabled." +
		"\n  - An internal (-int) NLB to balance traffic within the cluster's VPC."
)

// elb represents the expected state of an Elastic Load Balancer in AWS
type elb struct {
	name              string
	expectedListeners map[string]listener
}

// listener represents the expected state of an Elastic Load Balancer Listener in AWS
type listener struct {
	port           int32
	protocol       types.ProtocolEnum
	healthyTargets int
}

// Ensure NetworkLoadBalancer implements Component
var _ Component = &NetworkLoadBalancer{}

// NetworkLoadBalancerAPIClient is a client that implements what's needed to validate a NetworkLoadBalancer
type NetworkLoadBalancerAPIClient interface {
	elbv2.DescribeLoadBalancersAPIClient
	elbv2.DescribeListenersAPIClient
	elbv2.DescribeTargetGroupsAPIClient
	elbv2.DescribeTargetHealthAPIClient
}

type NetworkLoadBalancer struct {
	log         *zap.SugaredLogger
	InfraName   string
	PrivateLink bool
	Sts         bool
	VpcId       string

	ElbV2Client NetworkLoadBalancerAPIClient
}

func (c *Client) NewApiLoadBalancer() NetworkLoadBalancer {
	return NetworkLoadBalancer{
		log:         c.log,
		InfraName:   c.ClusterInfo.InfraName,
		PrivateLink: c.Cluster.AWS().PrivateLink(),
		Sts:         c.Cluster.AWS().STS() != nil,
		VpcId:       c.ClusterInfo.VpcId,
		ElbV2Client: elbv2.NewFromConfig(c.AwsConfig),
	}
}

func (n NetworkLoadBalancer) Validate(ctx context.Context) error {
	for name, nlb := range n.getExpectedNLBs() {
		n.log.Infof("searching for network load balancer: %s", nlb.name)
		resp, err := n.ElbV2Client.DescribeLoadBalancers(ctx, &elbv2.DescribeLoadBalancersInput{
			Names: []string{nlb.name},
		})
		if err != nil {
			return err
		}

		var (
			nlbArn  string
			matches []string
		)

		for _, lb := range resp.LoadBalancers {
			if *lb.VpcId == n.VpcId &&
				lb.Type == types.LoadBalancerTypeEnumNetwork {
				matches = append(matches, *lb.LoadBalancerArn)
			}
		}

		switch len(matches) {
		case 0:
			return fmt.Errorf("NLB %s not found in VPC: %s", nlb.name, n.VpcId)
		case 1:
			n.log.Infof("found NLB: %s", matches[0])
			nlbArn = matches[0]
		default:
			return fmt.Errorf("multiple matches found for NLB: %s in VPC %s", nlb.name, n.VpcId)
		}

		listenResp, err := n.ElbV2Client.DescribeListeners(ctx, &elbv2.DescribeListenersInput{
			LoadBalancerArn: aws.String(nlbArn),
		})
		if err != nil {
			return err
		}

		for _, l := range listenResp.Listeners {
			if len(nlb.expectedListeners) == 0 {
				break
			}

			n.log.Debugf("found listener: %s", *l.ListenerArn)
			for k, expectedListener := range nlb.expectedListeners {
				if listenersEqual(expectedListener, l) {
					if err := n.validateTargetGroups(ctx, *l.DefaultActions[0].TargetGroupArn, expectedListener.healthyTargets); err != nil {
						return err
					}

					n.log.Infof("listener validated for %s: %+v", k, expectedListener)
					delete(nlb.expectedListeners, k)
				}
			}
		}

		if len(nlb.expectedListeners) > 0 {
			return fmt.Errorf("missing required listeners in NLB %s: %v", name, nlb.expectedListeners)
		}
	}

	return nil
}

func (n NetworkLoadBalancer) Description() string {
	if n.PrivateLink {
		return privateLinkApiLoadBalancerDescription
	}
	// TODO: Handle non-STS
	return stsApiLoadBalancerDescription
}

func (n NetworkLoadBalancer) FilterValue() string {
	return "Network Load Balancers"
}

func (n NetworkLoadBalancer) Title() string {
	return "Network Load Balancers"
}

// getExpectedNLBs returns a map of expected elb instances given a NetworkLoadBalancer Component
func (n NetworkLoadBalancer) getExpectedNLBs() map[string]elb {
	expected := map[string]elb{}

	expected["api-int"] = elb{
		name: fmt.Sprintf("%s-int", n.InfraName),
		expectedListeners: map[string]listener{
			"etcd": {
				port:           22623,
				protocol:       types.ProtocolEnumTcp,
				healthyTargets: 3,
			},
			"kube-apiserver": {
				port:           6443,
				protocol:       types.ProtocolEnumTcp,
				healthyTargets: 3,
			},
		},
	}

	// TODO: Handle non-STS, where the external NLB is optional if it is a private cluster
	if !n.PrivateLink && n.Sts {
		expected["api-ext"] = elb{
			name: fmt.Sprintf("%s-ext", n.InfraName),
			expectedListeners: map[string]listener{
				"kube-apiserver": {
					port:           6443,
					protocol:       types.ProtocolEnumTcp,
					healthyTargets: 3,
				},
			},
		}
	}

	return expected
}

// listenersEqual returns true if a listener and types.Listener can be considered equivalent for our purposes
func listenersEqual(expected listener, actual types.Listener) bool {
	return expected.port == *actual.Port && expected.protocol == actual.Protocol
}

// validateTargetGroups searches for a target group by arn and checks if it has the expected number of healthy targets
func (n NetworkLoadBalancer) validateTargetGroups(ctx context.Context, arn string, expected int) error {
	n.log.Infof("validating target group: %s", arn)
	n.log.Debugf("searching for target group: %s", arn)
	resp, err := n.ElbV2Client.DescribeTargetGroups(ctx, &elbv2.DescribeTargetGroupsInput{
		TargetGroupArns: []string{arn},
	})
	if err != nil {
		return fmt.Errorf("failed to find target group %s: %w", arn, err)
	}

	switch len(resp.TargetGroups) {
	case 0:
		return fmt.Errorf("target group %s not found", arn)
	case 1:
		n.log.Debugf("found target group: %s", *resp.TargetGroups[0].TargetGroupArn)
	default:
		return fmt.Errorf("multiple matches found for target group: %s", arn)
	}

	n.log.Debugf("validating target group: %s has %d healthy targets", arn, expected)
	healthResp, err := n.ElbV2Client.DescribeTargetHealth(ctx, &elbv2.DescribeTargetHealthInput{
		TargetGroupArn: aws.String(arn),
	})
	if err != nil {
		return fmt.Errorf("failed to assess health of target group %s: %w", arn, err)
	}

	healthyTargets := 0
	for _, health := range healthResp.TargetHealthDescriptions {
		if health.TargetHealth.State == types.TargetHealthStateEnumHealthy {
			healthyTargets++
		}
	}

	if healthyTargets != expected {
		return fmt.Errorf("expected %d healthy targets for %s, only found %d", expected, arn, healthyTargets)
	}

	n.log.Infof("validated target group: %s", arn)
	return nil
}
