package internal

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/oracle/oci-go-sdk/v65/dns"
)

// fakeDnsClient records the calls made against it and replays canned RRSet contents.
type fakeDnsClient struct {
	records   []dns.Record
	getErr    error
	updateErr error
	gets      int
	updates   int
}

func (c *fakeDnsClient) GetRRSet(_ context.Context, _ dns.GetRRSetRequest) (dns.GetRRSetResponse, error) {
	c.gets++
	if c.getErr != nil {
		return dns.GetRRSetResponse{}, c.getErr
	}
	return dns.GetRRSetResponse{RrSet: dns.RrSet{Items: c.records}}, nil
}

func (c *fakeDnsClient) UpdateRRSet(_ context.Context, _ dns.UpdateRRSetRequest) (dns.UpdateRRSetResponse, error) {
	c.updates++
	return dns.UpdateRRSetResponse{}, c.updateErr
}

func aRecord(ipAddress string, ttl int) dns.Record {
	rtype := recordType
	return dns.Record{Rtype: &rtype, Rdata: &ipAddress, Ttl: &ttl}
}

func testService(client rrSetClient) *Service {
	return &Service{
		appConfig: &AppConfig{
			Zone:     "example.com",
			Hosts:    []string{"home.example.com"},
			Username: "user",
			Password: "pass",
		},
		logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		dnsClient: client,
	}
}

// call issues an authenticated update request from 203.0.113.7 and returns the recorded response.
func call(svc *Service, accept string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/nic/update", nil)
	req.RemoteAddr = "203.0.113.7:54321"
	req.SetBasicAuth("user", "pass")
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	writer := httptest.NewRecorder()
	_ = svc.updateHandler(writer, req)
	return writer
}

func checkPlainText(t *testing.T, writer *httptest.ResponseRecorder, wantStatus int, wantBody string) {
	t.Helper()
	if writer.Code != wantStatus {
		t.Errorf("HTTP status = %d, want %d", writer.Code, wantStatus)
	}
	if contentType := writer.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain", contentType)
	}
	if got := writer.Body.String(); got != wantBody+"\n" {
		t.Errorf("body = %q, want %q", got, wantBody+"\n")
	}
}

func TestUpdateGoodResponse(t *testing.T) {
	client := &fakeDnsClient{records: []dns.Record{aRecord("198.51.100.1", recordTtl)}}
	checkPlainText(t, call(testService(client), ""), http.StatusOK, "good 203.0.113.7")
	if client.updates != 1 {
		t.Errorf("UpdateRRSet calls = %d, want 1", client.updates)
	}
}

func TestUpdateNoChangeSkipsWrite(t *testing.T) {
	client := &fakeDnsClient{records: []dns.Record{aRecord("203.0.113.7", recordTtl)}}
	checkPlainText(t, call(testService(client), ""), http.StatusOK, "nochg 203.0.113.7")
	if client.updates != 0 {
		t.Errorf("UpdateRRSet calls = %d, want 0: an already-correct record must not be rewritten", client.updates)
	}
}

func TestUpdateWrongTtlIsRewritten(t *testing.T) {
	client := &fakeDnsClient{records: []dns.Record{aRecord("203.0.113.7", recordTtl*10)}}
	checkPlainText(t, call(testService(client), ""), http.StatusOK, "good 203.0.113.7")
	if client.updates != 1 {
		t.Errorf("UpdateRRSet calls = %d, want 1", client.updates)
	}
}

func TestUpdateMissingRecordIsCreated(t *testing.T) {
	client := &fakeDnsClient{}
	checkPlainText(t, call(testService(client), ""), http.StatusOK, "good 203.0.113.7")
	if client.updates != 1 {
		t.Errorf("UpdateRRSet calls = %d, want 1", client.updates)
	}
}

func TestUpdateFailureIs911(t *testing.T) {
	client := &fakeDnsClient{updateErr: errors.New("OCI is unavailable")}
	checkPlainText(t, call(testService(client), ""), http.StatusInternalServerError, "911")
}

func TestUnreadableRRSetStillUpdates(t *testing.T) {
	client := &fakeDnsClient{getErr: errors.New("no read permission")}
	checkPlainText(t, call(testService(client), ""), http.StatusOK, "good 203.0.113.7")
	if client.updates != 1 {
		t.Errorf("UpdateRRSet calls = %d, want 1", client.updates)
	}
}

func TestBadAuth(t *testing.T) {
	client := &fakeDnsClient{}
	req := httptest.NewRequest(http.MethodGet, "/nic/update", nil)
	req.RemoteAddr = "203.0.113.7:54321"
	req.SetBasicAuth("user", "wrong")
	writer := httptest.NewRecorder()
	_ = testService(client).updateHandler(writer, req)

	// A wrong password is a genuine HTTP 401, not a 200 with a sad body: ddclient reports a non-2xx from the
	// status line alone, and everything else (monitoring, `curl -f`, browsers) depends on it too.
	checkPlainText(t, writer, http.StatusUnauthorized, "badauth")
	if challenge := writer.Header().Get("WWW-Authenticate"); !strings.HasPrefix(challenge, "Basic ") {
		t.Errorf("WWW-Authenticate = %q, want a Basic challenge", challenge)
	}
	if client.updates != 0 {
		t.Errorf("UpdateRRSet calls = %d, want 0", client.updates)
	}
}

func TestForwardedForTakesFirstEntry(t *testing.T) {
	client := &fakeDnsClient{}
	req := httptest.NewRequest(http.MethodGet, "/nic/update", nil)
	req.RemoteAddr = "10.0.0.1:54321"
	req.SetBasicAuth("user", "pass")
	req.Header.Set("X-Forwarded-For", "203.0.113.7, 10.0.0.2")
	writer := httptest.NewRecorder()
	_ = testService(client).updateHandler(writer, req)

	checkPlainText(t, writer, http.StatusOK, "good 203.0.113.7")
}

func TestJsonResponseOnAcceptHeader(t *testing.T) {
	client := &fakeDnsClient{records: []dns.Record{aRecord("203.0.113.7", recordTtl)}}
	writer := call(testService(client), "application/json")

	if writer.Code != http.StatusOK {
		t.Errorf("HTTP status = %d, want %d", writer.Code, http.StatusOK)
	}
	if contentType := writer.Header().Get("Content-Type"); contentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", contentType)
	}
	var body jsonResponse
	if err := json.Unmarshal(writer.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if !strings.Contains(body.Message, "203.0.113.7") {
		t.Errorf("message = %q, want it to mention the IP address", body.Message)
	}
}

func TestJsonAuthFailureKeepsHttpStatus(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/nic/update", nil)
	req.RemoteAddr = "203.0.113.7:54321"
	req.SetBasicAuth("user", "wrong")
	req.Header.Set("Accept", "application/json")
	writer := httptest.NewRecorder()
	_ = testService(&fakeDnsClient{}).updateHandler(writer, req)

	if writer.Code != http.StatusUnauthorized {
		t.Errorf("HTTP status = %d, want %d", writer.Code, http.StatusUnauthorized)
	}
}

func TestJsonErrorKeepsHttpStatus(t *testing.T) {
	client := &fakeDnsClient{updateErr: errors.New("OCI is unavailable")}
	writer := call(testService(client), "application/json")

	if writer.Code != http.StatusInternalServerError {
		t.Errorf("HTTP status = %d, want %d", writer.Code, http.StatusInternalServerError)
	}
}

func TestWildcardAcceptIsPlainText(t *testing.T) {
	// curl and ddclient both send `*/*`, and must get the dyndns2 dialect.
	checkPlainText(t, call(testService(&fakeDnsClient{}), "*/*"), http.StatusOK, "good 203.0.113.7")
}
