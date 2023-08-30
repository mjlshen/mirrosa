package mirrosa

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/route53/types"
)

const (
	publicHostedZoneDescription = "A ROSA cluster's public hosted zone holds information about how to route traffic" +
		" on the internet to a cluster's API Server and Ingress."
	publicHostedZonePrivateLinkDescription = "A PrivateLink ROSA cluster's public hosted zone is typically empty," +
		" but is required for Let's Encrypt to complete DNS-01 challenges by populating specific TXT records" +
		" to prove ownership and renew TLS certificates for the cluster."
	privateHostedZoneDescription = "A ROSA cluster's private hosted zone holds information about how Route 53" +
		" to DNS queries within the associated VPC. Records for api-int, api, and *.apps are required." +
		"\n  - api so that the API server is routable." +
		"\n  - api.int so that the API server is routable within the cluster's VPC." +
		"\n  - *.apps so that applications running on the cluster are routable when exposed by an Ingress" +
		", including the OpenShift console."
	// \052 is ASCII for *
	privateHostedZoneAppsRecordPrefix   = "\\052.apps"
	privateHostedZoneApiRecordPrefix    = "api"
	privateHostedZoneApiIntRecordPrefix = "api-int"
)

// Ensure PublicHostedZone implements mirrosa.Component
var _ Component = &PublicHostedZone{}

type Route53AwsApi interface {
	GetHostedZone(ctx context.Context, params *route53.GetHostedZoneInput, optFns ...func(*route53.Options)) (*route53.GetHostedZoneOutput, error)
	ListHostedZonesByName(ctx context.Context, params *route53.ListHostedZonesByNameInput, optFns ...func(*route53.Options)) (*route53.ListHostedZonesByNameOutput, error)
	ListResourceRecordSets(ctx context.Context, params *route53.ListResourceRecordSetsInput, optFns ...func(*route53.Options)) (*route53.ListResourceRecordSetsOutput, error)
}

type PublicHostedZone struct {
	log         *slog.Logger
	BaseDomain  string
	PrivateLink bool

	Route53Client Route53AwsApi
}

func (c *Client) NewPublicHostedZone() PublicHostedZone {
	return PublicHostedZone{
		log:           c.log,
		BaseDomain:    c.ClusterInfo.BaseDomain,
		PrivateLink:   c.Cluster.AWS().PrivateLink(),
		Route53Client: route53.NewFromConfig(c.AwsConfig),
	}
}

func (p PublicHostedZone) Validate(ctx context.Context) error {
	expectedName := fmt.Sprintf("%s.", p.BaseDomain)

	p.log.Info("searching for Public Hosted Zone", slog.String("name", expectedName))
	hzs, err := p.Route53Client.ListHostedZonesByName(ctx, &route53.ListHostedZonesByNameInput{
		DNSName: aws.String(expectedName),
	})
	if err != nil {
		return err
	}

	for _, hz := range hzs.HostedZones {
		if !hz.Config.PrivateZone {
			p.log.Info("found Public Hosted Zone", slog.String("id", *hz.Id))
			return nil
		}
	}

	return fmt.Errorf("no public hosted zone for %s found", expectedName)
}

func (p PublicHostedZone) Description() string {
	if p.PrivateLink {
		return publicHostedZonePrivateLinkDescription
	}

	return publicHostedZoneDescription
}

func (p PublicHostedZone) FilterValue() string {
	return "Route53 Public Hosted Zone"
}

func (p PublicHostedZone) Title() string {
	return "Route53 Public Hosted Zone"
}

// Ensure PrivateHostedZone implements mirrosa.Component
var _ Component = &PrivateHostedZone{}

type PrivateHostedZone struct {
	log         *slog.Logger
	ClusterName string
	BaseDomain  string
	Region      types.VPCRegion
	VpcId       string

	Route53Client Route53AwsApi
}

func (c *Client) NewPrivateHostedZone() PrivateHostedZone {
	return PrivateHostedZone{
		log:           c.log,
		ClusterName:   c.ClusterInfo.Name,
		BaseDomain:    c.ClusterInfo.BaseDomain,
		Region:        types.VPCRegion(c.Cluster.Region().ID()),
		VpcId:         c.ClusterInfo.VpcId,
		Route53Client: route53.NewFromConfig(c.AwsConfig),
	}
}

func (p PrivateHostedZone) Validate(ctx context.Context) error {
	if p.VpcId == "" || p.BaseDomain == "" || p.ClusterName == "" {
		return errors.New("must specify a BaseDomain, ClusterName, and VpcId")
	}

	p.log.Info("searching for Private Hosted Zone", slog.String("name", fmt.Sprintf("%s.%s", p.ClusterName, p.BaseDomain)))
	var privateHostedZoneId string
	expectedName := fmt.Sprintf("%s.%s.", p.ClusterName, p.BaseDomain)

	resp, err := p.Route53Client.ListHostedZonesByName(ctx, &route53.ListHostedZonesByNameInput{
		DNSName: aws.String(expectedName),
	})
	if err != nil {
		return err
	}

	for _, hz := range resp.HostedZones {
		if hz.Config.PrivateZone && *hz.Name == expectedName {
			p.log.Debug("considering Hosted Zone", slog.String("id", *hz.Id), slog.String("name", *hz.Name))
			private, err := p.Route53Client.GetHostedZone(ctx, &route53.GetHostedZoneInput{
				Id: hz.Id,
			})
			if err != nil {
				return err
			}

			if len(private.VPCs) == 0 {
				p.log.Debug("Hosted Zone is not associated with any VPCs", slog.String("id", *hz.Id))
				continue
			} else {
				for _, vpc := range private.VPCs {
					if *vpc.VPCId == p.VpcId {
						p.log.Info("found Private Hosted Zone", slog.String("id", *private.HostedZone.Id))
						privateHostedZoneId = *private.HostedZone.Id
						break
					}
				}
			}
		}
	}

	if privateHostedZoneId == "" {
		return fmt.Errorf("no private hosted zone associated to %s for %s found", p.VpcId, expectedName)
	}

	p.log.Info("validating records in Private Hosted Zone", slog.String("id", privateHostedZoneId))
	// TODO: Handle pagination
	records, err := p.Route53Client.ListResourceRecordSets(ctx, &route53.ListResourceRecordSetsInput{
		HostedZoneId: aws.String(privateHostedZoneId),
	})
	if err != nil {
		return err
	}

	expectedRecords := map[string]struct{}{
		fmt.Sprintf("%s.%s.%s.", privateHostedZoneApiRecordPrefix, p.ClusterName, p.BaseDomain):    {},
		fmt.Sprintf("%s.%s.%s.", privateHostedZoneApiIntRecordPrefix, p.ClusterName, p.BaseDomain): {},
		fmt.Sprintf("%s.%s.%s.", privateHostedZoneAppsRecordPrefix, p.ClusterName, p.BaseDomain):   {},
	}

	for _, record := range records.ResourceRecordSets {
		// If we've found all the required records, stop
		if len(expectedRecords) == 0 {
			break
		}

		if _, ok := expectedRecords[*record.Name]; ok {
			p.log.Debug("found record", slog.String("name", *record.Name))
			// All expected records are A records
			if record.Type != types.RRTypeA || record.AliasTarget == nil {
				return fmt.Errorf("%s has no value or an incorrect type", *record.Name)
			}
			delete(expectedRecords, *record.Name)
		}
	}

	if len(expectedRecords) > 0 {
		return fmt.Errorf("missing required records in private hosted zone %s", privateHostedZoneId)
	}

	return nil
}

func (p PrivateHostedZone) Description() string {
	return privateHostedZoneDescription
}

func (p PrivateHostedZone) FilterValue() string {
	return "Route53 Private Hosted Zone"
}

func (p PrivateHostedZone) Title() string {
	return "Route53 Private Hosted Zone"
}
