package ocm

import (
	"errors"
	"fmt"
	"strings"

	"github.com/openshift-online/ocm-cli/pkg/ocm"
	sdk "github.com/openshift-online/ocm-sdk-go"
	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
)

const (
	ClusterServiceClusterSearch = "id = '%s' or name = '%s' or external_id = '%s'"
	CloudProviderAws            = "aws"
)

func CreateConnection() (*sdk.Connection, error) {
	connection, err := ocm.NewConnection().Build()
	if err != nil {
		if strings.Contains(err.Error(), "Not logged in, run the") {
			return nil, errors.New("failed to create OCM connection: Authentication error, run 'ocm login' first")
		}
		return nil, fmt.Errorf("failed to create OCM connection: %v", err)
	}

	return connection, nil
}

func GetCluster(conn *sdk.Connection, clusterId string) (*cmv1.Cluster, error) {
	// identifier in the accounts management service. To find those clusters we need to check
	// directly in the clusters management service.
	clustersSearch := fmt.Sprintf(ClusterServiceClusterSearch, clusterId, clusterId, clusterId)
	clustersListResponse, err := conn.ClustersMgmt().V1().Clusters().List().Search(clustersSearch).Size(1).Send()
	if err != nil {
		return nil, fmt.Errorf("can't retrieve clusters for clusterId '%s': %v", clusterId, err)
	}

	// If there is exactly one cluster matching then return it:
	clustersTotal := clustersListResponse.Total()
	if clustersTotal == 1 {
		return clustersListResponse.Items().Slice()[0], nil
	}

	return nil, fmt.Errorf("there are %d clusters with identifier or name '%s', expected 1", clustersTotal, clusterId)
}

// IsClusterSts returns true if this is an STS cluster
func IsClusterSts(cluster *cmv1.Cluster) bool {
	if cluster.CloudProvider().ID() != "aws" {
		return false
	}

	if cluster.AWS().STS().RoleARN() != "" {
		return true
	}

	return false
}

// IsClusterByovpc returns true if this cluster was installed into an existing VPC
func IsClusterByovpc(cluster *cmv1.Cluster) bool {
	if cluster.CloudProvider().ID() != CloudProviderAws {
		return false
	}

	if len(cluster.AWS().SubnetIDs()) > 0 {
		return true
	}

	return false
}

// IsClusterPrivateLink returns true if the cluster's API Server is not publicly accessible
func IsClusterPrivateLink(cluster *cmv1.Cluster) bool {
	return cluster.AWS().PrivateLink()
}

// IsClusterCCS returns true if the cluster is installed in a customer's AWS account
func IsClusterCCS(cluster *cmv1.Cluster) bool {
	return cluster.CCS().Enabled()
}
