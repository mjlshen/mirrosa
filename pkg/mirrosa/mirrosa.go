package mirrosa

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"

	"github.com/mjlshen/mirrosa/pkg/ocm"
	"github.com/mjlshen/mirrosa/pkg/rosa"
	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
)

// Client holds relevant data about a ROSA cluster gleaned from OCM
// and an AwsApi to validate the cluster in AWS
type Client struct {
	// Cluster holds a cluster object from OCM
	Cluster *cmv1.Cluster

	// AwsApi is used to connect to AWS and validate cloud infrastructure
	AwsApi *rosa.RosaClient
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

	return &Client{
		AwsApi:  rosaClient,
		Cluster: cluster,
	}, nil
}
