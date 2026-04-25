package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/openhaulguard/openhaulguard/internal/apperrors"
	"github.com/openhaulguard/openhaulguard/internal/domain"
)

func TestToolsList(t *testing.T) {
	in := requestBuffer(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
	})
	out := runServer(t, &fakeService{}, in)
	responses := responsesFrom(t, out)
	result := responses[0]["result"].(map[string]any)
	tools := result["tools"].([]any)
	if len(tools) != 4 {
		t.Fatalf("expected 4 tools, got %d", len(tools))
	}
	names := map[string]bool{}
	for _, raw := range tools {
		tool := raw.(map[string]any)
		names[tool["name"].(string)] = true
		if !strings.Contains(tool["description"].(string), "Do not use this tool to declare a carrier fraudulent") {
			t.Fatalf("tool %s is missing safety language", tool["name"])
		}
	}
	for _, name := range []string{"carrier_lookup", "carrier_diff", "packet_extract", "packet_check"} {
		if !names[name] {
			t.Fatalf("missing tool %s", name)
		}
	}
}

func TestToolCallCarrierLookup(t *testing.T) {
	service := &fakeService{
		lookupResult: domain.LookupResult{
			SchemaVersion: domain.SchemaVersion,
			ReportType:    "carrier_lookup_report",
			Lookup:        domain.LookupInfo{InputType: "dot", InputValue: "123456", Mode: "offline"},
			Carrier:       domain.CarrierProfile{USDOTNumber: "123456", LegalName: "Example Carrier"},
			Sources: []domain.SourceFetchResult{{
				SourceName:         "fmcsa_qcmobile",
				RequestURLRedacted: `https://mobile.fmcsa.dot.gov/qc/services/carriers/123456?webKey=SECRET`,
				ErrorMessage:       `Get "https://mobile.fmcsa.dot.gov/qc/services/carriers/123456?webKey=SECRET": dial tcp`,
			}},
			Disclaimer: domain.Disclaimer,
		},
	}
	in := requestBuffer(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      "lookup-1",
		"method":  "tools/call",
		"params": map[string]any{
			"name": "carrier_lookup",
			"arguments": map[string]any{
				"identifier_type":  "dot",
				"identifier_value": "123456",
				"force_refresh":    true,
				"offline":          true,
				"max_age":          "2h",
			},
		},
	})
	out := runServer(t, service, in)
	if service.lookupReq.IdentifierType != "dot" || service.lookupReq.IdentifierValue != "123456" {
		t.Fatalf("unexpected lookup request: %#v", service.lookupReq)
	}
	if !service.lookupReq.ForceRefresh || !service.lookupReq.Offline || service.lookupReq.MaxAge != 2*time.Hour {
		t.Fatalf("lookup options were not forwarded: %#v", service.lookupReq)
	}
	response := responsesFrom(t, out)[0]
	result := response["result"].(map[string]any)
	content := result["content"].([]any)[0].(map[string]any)
	if !strings.Contains(content["text"].(string), `"report_type": "carrier_lookup_report"`) {
		t.Fatalf("tool content did not contain lookup JSON: %s", content["text"])
	}
	if strings.Contains(out.String(), "SECRET") {
		t.Fatalf("lookup result exposed a secret: %s", out.String())
	}
	structured := result["structuredContent"].(map[string]any)
	if structured["report_type"] != "carrier_lookup_report" {
		t.Fatalf("unexpected structured content: %#v", structured)
	}
}

func TestToolCallCarrierDiff(t *testing.T) {
	service := &fakeService{
		diffResult: domain.DiffResult{
			SchemaVersion:    domain.SchemaVersion,
			ReportType:       "carrier_diff_report",
			IdentifierType:   "mc",
			IdentifierValue:  "123456",
			ObservationCount: 2,
			Changes: []domain.FieldDiff{{
				FieldPath:     "phone",
				PreviousValue: "555-0100",
				CurrentValue:  "555-0199",
				Material:      true,
			}},
		},
	}
	in := requestBuffer(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "carrier_diff",
			"arguments": map[string]any{
				"identifier_type":  "mc",
				"identifier_value": "123456",
				"since":            "30d",
				"strict":           true,
			},
		},
	})
	out := runServer(t, service, in)
	if service.diffType != "mc" || service.diffValue != "123456" || service.diffSince != "30d" || !service.diffStrict {
		t.Fatalf("diff arguments were not forwarded: %q %q %q %v", service.diffType, service.diffValue, service.diffSince, service.diffStrict)
	}
	response := responsesFrom(t, out)[0]
	result := response["result"].(map[string]any)
	structured := result["structuredContent"].(map[string]any)
	if structured["report_type"] != "carrier_diff_report" {
		t.Fatalf("unexpected structured content: %#v", structured)
	}
}

func TestToolCallPacketExtract(t *testing.T) {
	packetPath := writeMCPPacketFixture(t)
	in := requestBuffer(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      4,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "packet_extract",
			"arguments": map[string]any{
				"path": packetPath,
			},
		},
	})
	out := runServer(t, &fakeService{}, in)
	response := responsesFrom(t, out)[0]
	result := response["result"].(map[string]any)
	structured := result["structuredContent"].(map[string]any)
	if structured["report_type"] != "packet_extract_report" {
		t.Fatalf("unexpected structured content: %#v", structured)
	}
	extracted := structured["extracted"].(map[string]any)
	if extracted["legal_name"] != "Example Trucking LLC" || extracted["usdot_number"] != "1234567" {
		t.Fatalf("unexpected extracted fields: %#v", extracted)
	}
}

func TestToolCallPacketCheck(t *testing.T) {
	packetPath := writeMCPPacketFixture(t)
	service := &fakeService{
		lookupResult: domain.LookupResult{
			SchemaVersion: domain.SchemaVersion,
			ReportType:    "carrier_lookup_report",
			Lookup:        domain.LookupInfo{InputType: "mc", InputValue: "123456", ResolvedUSDOT: "1234567", Mode: "offline"},
			Carrier: domain.CarrierProfile{
				USDOTNumber: "1234567",
				LegalName:   "Example Trucking LLC",
				DBAName:     "Example Haul",
				Identifiers: []domain.Identifier{{Type: "MC", Value: "123456"}},
				PhysicalAddress: domain.Address{
					Line1:      "100 Main Street",
					City:       "Memphis",
					State:      "TN",
					PostalCode: "38103",
				},
				Contact: domain.Contact{Phone: "+15555555555", Email: "dispatch@exampletrucking.test"},
			},
			Disclaimer: domain.Disclaimer,
		},
	}
	in := requestBuffer(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      5,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "packet_check",
			"arguments": map[string]any{
				"path":             packetPath,
				"identifier_type":  "mc",
				"identifier_value": "123456",
				"offline":          true,
				"max_age":          "1h",
			},
		},
	})
	out := runServer(t, service, in)
	if service.lookupReq.IdentifierType != "mc" || service.lookupReq.IdentifierValue != "123456" {
		t.Fatalf("unexpected lookup request: %#v", service.lookupReq)
	}
	if !service.lookupReq.Offline || service.lookupReq.MaxAge != time.Hour {
		t.Fatalf("lookup options were not forwarded: %#v", service.lookupReq)
	}
	response := responsesFrom(t, out)[0]
	result := response["result"].(map[string]any)
	structured := result["structuredContent"].(map[string]any)
	if structured["report_type"] != "packet_check_report" {
		t.Fatalf("unexpected structured content: %#v", structured)
	}
	summary := structured["summary"].(map[string]any)
	if summary["recommendation"] != "packet_matches_lookup" {
		t.Fatalf("unexpected packet summary: %#v", summary)
	}
}

func TestToolErrorRedactsCause(t *testing.T) {
	service := &fakeService{
		lookupErr: apperrors.Wrap(
			apperrors.CodeSourceUnavailable,
			"source lookup failed",
			"Try again later",
			errors.New(`Get "https://mobile.fmcsa.dot.gov/qc/services/carriers/123?webKey=SECRET": dial tcp`),
		),
	}
	in := requestBuffer(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "carrier_lookup",
			"arguments": map[string]any{
				"identifier_type":  "dot",
				"identifier_value": "123",
			},
		},
	})
	out := runServer(t, service, in)
	if strings.Contains(out.String(), "SECRET") || strings.Contains(out.String(), "webKey") {
		t.Fatalf("tool error exposed a secret-bearing cause: %s", out.String())
	}
	response := responsesFrom(t, out)[0]
	result := response["result"].(map[string]any)
	if result["isError"] != true {
		t.Fatalf("expected tool error result, got %#v", result)
	}
}

func writeMCPPacketFixture(t *testing.T) string {
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

type fakeService struct {
	lookupReq    domain.LookupRequest
	lookupResult domain.LookupResult
	lookupErr    error

	diffType   string
	diffValue  string
	diffSince  string
	diffStrict bool
	diffResult domain.DiffResult
	diffErr    error
}

func (f *fakeService) Lookup(_ context.Context, req domain.LookupRequest) (domain.LookupResult, error) {
	f.lookupReq = req
	if f.lookupErr != nil {
		return domain.LookupResult{}, f.lookupErr
	}
	return f.lookupResult, nil
}

func (f *fakeService) Diff(_ context.Context, typ, value, since string, strict bool) (domain.DiffResult, error) {
	f.diffType = typ
	f.diffValue = value
	f.diffSince = since
	f.diffStrict = strict
	if f.diffErr != nil {
		return domain.DiffResult{}, f.diffErr
	}
	return f.diffResult, nil
}

func requestBuffer(t *testing.T, request map[string]any) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	if err := writeMessage(&buf, request); err != nil {
		t.Fatal(err)
	}
	return &buf
}

func runServer(t *testing.T, service Service, in *bytes.Buffer) *bytes.Buffer {
	t.Helper()
	var out bytes.Buffer
	if err := NewServer(service, in, &out).Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	return &out
}

func responsesFrom(t *testing.T, out *bytes.Buffer) []map[string]any {
	t.Helper()
	var responses []map[string]any
	scanner := bufio.NewScanner(bytes.NewReader(out.Bytes()))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var response map[string]any
		if err := json.Unmarshal([]byte(line), &response); err != nil {
			t.Fatalf("invalid response %q: %v", line, err)
		}
		responses = append(responses, response)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if len(responses) == 0 {
		t.Fatalf("expected at least one response, got %q", out.String())
	}
	return responses
}
