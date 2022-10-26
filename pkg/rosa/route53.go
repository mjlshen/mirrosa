package rosa

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/route53/types"
	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"

	"github.com/mjlshen/mirrosa/pkg/mirrosa"
)

const (
	publicHostedZoneDescription = "A ROSA cluster's public hosted zone holds information about how to route traffic" +
		" on the internet to a cluster's API Server and Ingress."
	publicHostedZonePrivateLinkDescription = "A PrivateLink ROSA cluster's public hosted zone is typically empty," +
		" but is required for Let's Encrypt to complete DNS-01 challenges by populating specific TXT records" +
		" to prove ownership and renew TLS certificates for the cluster."
	privateHostedZoneDescription = "A ROSA cluster's private hosted zone holds information about how Route 53" +
		" to DNS queries within the associated VPC. Records for api-int, api, and *.apps are required."
	privateHostedZoneRecordsDescription = "A ROSA cluster's private hosted zone must contain a minimum of three records:" +
		"\n  - api so that the API server is routable." +
		"\n  - api.int so that the API server is routable within the cluster's VPC." +
		"\n  - *.apps so that applications running on the cluster are routable when exposed by an Ingress" +
		", including the OpenShift console."
)

type Route53AwsApi interface {
	GetHostedZone(ctx context.Context, params *route53.GetHostedZoneInput, optFns ...func(*route53.Options)) (*route53.GetHostedZoneOutput, error)
	ListHostedZonesByName(ctx context.Context, params *route53.ListHostedZonesByNameInput, optFns ...func(*route53.Options)) (*route53.ListHostedZonesByNameOutput, error)
	ListHostedZonesByVPC(ctx context.Context, params *route53.ListHostedZonesByVPCInput, optFns ...func(*route53.Options)) (*route53.ListHostedZonesByVPCOutput, error)
	ListResourceRecordSets(ctx context.Context, params *route53.ListResourceRecordSetsInput, optFns ...func(*route53.Options)) (*route53.ListResourceRecordSetsOutput, error)
}

var _ mirrosa.Component = &PublicHostedZone{}

type PublicHostedZone struct {
	BaseDomain  string
	PrivateLink bool

	Route53Client Route53AwsApi
}

func NewPublicHostedZone(cluster *cmv1.Cluster, api Route53AwsApi) PublicHostedZone {
	return PublicHostedZone{
		BaseDomain:    cluster.DNS().BaseDomain(),
		PrivateLink:   cluster.AWS().PrivateLink(),
		Route53Client: api,
	}
}

func (p PublicHostedZone) Validate(ctx context.Context) (string, error) {
	if p.BaseDomain == "" {
		return "", errors.New("must specify a BaseDomain")
	}

	expectedName := fmt.Sprintf("%s.", p.BaseDomain)

	hzs, err := p.Route53Client.ListHostedZonesByName(ctx, &route53.ListHostedZonesByNameInput{
		DNSName: aws.String(expectedName),
	})
	if err != nil {
		return "", err
	}

	for _, hz := range hzs.HostedZones {
		if !hz.Config.PrivateZone {
			return *hz.Id, nil
		}
	}

	return "", fmt.Errorf("no public hosted zone for %s found", expectedName)
}

func (p PublicHostedZone) Documentation() string {
	if p.PrivateLink {
		return publicHostedZonePrivateLinkDescription
	}

	return publicHostedZoneDescription
}

// Ensure PrivateHostedZone implements mirrosa.Component
var _ mirrosa.Component = &PrivateHostedZone{}

type PrivateHostedZone struct {
	ClusterName string
	BaseDomain  string
	Region      types.VPCRegion
	VpcId       string

	Route53Client Route53AwsApi
}

func NewPrivateHostedZone(cluster *cmv1.Cluster, api Route53AwsApi, vpcId string) PrivateHostedZone {
	return PrivateHostedZone{
		ClusterName:   cluster.Name(),
		BaseDomain:    cluster.DNS().BaseDomain(),
		Region:        types.VPCRegion(cluster.Region().ID()),
		VpcId:         vpcId,
		Route53Client: api,
	}
}

func (p PrivateHostedZone) Validate(ctx context.Context) (string, error) {
	if p.VpcId == "" || p.BaseDomain == "" || p.ClusterName == "" {
		return "", errors.New("must specify a BaseDomain, ClusterName, and VpcId")
	}

	expectedName := fmt.Sprintf("%s.%s.", p.ClusterName, p.BaseDomain)

	hzs, err := p.Route53Client.ListHostedZonesByVPC(ctx, &route53.ListHostedZonesByVPCInput{
		VPCId:     aws.String(p.VpcId),
		VPCRegion: p.Region,
	})
	if err != nil {
		return "", err
	}

	for _, hz := range hzs.HostedZoneSummaries {
		if *hz.Name == expectedName {
			private, err := p.Route53Client.GetHostedZone(ctx, &route53.GetHostedZoneInput{
				Id: hz.HostedZoneId,
			})
			if err != nil {
				return "", err
			}
			return *private.HostedZone.Id, nil
		}
	}

	return "", fmt.Errorf("no private hosted zone associated to %s for %s found", p.VpcId, expectedName)
}

func (p PrivateHostedZone) Documentation() string {
	return privateHostedZoneDescription
}

type PrivateHostedZoneRecords struct {
	BaseDomain          string
	ClusterName         string
	PrivateHostedZoneId string

	Route53Client Route53AwsApi
}

// Ensure PrivateHostedZoneRecords implements mirrosa.Component
var _ mirrosa.Component = &PrivateHostedZoneRecords{}

func NewPrivateHostedZoneRecords(cluster *cmv1.Cluster, api Route53AwsApi, privateHostedZoneId string) PrivateHostedZoneRecords {
	return PrivateHostedZoneRecords{
		BaseDomain:          cluster.DNS().BaseDomain(),
		ClusterName:         cluster.Name(),
		PrivateHostedZoneId: privateHostedZoneId,
		Route53Client:       api,
	}
}

func (p PrivateHostedZoneRecords) Validate(ctx context.Context) (string, error) {
	if p.PrivateHostedZoneId == "" {
		return "", errors.New("must specify a private hosted zone id")
	}

	// TODO: Handle pagination
	records, err := p.Route53Client.ListResourceRecordSets(ctx, &route53.ListResourceRecordSetsInput{
		HostedZoneId: aws.String(p.PrivateHostedZoneId),
	})
	if err != nil {
		return "", err
	}

	api, apiInt, apps := false, false, false
	for _, record := range records.ResourceRecordSets {
		if api && apiInt && apps {
			return "", nil
		}

		switch *record.Name {
		case fmt.Sprintf("api.%s.%s.", p.ClusterName, p.BaseDomain):
			fmt.Println("found api")
			api = true
		case fmt.Sprintf("api-int.%s.%s.", p.ClusterName, p.BaseDomain):
			fmt.Println("found api-int")
			apiInt = true
		case fmt.Sprintf("\\052.apps.%s.%s.", p.ClusterName, p.BaseDomain):
			// \052 is ASCII for *
			fmt.Println("found apps")
			apps = true
		}
	}

	return "", fmt.Errorf("missing required records for api, api-int, or *.apps")
}

func (p PrivateHostedZoneRecords) Documentation() string {
	return privateHostedZoneRecordsDescription
}
