package internal

import (
	"context"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/dns"
	"log/slog"
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

func UpdateDns(ctx context.Context, appConfig *AppConfig, dnsClient *dns.DnsClient, ipAddress string,
	logger *slog.Logger) (err error) {
	rtype := "A"
	ttl := 60
	retryPolicy := common.DefaultRetryPolicy()

	for _, host := range appConfig.Hosts {
		request := dns.UpdateRRSetRequest{
			CompartmentId: &appConfig.OciConfig.Tenancy,
			ZoneNameOrId:  &appConfig.Zone,
			Domain:        &host,
			Rtype:         &rtype,
			UpdateRrSetDetails: dns.UpdateRrSetDetails{
				Items: []dns.RecordDetails{
					{
						Domain: &host,
						Rtype:  &rtype,
						Rdata:  &ipAddress,
						Ttl:    &ttl,
					},
				},
			},
			RequestMetadata: common.RequestMetadata{RetryPolicy: &retryPolicy},
		}
		_, updateErr := dnsClient.UpdateRRSet(ctx, request)
		logger.Info("DNS update", "host", host, "ipAddress", ipAddress, "error", updateErr)
		if (updateErr != nil) && (err == nil) {
			err = updateErr
		}
	}
	return err
}
