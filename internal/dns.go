package internal

import (
	"context"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/dns"
	"log/slog"
	"net/http"
)

const (
	recordType = "A"
	recordTtl  = 60
)

// rrSetClient is the subset of dns.DnsClient that UpdateDns uses. It exists so that the update logic can be
// exercised without OCI credentials.
type rrSetClient interface {
	GetRRSet(ctx context.Context, request dns.GetRRSetRequest) (dns.GetRRSetResponse, error)
	UpdateRRSet(ctx context.Context, request dns.UpdateRRSetRequest) (dns.UpdateRRSetResponse, error)
}

func OciDNSClient(config OciConfig) (*dns.DnsClient, error) {
	configProvider := common.NewRawConfigurationProvider(config.Tenancy, config.User, config.Region, config.Fingerprint,
		config.PrivateKey, nil)
	dnsClient, err := dns.NewDnsClientWithConfigurationProvider(configProvider)
	if err != nil {
		return nil, err
	}
	return &dnsClient, nil
}

// UpdateDns points every configured host at ipAddress and reports whether any record actually changed. Each RRSet
// is read first, so a client polling every few minutes does not rewrite a record that is already correct.
func UpdateDns(ctx context.Context, appConfig *AppConfig, dnsClient rrSetClient, ipAddress string,
	logger *slog.Logger) (changed bool, err error) {
	rtype := recordType
	ttl := recordTtl
	retryPolicy := common.DefaultRetryPolicy()

	for _, host := range appConfig.Hosts {
		if current, getErr := getRecords(ctx, appConfig, dnsClient, host, &retryPolicy); getErr != nil {
			// Treat an unreadable RRSet as "needs updating": the update below will report the error if it persists.
			logger.Info("DNS lookup failed, updating anyway", "host", host, "error", getErr)
		} else if recordsMatch(current, ipAddress, ttl) {
			logger.Info("DNS unchanged", "host", host, "ipAddress", ipAddress)
			continue
		}

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
		if updateErr != nil {
			if err == nil {
				err = updateErr
			}
			continue
		}
		changed = true
	}
	return changed, err
}

// getRecords returns the current contents of the host's A RRSet. A record that does not exist yet is reported as
// an empty RRSet rather than an error, since that is the normal first-run case.
func getRecords(ctx context.Context, appConfig *AppConfig, dnsClient rrSetClient, host string,
	retryPolicy *common.RetryPolicy) ([]dns.Record, error) {
	rtype := recordType
	request := dns.GetRRSetRequest{
		ZoneNameOrId:    &appConfig.Zone,
		Domain:          &host,
		Rtype:           &rtype,
		RequestMetadata: common.RequestMetadata{RetryPolicy: retryPolicy},
	}
	response, err := dnsClient.GetRRSet(ctx, request)
	if err != nil {
		if svcErr, ok := common.IsServiceError(err); ok && svcErr.GetHTTPStatusCode() == http.StatusNotFound {
			return nil, nil
		}
		return nil, err
	}
	return response.Items, nil
}

// recordsMatch reports whether the RRSet already holds exactly the single A record that UpdateDns would write.
func recordsMatch(records []dns.Record, ipAddress string, ttl int) bool {
	if len(records) != 1 {
		return false
	}
	record := records[0]
	return record.Rdata != nil && *record.Rdata == ipAddress && record.Ttl != nil && *record.Ttl == ttl
}
