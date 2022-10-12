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
	OcmClusterSubscriptionSearch = "(name = '%s' or cluster_id = '%s' or external_cluster_id = '%s') and " +
		"status in ('Reserved', 'Active')"
	ClusterServiceClusterSearch = "id = '%s' or name = '%s' or external_id = '%s'"
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

func GetCluster(connection *sdk.Connection, key string) (*cmv1.Cluster, error) {
	// Prepare the resources that we will be using:
	clustersResource := connection.ClustersMgmt().V1().Clusters()

	//// Try to find a matching subscription:
	//subsSearch := fmt.Sprintf(OcmClusterSubscriptionSearch, key, key, key)
	//subsListResponse, err := connection.AccountsMgmt().V1().Subscriptions().
	//	List().Search(subsSearch).Size(1).Send()
	//if err != nil {
	//	return nil, fmt.Errorf("can't retrieve subscription for key '%s': %v", key, err)
	//}
	//
	//// If there are multiple subscriptions that match the cluster then we should report it as
	//// an error:
	//subsTotal := subsListResponse.Total()
	//if subsTotal > 1 {
	//	return nil, fmt.Errorf("there are %d subscriptions with cluster identifier or name '%s'", subsTotal, key)
	//}
	//
	//// If there is exactly one matching subscription then return the corresponding cluster:
	//if subsTotal == 1 {
	//	id, ok := subsListResponse.Items().Slice()[0].GetClusterID()
	//	if ok {
	//		clusterGetResponse, err := clustersResource.Cluster(id).Get().Send()
	//		if err != nil {
	//			return nil, fmt.Errorf("can't retrieve cluster for key '%s': %v", key, err)
	//		}
	//
	//		return clusterGetResponse.Body(), nil
	//	}
	//}

	// If we are here then no subscription matches the passed key. It may still be possible that
	// the cluster exists but it is not reporting metrics, so it will not have the external
	// identifier in the accounts management service. To find those clusters we need to check
	// directly in the clusters management service.
	clustersSearch := fmt.Sprintf(ClusterServiceClusterSearch, key, key, key)
	clustersListResponse, err := clustersResource.List().Search(clustersSearch).Size(1).Send()
	if err != nil {
		return nil, fmt.Errorf("can't retrieve clusters for key '%s': %v", key, err)
	}

	// If there is exactly one cluster matching then return it:
	clustersTotal := clustersListResponse.Total()
	if clustersTotal == 1 {
		return clustersListResponse.Items().Slice()[0], nil
	}

	// If there are multiple matching clusters then we should report it as an error:
	if clustersTotal > 1 {
		err = fmt.Errorf(
			"there are %d clusters with identifier or name '%s'",
			clustersTotal, key,
		)
		return
	}

	// If we are here then there are no subscriptions or clusters matching the passed key:
	err = fmt.Errorf(
		"There are no subscriptions or clusters with identifier or name '%s'",
		key,
	)
	return
}
