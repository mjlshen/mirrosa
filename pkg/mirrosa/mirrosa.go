package mirrosa

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"

	"github.com/mjlshen/mirrosa/pkg/ocm"
	"github.com/mjlshen/mirrosa/pkg/rosa"
	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
)

// Client holds relevant information about a ROSA cluster gleaned from OCM and an AwsApi to validate the cluster in AWS
type Client struct {
	// Cluster holds a cluster object from OCM
	Cluster *cmv1.Cluster

	// AwsApi is used to connect to AWS and validate cloud infrastructure
	AwsApi *rosa.RosaClient

	// ClusterInfo contains information about the ROSA cluster that will be used to validate it
	ClusterInfo *ClusterInfo
}

// ClusterInfo contains information about the ROSA cluster that will be used to validate it
type ClusterInfo struct {
	// Name of the cluster
	Name string

	// BaseDomain is the DNS base domain of the cluster
	BaseDomain string

	// VpcId is the AWS VPC ID the cluster is installed in
	VpcId string
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

	region := cluster.Region().ID()
	if region == "" {
		return nil, fmt.Errorf("empty region for cluster %s", clusterId)
	}

	rosaClient, err := rosa.NewClient(ctx, config.WithRegion(region))
	if err != nil {
		return nil, err
	}

	c := &Client{
		AwsApi:  rosaClient,
		Cluster: cluster,
		ClusterInfo: &ClusterInfo{
			Name:       cluster.Name(),
			BaseDomain: cluster.DNS().BaseDomain(),
		},
	}

	if err := c.DetermineVpcId(ctx); err != nil {
		return nil, err
	}

	return c, nil
}

// DetermineVpcId populates the VpcId field of the Client struct based on the type of cluster
func (c *Client) DetermineVpcId(ctx context.Context) error {
	var (
		vpcId *string
		err   error
	)

	if ocm.IsClusterByovpc(c.Cluster) {
		vpcId, err = c.AwsApi.GetVpcIdFromSubnetId(ctx, c.Cluster.AWS().SubnetIDs()[0])
		if err != nil {
			return err
		}

	} else {
		vpcId, err = c.AwsApi.GetVpcIdFromBaseDomain(ctx, c.ClusterInfo.Name, c.ClusterInfo.BaseDomain)
		if err != nil {
			return err
		}

	}

	c.ClusterInfo.VpcId = *vpcId

	return nil
}
