package mirrosa

import (
	"context"
	"errors"
	"fmt"

	"github.com/mjlshen/mirrosa/pkg/ocm"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
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

	// BaseDomain is the DNS base domain of the cluster
	BaseDomain string

	// VpcId is the AWS ID of the VPC the cluster is installed in
	VpcId string

	// PublicHostedZoneId is the AWS ID of the Public Route53 Hosted Zone of the cluster
	PublicHostedZoneId string

	// PrivateHostedZoneId is the AWS ID of the Private Route53 Hosted Zone of the cluster
	PrivateHostedZoneId string
}

// NewClient looks up information in OCM about a given cluster id and returns a new
// mirrosa client. Requires valid AWS and OCM credentials to be present beforehand.
func NewClient(ctx context.Context, clusterId string) (*Client, error) {
	ocmConn, err := ocm.CreateConnection()
	if err != nil {
		return nil, err
	}

	cluster, err := ocm.GetCluster(ocmConn, clusterId)
	if err != nil {
		if err := ocmConn.Close(); err != nil {
			return nil, err
		}
		return nil, err
	}

	if err := ocmConn.Close(); err != nil {
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

	region := cluster.Region().ID()
	if region == "" {
		return nil, fmt.Errorf("empty region for cluster %s", clusterId)
	}

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, err
	}

	c := &Client{
		AwsConfig: cfg,
		Cluster:   cluster,
		ClusterInfo: &ClusterInfo{
			Name:       cluster.Name(),
			BaseDomain: cluster.DNS().BaseDomain(),
		},
	}

	return c, nil
}

// ValidateComponent wraps the Validate method on a specific Component
func (c *Client) ValidateComponent(ctx context.Context, component Component) (string, error) {
	return component.Validate(ctx)
}
