package ocm

import (
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/openshift-online/ocm-cli/pkg/ocm"
	sdk "github.com/openshift-online/ocm-sdk-go"
	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	"github.com/openshift/backplane-cli/cmd/ocm-backplane/cloud"
	bpconfig "github.com/openshift/backplane-cli/pkg/cli/config"
)

const (
	ClusterServiceClusterSearch = "id = '%s' or name = '%s' or external_id = '%s'"
)

// CreateConnection takes care of common friendly errors and creates an OCM connection.
// As a consumer, remember to close the connection when you are done using it.
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

// GetCluster returns an OCM cluster object given an OCM connection and cluster id
// (internal and external ids both supported).
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

// GetCloudCredentials sets up AWS credentials via backplane-api given a cluster id, OCM token, and backplane-api URL
func GetCloudCredentials(conn *sdk.Connection, cluster *cmv1.Cluster) (aws.Config, error) {
	bp, err := bpconfig.GetBackplaneConfiguration()
	if err != nil {
		return aws.Config{}, err
	}

	qc := &cloud.QueryConfig{
		BackplaneConfiguration: bp,
		OcmConnection:          conn,
		Cluster:                cluster,
	}

	return qc.GetAWSV2Config()
}
