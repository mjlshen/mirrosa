package mirrosa

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/mjlshen/mirrosa/pkg/ocm"
	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
)

// Client holds relevant information about a ROSA cluster gleaned from OCM and an AwsApi to validate the cluster in AWS
type Client struct {
	log *slog.Logger

	// Cluster holds a cluster object from OCM
	Cluster *cmv1.Cluster

	// AwsConfig holds the configuration for building an AWS client
	AwsConfig aws.Config

	// ClusterInfo contains information about the ROSA cluster that will be used to validate it
	ClusterInfo *ClusterInfo
}

// ClusterInfo contains information about the ROSA cluster that will be used to validate it
type ClusterInfo struct {
	// Name of the cluster
	Name string

	// InfraName is the name with an additional slug that hive gives a ROSA cluster
	InfraName string

	// BaseDomain is the DNS base domain of the cluster
	BaseDomain string

	// VpcId is the AWS ID of the VPC the cluster is installed in
	VpcId string
}

func (c ClusterInfo) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("name", c.Name),
		slog.String("infraName", c.InfraName),
		slog.String("baseDomain", c.BaseDomain),
		slog.String("vpcId", c.VpcId),
	)
}

// NewClient looks up information in OCM about a given cluster id and returns a new
// mirrosa client. Requires valid AWS and OCM credentials to be present beforehand.
func NewClient(logger *slog.Logger, clusterId string, awsProxy string) (*Client, error) {
	ocmConn, err := ocm.CreateConnection()
	if err != nil {
		return nil, err
	}
	defer ocmConn.Close()

	cluster, err := ocm.GetCluster(ocmConn, clusterId)
	if err != nil {
		if err := ocmConn.Close(); err != nil {
			return nil, err
		}
		return nil, err
	}

	if cluster.CloudProvider().ID() != "aws" {
		return nil, fmt.Errorf("incompatible cloud provider: %s, mirrosa is only compatible with ROSA (AWS) clusters", cluster.CloudProvider().ID())
	}

	cfg, err := ocm.GetCloudCredentials(ocmConn, cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to generate cloud credentials: %w", err)
	}

	if awsProxy != "" {
		cfg.HTTPClient = &http.Client{
			Transport: &http.Transport{
				Proxy: func(*http.Request) (*url.URL, error) {
					return url.Parse(awsProxy)
				},
			},
		}
	}

	c := &Client{
		AwsConfig: cfg,
		Cluster:   cluster,
		ClusterInfo: &ClusterInfo{
			Name: cluster.Name(),
		},
		log: logger,
	}

	return c, nil
}

func NewRosaClient(ctx context.Context, logger *slog.Logger, clusterId string, awsProxy string) (*Client, error) {
	c, err := NewClient(logger, clusterId, awsProxy)
	if err != nil {
		return nil, err
	}

	if c.Cluster.Product().ID() != "rosa" && c.Cluster.Product().ID() != "osd" {
		return nil, fmt.Errorf("incompatible product type: %s, mirrosa is only compatible with ROSA clusters", c.Cluster.Product().ID())
	}

	if !c.Cluster.CCS().Enabled() {
		return nil, errors.New("mirrosa is only compatible with CCS clusters")
	}

	c.ClusterInfo.InfraName = c.Cluster.InfraID()
	c.ClusterInfo.BaseDomain = c.Cluster.DNS().BaseDomain()

	if err := c.FindVpcId(ctx); err != nil {
		return nil, fmt.Errorf("failed to find vpc id: %w", err)
	}

	return c, nil
}

// FindVpcId determines c.ClusterInfo.VpcId by determining the AWS VPC ID of a cluster
func (c *Client) FindVpcId(ctx context.Context) error {
	ec2Client := ec2.NewFromConfig(c.AwsConfig)

	if len(c.Cluster.AWS().SubnetIDs()) == 0 {
		// Non-BYOVPC, use the cluster's infra name to find the VPC id of the cluster
		resp, err := ec2Client.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{
			Filters: []types.Filter{
				{
					Name:   aws.String("tag:Name"),
					Values: []string{fmt.Sprintf("%s-vpc", c.ClusterInfo.InfraName)},
				},
				{
					Name:   aws.String(fmt.Sprintf("tag:kubernetes.io/cluster/%s", c.ClusterInfo.InfraName)),
					Values: []string{"owned"},
				},
			},
		})
		if err != nil {
			return fmt.Errorf("failed to describe vpc by tag: %w", err)
		}

		switch len(resp.Vpcs) {
		case 0:
			return fmt.Errorf("no VPCs found with expected Name tag: %s-vpc", c.ClusterInfo.InfraName)
		case 1:
			c.ClusterInfo.VpcId = *resp.Vpcs[0].VpcId
			return nil
		default:
			return fmt.Errorf("multiple VPCs found with the expected Name tag: %s-vpc", c.ClusterInfo.InfraName)
		}
	} else {
		// BYOVPC, use the provided subnets to find the VPC id of the cluster
		resp, err := ec2Client.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{SubnetIds: c.Cluster.AWS().SubnetIDs()})
		if err != nil {
			return fmt.Errorf("failed to find subnets by id: %w", err)
		}

		if len(resp.Subnets) == 0 {
			return fmt.Errorf("no subnets found for ids %v: %w", c.Cluster.AWS().SubnetIDs(), err)
		}

		c.ClusterInfo.VpcId = *resp.Subnets[0].VpcId
		return nil
	}
}

// ValidateComponents wraps the Validate method on one or many Component(s)
func (c *Client) ValidateComponents(ctx context.Context, components ...Component) error {
	for _, component := range components {
		if err := component.Validate(ctx); err != nil {
			return fmt.Errorf("%s: %w", component.Description(), err)
		}
	}

	return nil
}
