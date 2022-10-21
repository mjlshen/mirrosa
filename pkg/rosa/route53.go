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
)

type Route53AwsApi interface {
	GetHostedZone(ctx context.Context, params *route53.GetHostedZoneInput, optFns ...func(*route53.Options)) (*route53.GetHostedZoneOutput, error)
	ListHostedZonesByName(ctx context.Context, params *route53.ListHostedZonesByNameInput, optFns ...func(*route53.Options)) (*route53.ListHostedZonesByNameOutput, error)
	ListHostedZonesByVPC(ctx context.Context, params *route53.ListHostedZonesByVPCInput, optFns ...func(*route53.Options)) (*route53.ListHostedZonesByVPCOutput, error)
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
