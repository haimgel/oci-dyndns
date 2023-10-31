package internal

import (
	"context"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/dns"
)

func OciDNSClient(config OciConfig) (*dns.DnsClient, error) {
	configProvider := common.NewRawConfigurationProvider(config.Tenancy, config.User, config.Region, config.Fingerprint,
		config.PrivateKey, nil)
	dnsClient, err := dns.NewDnsClientWithConfigurationProvider(configProvider)
	if err != nil {
		return nil, err
	}
	return &dnsClient, nil
}

func UpdateDns(appConfig *AppConfig, dnsClient *dns.DnsClient, ctx context.Context, ipAddress string) error {
	rtype := "A"
	ttl := 60
	retryPolicy := common.DefaultRetryPolicy()

	request := dns.UpdateRRSetRequest{
		CompartmentId: &appConfig.OciConfig.Tenancy,
		ZoneNameOrId:  &appConfig.Zone,
		Domain:        &appConfig.Host,
		Rtype:         &rtype,
		UpdateRrSetDetails: dns.UpdateRrSetDetails{
			Items: []dns.RecordDetails{
				{
					Domain: &appConfig.Host,
					Rtype:  &rtype,
					Rdata:  &ipAddress,
					Ttl:    &ttl,
				},
			},
		},
		RequestMetadata: common.RequestMetadata{RetryPolicy: &retryPolicy},
	}
	_, err := dnsClient.UpdateRRSet(ctx, request)
	return err
}
