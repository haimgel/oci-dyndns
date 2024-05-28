package internal

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"github.com/oracle/oci-go-sdk/v65/dns"
	"log/slog"
	"net"
	"net/http"
)

type Service struct {
	appConfig *AppConfig
	logger    *slog.Logger
	dnsClient *dns.DnsClient
}

type response struct {
	Message string `json:"message"`
}

func NewService(appConfig *AppConfig, logger *slog.Logger) (*Service, error) {
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
	svc.logger.Info("Serving response",
		"status", status,
		"message", message,
	)
	return json.NewEncoder(writer).Encode(response{Message: message})
}

func remoteAddress(req *http.Request) (string, error) {
	fwdAddress := req.Header.Get("X-Forwarded-For")
	if fwdAddress != "" {
		return fwdAddress, nil
	}
	directAddress, _, err := net.SplitHostPort(req.RemoteAddr)
	if directAddress != "" {
		return directAddress, nil
	}
	return "", err
}

func (svc *Service) updateHandler(w http.ResponseWriter, req *http.Request) error {
	svc.logger.Info("Incoming request",
		"remoteAddr", req.RemoteAddr,
		"requestURI", req.RequestURI,
		"method", req.Method,
	)
	// Protocol: dyndns2 api
	if req.Method != "GET" && req.Method != "POST" {
		return svc.serveResponse(http.StatusNotFound, "Not found", w)
	}

	username := req.URL.User.Username()
	password, _ := req.URL.User.Password()
	usernameMatch := subtle.ConstantTimeCompare([]byte(username), []byte(svc.appConfig.Username))
	passwordMatch := subtle.ConstantTimeCompare([]byte(password), []byte(svc.appConfig.Password))

	if usernameMatch != 1 || passwordMatch != 1 {
		return svc.serveResponse(http.StatusForbidden, "Not authorized", w)
	}

	ipAddress, err := remoteAddress(req)
	if err != nil {
		return svc.serveResponse(http.StatusInternalServerError, err.Error(), w)
	}

	if err := UpdateDns(svc.appConfig, svc.dnsClient, req.Context(), ipAddress); err != nil {
		return svc.serveResponse(http.StatusInternalServerError, err.Error(), w)
	}

	return svc.serveResponse(http.StatusOK, fmt.Sprintf("Updated '%s' to '%s'", svc.appConfig.Host, ipAddress), w)
}

func (svc *Service) Serve(listenAddress *string) error {
	svc.logger.Info("Server startup",
		"listen", listenAddress,
		"domain", svc.appConfig.Zone,
		"hostname", svc.appConfig.Host,
	)
	handler := func(w http.ResponseWriter, r *http.Request) {
		_ = svc.updateHandler(w, r)
	}
	http.HandleFunc("/nic/update", handler)
	return http.ListenAndServe(*listenAddress, nil)
}
