package mirrosa

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"go.uber.org/zap"
)

const dhcpOptionsDescription = "A ROSA cluster's DHCP Options Set must not have uppercase letters in its domain-name" +
	" due to https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#dns-subdomain-names."

// Ensure DhcpOptions implements Component
var _ Component = &DhcpOptions{}

type MirrosaDhcpOptionsAPIClient interface {
	ec2.DescribeDhcpOptionsAPIClient
	ec2.DescribeVpcsAPIClient
}

type DhcpOptions struct {
	log   *zap.SugaredLogger
	VpcId string

	Ec2Client MirrosaDhcpOptionsAPIClient
}

func (c *Client) NewDhcpOptions() DhcpOptions {
	return DhcpOptions{
		log:       c.log,
		VpcId:     c.ClusterInfo.VpcId,
		Ec2Client: ec2.NewFromConfig(c.AwsConfig),
	}
}

func (d DhcpOptions) Validate(ctx context.Context) error {
	d.log.Debugf("validating that the attached DHCP Options Set has no uppercase characters in its domain name(s)")
	vpcResp, err := d.Ec2Client.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{
		VpcIds: []string{d.VpcId},
	})
	if err != nil {
		return err
	}

	if len(vpcResp.Vpcs) != 1 {
		return fmt.Errorf("unexpectedly received %d VPCs when describing: %s", len(vpcResp.Vpcs), d.VpcId)
	}
	dhcpOptionsId := *vpcResp.Vpcs[0].DhcpOptionsId

	dhcpResp, err := d.Ec2Client.DescribeDhcpOptions(ctx, &ec2.DescribeDhcpOptionsInput{
		DhcpOptionsIds: []string{dhcpOptionsId},
	})
	if err != nil {
		return err
	}

	if len(dhcpResp.DhcpOptions) != 1 {
		return fmt.Errorf("unexepctedly received %d DHCP Options Sets when describing: %s", len(dhcpResp.DhcpOptions), dhcpOptionsId)
	}

	for _, config := range dhcpResp.DhcpOptions[0].DhcpConfigurations {
		switch *config.Key {
		case "domain-name":
			for _, v := range config.Values {
				d.log.Debugf("validating DHCP Options Set domain name: %s", *v.Value)
				if *v.Value != strings.ToLower(*v.Value) {
					return fmt.Errorf("DHCP Options set: %s contains uppercase letters in the domain name: %s", dhcpOptionsId, *v.Value)
				}
			}
		default:
			// Other DHCP Options set configurations have no hard rules
			continue
		}
	}

	return nil
}

func (d DhcpOptions) Description() string {
	return dhcpOptionsDescription
}

func (d DhcpOptions) FilterValue() string {
	return "DHCP Options"
}

func (d DhcpOptions) Title() string {
	return "DHCP Options"
}
