package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/dns"
	"golang.org/x/exp/slices"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
)

type OciConfig struct {
	Tenancy              string `json:"tenancy"`
	User                 string `json:"user"`
	Region               string `json:"region"`
	Fingerprint          string `json:"fingerprint"`
	PrivateKey           string `json:"privateKey"`
	PrivateKeyPassphrase string `json:"privateKeyPassphrase"`
}

type appConfig struct {
	OciConfig OciConfig `json:"oci"`
	Zone      string    `json:"zone"`
	Host      string    `json:"host"`
	Token     string    `json:"token"`
}

type response struct {
	Message string `json:"message"`
}

func loadAppConfig(fileName *string) (appConfig, error) {
	appConfig := appConfig{}

	jsonFile, err := os.Open(*fileName)
	if err != nil {
		return appConfig, fmt.Errorf("unable to open config file: %v", err)
	}
	// noinspection GoUnhandledErrorResult
	defer jsonFile.Close()
	jsonBytes, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		return appConfig, fmt.Errorf("unable to read config file: %v", err)
	}
	if err := json.Unmarshal(jsonBytes, &appConfig); err != nil {
		return appConfig, fmt.Errorf("cannot parse the config file: %v", err)
	}
	return appConfig, nil
}

func ociDNSClient(config OciConfig) (*dns.DnsClient, error) {
	configProvider := common.NewRawConfigurationProvider(config.Tenancy, config.User, config.Region, config.Fingerprint,
		config.PrivateKey, nil)
	dnsClient, err := dns.NewDnsClientWithConfigurationProvider(configProvider)
	if err != nil {
		return nil, err
	}
	return &dnsClient, nil
}

func updateDns(appConfig *appConfig, dnsClient *dns.DnsClient, ctx context.Context, ipAddress string) error {
	rtype := "A"
	ttl := 60
	retryPolicy := common.DefaultRetryPolicy()

	request := dns.PatchZoneRecordsRequest{
		CompartmentId: &appConfig.OciConfig.Tenancy,
		ZoneNameOrId:  &appConfig.Zone,
		PatchZoneRecordsDetails: dns.PatchZoneRecordsDetails{
			Items: []dns.RecordOperation{
				{
					Domain:    &appConfig.Host,
					Rtype:     &rtype,
					Rdata:     &ipAddress,
					Ttl:       &ttl,
					Operation: dns.RecordOperationOperationAdd,
				},
			},
		},
		RequestMetadata: common.RequestMetadata{RetryPolicy: &retryPolicy},
	}
	_, err := dnsClient.PatchZoneRecords(ctx, request)
	return err
}

func serveResponse(status int, message string, writer http.ResponseWriter) error {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	return json.NewEncoder(writer).Encode(response{Message: message})
}

func updateHandler(appConfig *appConfig, dnsClient *dns.DnsClient, w http.ResponseWriter, req *http.Request) error {
	if req.Method != "POST" {
		return serveResponse(http.StatusNotFound, "Not found", w)
	}

	token := req.URL.Query()["token"]
	if !slices.Contains(token, appConfig.Token) {
		return serveResponse(http.StatusForbidden, "Not authorized", w)
	}

	ipAddress, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		return serveResponse(http.StatusInternalServerError, err.Error(), w)
	}

	if err := updateDns(appConfig, dnsClient, req.Context(), ipAddress); err != nil {
		return serveResponse(http.StatusInternalServerError, err.Error(), w)
	}

	return serveResponse(http.StatusOK, fmt.Sprintf("Updated '%s' to '%s'", appConfig.Host, ipAddress), w)
}

func main() {
	configFileName := flag.String("config", "config.json", "Configuration file name")
	listenAddress := flag.String("listen", ":8080", "Address and port to listen to")
	flag.Parse()

	appConfig, err := loadAppConfig(configFileName)
	if err != nil {
		panic(err)
	}
	dnsClient, err := ociDNSClient(appConfig.OciConfig)
	if err != nil {
		panic(err)
	}

	handler := func(w http.ResponseWriter, r *http.Request) {
		_ = updateHandler(&appConfig, dnsClient, w, r)
	}
	http.HandleFunc("/update", handler)
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}
