package internal

import (
	"encoding/json"
	"fmt"
	"github.com/oracle/oci-go-sdk/v65/dns"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"
	"net"
	"net/http"
)

type Service struct {
	appConfig *AppConfig
	logger    *zap.SugaredLogger
	dnsClient *dns.DnsClient
}

type response struct {
	Message string `json:"message"`
}

func NewService(appConfig *AppConfig, logger *zap.SugaredLogger) (*Service, error) {
	var err error
	svc := Service{
		appConfig: appConfig,
		logger:    logger,
	}
	svc.dnsClient, err = OciDNSClient(appConfig.OciConfig)
	return &svc, err
}

func (svc *Service) serveResponse(status int, message string, writer http.ResponseWriter) error {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	svc.logger.Infow("Serving response",
		"status", status,
		"message", message,
	)
	return json.NewEncoder(writer).Encode(response{Message: message})
}

func (svc *Service) updateHandler(w http.ResponseWriter, req *http.Request) error {
	svc.logger.Infow("Incoming request",
		"remoteAddr", req.RemoteAddr,
		"requestURI", req.RequestURI,
		"method", req.Method,
	)
	if req.Method != "POST" {
		return svc.serveResponse(http.StatusNotFound, "Not found", w)
	}

	token := req.URL.Query()["token"]
	if !slices.Contains(token, svc.appConfig.Token) {
		return svc.serveResponse(http.StatusForbidden, "Not authorized", w)
	}

	ipAddress, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		return svc.serveResponse(http.StatusInternalServerError, err.Error(), w)
	}

	if err := UpdateDns(svc.appConfig, svc.dnsClient, req.Context(), ipAddress); err != nil {
		return svc.serveResponse(http.StatusInternalServerError, err.Error(), w)
	}

	return svc.serveResponse(http.StatusOK, fmt.Sprintf("Updated '%s' to '%s'", svc.appConfig.Host, ipAddress), w)
}

func (svc *Service) Serve(listenAddress *string) error {
	svc.logger.Infow("Server startup",
		"listen", listenAddress,
		"domain", svc.appConfig.Zone,
		"hostname", svc.appConfig.Host,
	)
	handler := func(w http.ResponseWriter, r *http.Request) {
		_ = svc.updateHandler(w, r)
	}
	http.HandleFunc("/update", handler)
	return http.ListenAndServe(*listenAddress, nil)
}
