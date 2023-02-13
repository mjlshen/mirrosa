package mirrosa

import (
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"go.uber.org/zap"
)

const vpceServiceDescription = "A VPC Endpoint Service allows for a load balancer to be exposed through PrivateLink, AWS' internal network, to other AWS accounts via VPC Endpoints [1]. " +
	" A PrivateLink ROSA cluster must have a VPC Endpoint Service with a single VPC Endpoint connection that allows Hive " +
	" to connect to the cluster over PrivateLink [2] to allow for management via SyncSets and backplane." +
	"\n\nReferences:\n" +
	"1. https://docs.aws.amazon.com/vpc/latest/privatelink/privatelink-share-your-services.html\n" +
	"2. https://github.com/openshift/hive/tree/master/pkg/controller/awsprivatelink"

var _ Component = &VpcEndpointService{}

// MirrosaVpcEndpointServiceAPIClient is a client that implements what's needed to validate a VpcEndpointService
type MirrosaVpcEndpointServiceAPIClient interface {
	DescribeVpcEndpointServices(ctx context.Context, params *ec2.DescribeVpcEndpointServicesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcEndpointServicesOutput, error)
	ec2.DescribeVpcEndpointConnectionsAPIClient
}

type VpcEndpointService struct {
	log         *zap.SugaredLogger
	InfraName   string
	PrivateLink bool

	Ec2Client MirrosaVpcEndpointServiceAPIClient
}

func (c *Client) NewVpcEndpointService() VpcEndpointService {
	return VpcEndpointService{
		log:         c.log,
		InfraName:   c.ClusterInfo.InfraName,
		PrivateLink: c.Cluster.AWS().PrivateLink(),
		Ec2Client:   ec2.NewFromConfig(c.AwsConfig),
	}
}

func (v VpcEndpointService) Validate(ctx context.Context) error {
	// non-PrivateLink clusters do not have a VPC Endpoint Service
	if !v.PrivateLink {
		return nil
	}

	v.log.Infof("searching for PrivateLink VPC Endpoint Service: %s-vpc-endpoint-service", v.InfraName)
	var serviceId string
	resp, err := v.Ec2Client.DescribeVpcEndpointServices(ctx, &ec2.DescribeVpcEndpointServicesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("tag:Name"),
				Values: []string{fmt.Sprintf("%s-vpc-endpoint-service", v.InfraName)},
			},
			{
				Name:   aws.String("tag:hive.openshift.io/private-link-access-for"),
				Values: []string{v.InfraName},
			},
		},
	})
	if err != nil {
		return err
	}

	switch len(resp.ServiceDetails) {
	case 0:
		return errors.New("no VPC Endpoint Services found for PrivateLink cluster")
	case 1:
		v.log.Infof("found VPC Endpoint Service: %s", *resp.ServiceDetails[0].ServiceId)
		serviceId = *resp.ServiceDetails[0].ServiceId
	default:
		return errors.New("multiple VPC Endpoint Services found for PrivateLink cluster")
	}

	v.log.Infof("validating VPC Endpoint Service: %s", *resp.ServiceDetails[0].ServiceId)
	cxResp, err := v.Ec2Client.DescribeVpcEndpointConnections(ctx, &ec2.DescribeVpcEndpointConnectionsInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("service-id"),
				Values: []string{serviceId},
			},
			{
				Name:   aws.String("vpc-endpoint-state"),
				Values: []string{"available"},
			},
		},
	})
	if err != nil {
		return err
	}

	switch len(cxResp.VpcEndpointConnections) {
	case 0:
		return fmt.Errorf("no available VPC Endpoint connections found for %s", serviceId)
	case 1:
		v.log.Infof("validated that one accepted VPC Endpoint connection for %s exists", serviceId)
		return nil
	default:
		return fmt.Errorf("multiple available VPC Endpoint connections found for %s", serviceId)
	}
}

func (v VpcEndpointService) Description() string {
	return vpceServiceDescription
}

func (v VpcEndpointService) FilterValue() string {
	return "VPC Endpoint Service"
}

func (v VpcEndpointService) Title() string {
	return "VPC Endpoint Service"
}
