package mirrosa

import (
	"context"
	"errors"
	"fmt"
	"github.com/mjlshen/mirrosa/pkg/ocm"

	"github.com/aws/aws-sdk-go-v2/aws"
	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
)

// Client holds relevant information about a ROSA cluster gleaned from OCM and an AwsApi to validate the cluster in AWS
type Client struct {
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

	// PublicHostedZoneId is the AWS ID of the Public Route53 Hosted Zone of the cluster
	PublicHostedZoneId string

	// PrivateHostedZoneId is the AWS ID of the Private Route53 Hosted Zone of the cluster
	PrivateHostedZoneId string

	// AppsLbSecurityGroupId is the AWS ID of the cluster's *.apps load balancer
	AppsLbSecurityGroupId string

	// ApiLbSecurityGroupId is the AWS ID of the cluster's api load balancer
	ApiLbSecurityGroupId string

	// ApiIntLbSecurityGroupId is the AWS ID of the cluster's api.int load balancer
	ApiIntLbSecurityGroupId string
}

// NewClient looks up information in OCM about a given cluster id and returns a new
// mirrosa client. Requires valid AWS and OCM credentials to be present beforehand.
func NewClient(ctx context.Context, clusterId, infraName string) (*Client, error) {
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

	if cluster.Product().ID() != "rosa" && cluster.Product().ID() != "osd" {
		return nil, fmt.Errorf("incompatible product type: %s, mirrosa is only compatible with ROSA clusters", cluster.Product().ID())
	}

	if !cluster.CCS().Enabled() {
		return nil, errors.New("mirrosa is only compatible with CCS clusters")
	}

	token, err := ocm.GetToken(ocmConn)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve ocm token: %w", err)
	}

	hiveShard, err := ocm.GetHiveShard(ocmConn, clusterId)
	if err != nil {
		return nil, fmt.Errorf("failed to get hive shard: %w", err)
	}

	backplaneUrl, err := ocm.GenerateBackplaneUrl(hiveShard)
	if err != nil {
		return nil, fmt.Errorf("failed to generate backplane url: %w", err)
	}

	cfg, err := ocm.GetCloudCredentials(ctx, backplaneUrl, clusterId, token)
	if err != nil {
		return nil, fmt.Errorf("failed to generate cloud credentials: %w", err)
	}

	if infraName == "" {
		infraName = cluster.InfraID()
	}

	if infraName == "" {
		return nil, fmt.Errorf("unable to determine infra name from OCM, please specify with --infra-name")
	}

	c := &Client{
		AwsConfig: cfg,
		Cluster:   cluster,
		ClusterInfo: &ClusterInfo{
			Name:       cluster.Name(),
			InfraName:  cluster.InfraID(),
			BaseDomain: cluster.DNS().BaseDomain(),
		},
	}

	return c, nil
}

// ValidateComponent wraps the Validate method on a specific Component
func (c *Client) ValidateComponent(ctx context.Context, component Component) (string, error) {
	return component.Validate(ctx)
}
