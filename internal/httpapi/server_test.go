package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/openhaulguard/openhaulguard/internal/app"
	"github.com/openhaulguard/openhaulguard/internal/apperrors"
	"github.com/openhaulguard/openhaulguard/internal/domain"
)

func TestHealth(t *testing.T) {
	srv := NewServer(&fakeService{})
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rr.Code, rr.Body.String())
	}
	var body map[string]any
	decodeHTTPJSON(t, rr.Body.Bytes(), &body)
	if body["status"] != "ok" || body["service"] != "openhaulguard_http" || body["auth_required"] != false {
		t.Fatalf("health body = %#v", body)
	}
}

func TestAuthRequiredForV1WhenTokenConfigured(t *testing.T) {
	srv := NewServer(&fakeService{}, WithToken("secret"))

	unauthorized := httptest.NewRecorder()
	srv.Handler().ServeHTTP(unauthorized, jsonRequest(http.MethodPost, "/v1/carrier/lookup", map[string]any{
		"identifier_type":  "mc",
		"identifier_value": "123456",
	}))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d, body %s", unauthorized.Code, unauthorized.Body.String())
	}

	authorizedReq := jsonRequest(http.MethodPost, "/v1/carrier/lookup", map[string]any{
		"identifier_type":  "mc",
		"identifier_value": "123456",
	})
	authorizedReq.Header.Set("Authorization", "Bearer secret")
	authorized := httptest.NewRecorder()
	srv.Handler().ServeHTTP(authorized, authorizedReq)
	if authorized.Code != http.StatusOK {
		t.Fatalf("authorized status = %d, body %s", authorized.Code, authorized.Body.String())
	}
}

func TestCarrierLookupForwardsOptionsAndRedactsSecrets(t *testing.T) {
	service := &fakeService{
		lookupResult: domain.LookupResult{
			SchemaVersion: domain.SchemaVersion,
			ReportType:    "carrier_lookup_report",
			Lookup:        domain.LookupInfo{InputType: "mc", InputValue: "123456", ResolvedUSDOT: "1234567", Mode: "live"},
			Carrier:       domain.CarrierProfile{USDOTNumber: "1234567", LegalName: "Example Trucking LLC"},
			Sources: []domain.SourceFetchResult{{
				SourceName:         "fmcsa",
				Endpoint:           "/carriers?webKey=supersecret",
				RequestURLRedacted: "https://example.test?webKey=supersecret",
			}},
			Disclaimer: domain.Disclaimer,
		},
	}
	srv := NewServer(service, WithDefaultOffline(true))
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, jsonRequest(http.MethodPost, "/v1/carrier/lookup", map[string]any{
		"identifier_type":  "mc",
		"identifier_value": "123456",
		"force_refresh":    true,
		"offline":          false,
		"max_age":          "2h",
	}))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rr.Code, rr.Body.String())
	}
	if service.lookupReq.IdentifierType != "mc" || service.lookupReq.IdentifierValue != "123456" || !service.lookupReq.ForceRefresh || service.lookupReq.Offline || service.lookupReq.MaxAge != 2*time.Hour {
		t.Fatalf("lookup request = %#v", service.lookupReq)
	}
	var result domain.LookupResult
	decodeHTTPJSON(t, rr.Body.Bytes(), &result)
	if strings.Contains(result.Sources[0].Endpoint, "supersecret") || strings.Contains(result.Sources[0].RequestURLRedacted, "supersecret") {
		t.Fatalf("source secrets were not redacted: %#v", result.Sources[0])
	}
}

func TestCarrierDiffAndWatchExport(t *testing.T) {
	service := &fakeService{
		diffResult: domain.DiffResult{
			SchemaVersion:    domain.SchemaVersion,
			ReportType:       "carrier_diff_report",
			IdentifierType:   "mc",
			IdentifierValue:  "123456",
			ObservationCount: 2,
		},
		watchExport: app.WatchExportResult{
			SchemaVersion: domain.SchemaVersion,
			ReportType:    "watchlist_export_report",
			Total:         1,
			Items: []app.WatchExportItem{{
				IdentifierType:  "mc",
				IdentifierValue: "123456",
				NormalizedValue: "123456",
				Active:          true,
			}},
		},
	}
	srv := NewServer(service)

	diffRR := httptest.NewRecorder()
	srv.Handler().ServeHTTP(diffRR, jsonRequest(http.MethodPost, "/v1/carrier/diff", map[string]any{
		"identifier_type":  "mc",
		"identifier_value": "123456",
		"since":            "30d",
		"strict":           true,
	}))
	if diffRR.Code != http.StatusOK {
		t.Fatalf("diff status = %d, body %s", diffRR.Code, diffRR.Body.String())
	}
	if service.diffTyp != "mc" || service.diffValue != "123456" || service.diffSince != "30d" || !service.diffStrict {
		t.Fatalf("diff args = %q %q %q %t", service.diffTyp, service.diffValue, service.diffSince, service.diffStrict)
	}

	exportRR := httptest.NewRecorder()
	srv.Handler().ServeHTTP(exportRR, httptest.NewRequest(http.MethodGet, "/v1/watch/export", nil))
	if exportRR.Code != http.StatusOK {
		t.Fatalf("export status = %d, body %s", exportRR.Code, exportRR.Body.String())
	}
	var export app.WatchExportResult
	decodeHTTPJSON(t, exportRR.Body.Bytes(), &export)
	if export.Total != 1 || export.Items[0].IdentifierType != "mc" {
		t.Fatalf("export = %#v", export)
	}
}

func TestPacketExtractAndCheck(t *testing.T) {
	packetPath := writePacketFixture(t)
	service := &fakeService{
		lookupResult: domain.LookupResult{
			SchemaVersion: domain.SchemaVersion,
			ReportType:    "carrier_lookup_report",
			Lookup:        domain.LookupInfo{InputType: "mc", InputValue: "123456", ResolvedUSDOT: "1234567", Mode: "cache"},
			Carrier: domain.CarrierProfile{
				USDOTNumber:       "1234567",
				LegalName:         "Example Trucking LLC",
				DBAName:           "Example Haul",
				Identifiers:       []domain.Identifier{{Type: "MC", Value: "123456"}},
				PhysicalAddress:   domain.Address{Line1: "100 Main Street", City: "Memphis", State: "TN", PostalCode: "38103"},
				Contact:           domain.Contact{Phone: "+15555555555", Email: "dispatch@exampletrucking.test"},
				LocalLastSeenAt:   "2026-04-25T00:00:00Z",
				LocalFirstSeenAt:  "2026-04-25T00:00:00Z",
				SourceFirstSeenAt: "2026-04-25T00:00:00Z",
			},
			Disclaimer: domain.Disclaimer,
		},
	}
	srv := NewServer(service)

	extractRR := httptest.NewRecorder()
	srv.Handler().ServeHTTP(extractRR, jsonRequest(http.MethodPost, "/v1/packet/extract", map[string]any{"path": packetPath}))
	if extractRR.Code != http.StatusOK {
		t.Fatalf("extract status = %d, body %s", extractRR.Code, extractRR.Body.String())
	}
	var extract struct {
		ReportType string `json:"report_type"`
		Extracted  struct {
			USDOTNumber string `json:"usdot_number"`
		} `json:"extracted"`
	}
	decodeHTTPJSON(t, extractRR.Body.Bytes(), &extract)
	if extract.ReportType != "packet_extract_report" || extract.Extracted.USDOTNumber != "1234567" {
		t.Fatalf("extract = %#v", extract)
	}

	checkRR := httptest.NewRecorder()
	srv.Handler().ServeHTTP(checkRR, jsonRequest(http.MethodPost, "/v1/packet/check", map[string]any{
		"path":             packetPath,
		"identifier_type":  "mc",
		"identifier_value": "123456",
		"offline":          true,
	}))
	if checkRR.Code != http.StatusOK {
		t.Fatalf("check status = %d, body %s", checkRR.Code, checkRR.Body.String())
	}
	if !service.lookupReq.Offline {
		t.Fatalf("packet check did not forward offline lookup option: %#v", service.lookupReq)
	}
	var check struct {
		ReportType string `json:"report_type"`
		Summary    struct {
			Recommendation string `json:"recommendation"`
		} `json:"summary"`
	}
	decodeHTTPJSON(t, checkRR.Body.Bytes(), &check)
	if check.ReportType != "packet_check_report" || check.Summary.Recommendation != "packet_matches_lookup" {
		t.Fatalf("check = %#v", check)
	}
}

func TestErrorsAndListenValidation(t *testing.T) {
	service := &fakeService{
		lookupErr: apperrors.New(apperrors.CodeSourceNotFound, "carrier was not found", ""),
	}
	srv := NewServer(service)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, jsonRequest(http.MethodPost, "/v1/carrier/lookup", map[string]any{
		"identifier_type":  "mc",
		"identifier_value": "123456",
	}))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body %s", rr.Code, rr.Body.String())
	}
	var payload struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	decodeHTTPJSON(t, rr.Body.Bytes(), &payload)
	if payload.Error.Code != apperrors.CodeSourceNotFound || payload.Error.Message != "carrier was not found" {
		t.Fatalf("error payload = %#v", payload)
	}

	if err := ValidateListen("127.0.0.1:8787", false); err != nil {
		t.Fatalf("loopback listen should be allowed: %v", err)
	}
	if err := ValidateListen("0.0.0.0:8787", false); err == nil {
		t.Fatal("non-loopback listen without auth should fail")
	}
	if err := ValidateListen("0.0.0.0:8787", true); err != nil {
		t.Fatalf("non-loopback listen with auth should be allowed: %v", err)
	}
}

type fakeService struct {
	lookupReq    domain.LookupRequest
	lookupResult domain.LookupResult
	lookupErr    error
	diffTyp      string
	diffValue    string
	diffSince    string
	diffStrict   bool
	diffResult   domain.DiffResult
	diffErr      error
	watchExport  app.WatchExportResult
	watchErr     error
}

func (f *fakeService) Lookup(_ context.Context, req domain.LookupRequest) (domain.LookupResult, error) {
	f.lookupReq = req
	if f.lookupErr != nil {
		return domain.LookupResult{}, f.lookupErr
	}
	return f.lookupResult, nil
}

func (f *fakeService) Diff(_ context.Context, typ, value, since string, strict bool) (domain.DiffResult, error) {
	f.diffTyp = typ
	f.diffValue = value
	f.diffSince = since
	f.diffStrict = strict
	if f.diffErr != nil {
		return domain.DiffResult{}, f.diffErr
	}
	return f.diffResult, nil
}

func (f *fakeService) WatchExport(context.Context) (app.WatchExportResult, error) {
	if f.watchErr != nil {
		return app.WatchExportResult{}, f.watchErr
	}
	return f.watchExport, nil
}

func jsonRequest(method, path string, body any) *http.Request {
	payload, _ := json.Marshal(body)
	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func decodeHTTPJSON(t *testing.T, body []byte, target any) {
	t.Helper()
	if err := json.Unmarshal(body, target); err != nil {
		t.Fatalf("decode JSON %s: %v", string(body), err)
	}
}

func writePacketFixture(t *testing.T) string {
	t.Helper()
	path := t.TempDir() + "/packet.txt"
	body := `Carrier Packet

Legal Name: Example Trucking LLC
DBA: Example Haul
USDOT: 1234567
MC: 123456
Address: 100 Main Street, Memphis, TN 38103
Phone: (555) 555-5555
Email: dispatch@exampletrucking.test
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
