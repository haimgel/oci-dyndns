package internal

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"strings"
)

type Service struct {
	appConfig *AppConfig
	logger    *slog.Logger
	dnsClient rrSetClient
}

type jsonResponse struct {
	Message string `json:"message"`
}

// result is a single reply to `/nic/update`. dyndns2 clients get `status` as a bare text/plain line: the protocol
// carries the update outcome in the body, since `good`/`nochg` have no equivalent in the HTTP status. It does not
// follow that everything must be HTTP 200 -- authentication and server failures do have a natural status, and
// ddclient reports those from the status line alone (see `header_ok`), so they keep 401/500. An empty `status`
// marks an outcome dyndns2 has no keyword for, and is served with `message` as the body.
//
// Callers that ask for JSON get `message` under the same `httpStatus`, the way this service replied before it
// spoke dyndns2.
type result struct {
	status     string
	message    string
	httpStatus int
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

// wantsJSON reports whether the caller explicitly asked for JSON. A wildcard `Accept` (what curl and ddclient send)
// does not count: dyndns2 is the default dialect of this endpoint.
func wantsJSON(req *http.Request) bool {
	for _, accept := range strings.Split(req.Header.Get("Accept"), ",") {
		if mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(accept)); err == nil {
			if mediaType == "application/json" {
				return true
			}
		}
	}
	return false
}

func (svc *Service) serveResponse(res result, writer http.ResponseWriter, req *http.Request) error {
	svc.logger.Info("Serving response",
		"status", res.status,
		"httpStatus", res.httpStatus,
		"message", res.message,
	)
	if wantsJSON(req) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(res.httpStatus)
		return json.NewEncoder(writer).Encode(jsonResponse{Message: res.message})
	}
	body := res.status
	if body == "" {
		body = res.message
	}
	writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	writer.WriteHeader(res.httpStatus)
	_, err := fmt.Fprintln(writer, body)
	return err
}

func remoteAddress(req *http.Request) (string, error) {
	fwdAddress := req.Header.Get("X-Forwarded-For")
	if fwdAddress != "" {
		// The header may list the whole proxy chain; the left-most entry is the original client.
		client, _, _ := strings.Cut(fwdAddress, ",")
		if client = strings.TrimSpace(client); client != "" {
			return client, nil
		}
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
	if req.Method != http.MethodGet && req.Method != http.MethodPost {
		return svc.serveResponse(result{message: "Not found", httpStatus: http.StatusNotFound}, w, req)
	}
	username, password, _ := req.BasicAuth()
	usernameMatch := subtle.ConstantTimeCompare([]byte(username), []byte(svc.appConfig.Username))
	passwordMatch := subtle.ConstantTimeCompare([]byte(password), []byte(svc.appConfig.Password))

	if usernameMatch != 1 || passwordMatch != 1 {
		// 401 rather than the 403 served before: the credentials are missing or wrong, not insufficient, and
		// RFC 9110 requires a challenge alongside it.
		w.Header().Set("WWW-Authenticate", `Basic realm="oci-dyndns", charset="UTF-8"`)
		return svc.serveResponse(result{
			status:     "badauth",
			message:    "Not authorized",
			httpStatus: http.StatusUnauthorized,
		}, w, req)
	}

	ipAddress, err := remoteAddress(req)
	if err != nil {
		return svc.serveResponse(svc.serverError(err), w, req)
	}

	changed, err := UpdateDns(req.Context(), svc.appConfig, svc.dnsClient, ipAddress, svc.logger)
	if err != nil {
		return svc.serveResponse(svc.serverError(err), w, req)
	}
	if !changed {
		return svc.serveResponse(result{
			status:     "nochg " + ipAddress,
			message:    fmt.Sprintf("Hosts %v already point to '%s'", svc.appConfig.Hosts, ipAddress),
			httpStatus: http.StatusOK,
		}, w, req)
	}
	return svc.serveResponse(result{
		status:     "good " + ipAddress,
		message:    fmt.Sprintf("Updated hosts %v to '%s'", svc.appConfig.Hosts, ipAddress),
		httpStatus: http.StatusOK,
	}, w, req)
}

// serverError builds the reply for a failure on our side: dyndns2's `911` ("retry later, do not reconfigure") in
// the body, over the HTTP 500 that says the same thing to everything else.
func (svc *Service) serverError(err error) result {
	return result{
		status:     "911",
		message:    err.Error(),
		httpStatus: http.StatusInternalServerError,
	}
}

func (svc *Service) Serve(listenAddress *string) error {
	svc.logger.Info("Server startup",
		"listen", listenAddress,
		"domain", svc.appConfig.Zone,
		"hosts", svc.appConfig.Hosts,
	)
	handler := func(w http.ResponseWriter, r *http.Request) {
		_ = svc.updateHandler(w, r)
	}
	http.HandleFunc("/nic/update", handler)
	return http.ListenAndServe(*listenAddress, nil)
}
