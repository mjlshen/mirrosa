package ocm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/openshift-online/ocm-cli/pkg/ocm"
	sdk "github.com/openshift-online/ocm-sdk-go"
	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
)

const (
	ClusterServiceClusterSearch   = "id = '%s' or name = '%s' or external_id = '%s'"
	backplaneCloudCredentialsPath = "/backplane/cloud/credentials"
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

// GetToken retrieves the OCM token used to create the connection
func GetToken(ctx context.Context, conn *sdk.Connection) (string, error) {
	token, _, err := conn.TokensContext(ctx)
	return strings.TrimSuffix(token, "\n"), err
}

type BackplaneCloudCredentialsResponse struct {
	Cluster     string  `json:"clusterID"`
	ConsoleLink *string `json:"consoleLink,omitempty"`
	Credentials *string `json:"credentials,omitempty"`
	Region      string  `json:"region"`
}

type AWSCredentialsResponse struct {
	AccessKeyId     string `json:"AccessKeyId"`
	SecretAccessKey string `json:"SecretAccessKey"`
	SessionToken    string `json:"SessionToken"`
	Region          string `json:"Region"`
	Expiration      string `json:"Expiration"`
}

// GetCloudCredentials sets up AWS credentials via backplane-api given a cluster id, OCM token, and backplane-api URL
func GetCloudCredentials(ctx context.Context, backplaneUrl *url.URL, clusterId string, token string) (aws.Config, error) {
	cloudCredentialsUrl, err := backplaneUrl.Parse(fmt.Sprintf("%s/%s", backplaneCloudCredentialsPath, clusterId))
	if err != nil {
		return aws.Config{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cloudCredentialsUrl.String(), nil)
	if err != nil {
		return aws.Config{}, err
	}

	req.Header.Add("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return aws.Config{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return aws.Config{}, fmt.Errorf("received status code %d", resp.StatusCode)
	}

	respBodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return aws.Config{}, err
	}

	credsResp := new(BackplaneCloudCredentialsResponse)
	if err := json.Unmarshal(respBodyBytes, credsResp); err != nil {
		return aws.Config{}, err
	}

	awsCrds := new(AWSCredentialsResponse)
	if err := json.Unmarshal([]byte(*credsResp.Credentials), awsCrds); err != nil {
		return aws.Config{}, err
	}

	return config.LoadDefaultConfig(ctx,
		config.WithCredentialsProvider(
			aws.NewCredentialsCache(
				credentials.NewStaticCredentialsProvider(
					awsCrds.AccessKeyId,
					awsCrds.SecretAccessKey,
					awsCrds.SessionToken,
				),
			),
		),
		config.WithRegion(credsResp.Region),
	)
}

// GetHiveShard retrieves the url of the hive shard associated with a cluster
func GetHiveShard(conn *sdk.Connection, clusterId string) (*url.URL, error) {
	hive, err := conn.ClustersMgmt().V1().Clusters().Cluster(clusterId).ProvisionShard().Get().Send()
	if err != nil {
		return nil, err
	}

	hiveShardUrl, ok := hive.Body().HiveConfig().GetServer()
	if !ok {
		return nil, fmt.Errorf("no hive shard found for %s", clusterId)
	}

	return url.Parse(hiveShardUrl)
}

// GenerateBackplaneUrl takes a cluster's hive shard url and converts it to the backplane api url.
// The hive shard url is typically in the form of: https://api.${HIVE_SHARD}.${SLUG}.${ENV}.${DOMAIN}:6443
// while backplane-api is hosted at https://api-backplane.apps.${HIVE_SHARD}.${SLUG}.${ENV}.${DOMAIN}
func GenerateBackplaneUrl(hiveShardUrl *url.URL) (*url.URL, error) {
	backplaneUrl := hiveShardUrl

	// Just ensure it's https, typically this is a no-op
	backplaneUrl.Scheme = "https"

	// Strip off the port and replace api --> api-backplane.apps
	backplaneUrl.Host = strings.Replace(backplaneUrl.Hostname(), "api", "api-backplane.apps", 1)

	return backplaneUrl, nil
}
