package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openhaulguard/openhaulguard/internal/domain"
	"github.com/openhaulguard/openhaulguard/internal/packet"
)

func TestCLIIntegrationLocalFlows(t *testing.T) {
	home := t.TempDir()
	validFixture := fixturePath(t, "fmcsa_qcmobile", "fmcsa_qcmobile_carrier_valid.json")
	changedFixture := fixturePath(t, "fmcsa_qcmobile", "fmcsa_qcmobile_carrier_changed_phone.json")
	packetFixture := fixturePath(t, "packets", "basic_carrier_packet.txt")

	setupOut, _, err := runCLI(t, "--home", home, "--format", "json", "setup", "--quick")
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	var setupResult map[string]any
	decodeJSON(t, setupOut, &setupResult)
	if setupResult["status"] != "ok" {
		t.Fatalf("setup status = %v, want ok", setupResult["status"])
	}

	lookupOut, _, err := runCLI(t, "--home", home, "--format", "json", "carrier", "lookup", "--mc", "123456", "--fixture", validFixture)
	if err != nil {
		t.Fatalf("fixture lookup failed: %v", err)
	}
	var lookup domain.LookupResult
	decodeJSON(t, lookupOut, &lookup)
	if lookup.Lookup.Mode != "fixture" {
		t.Fatalf("lookup mode = %q, want fixture", lookup.Lookup.Mode)
	}
	if lookup.Carrier.USDOTNumber != "1234567" {
		t.Fatalf("lookup USDOT = %q, want 1234567", lookup.Carrier.USDOTNumber)
	}

	changedOut, _, err := runCLI(t, "--home", home, "--format", "json", "carrier", "lookup", "--mc", "123456", "--fixture", changedFixture)
	if err != nil {
		t.Fatalf("changed fixture lookup failed: %v", err)
	}
	var changed domain.LookupResult
	decodeJSON(t, changedOut, &changed)
	if changed.Carrier.Contact.Phone != "+15555010123" {
		t.Fatalf("changed phone = %q, want +15555010123", changed.Carrier.Contact.Phone)
	}

	diffOut, _, err := runCLI(t, "--home", home, "--format", "json", "carrier", "diff", "--mc", "123456", "--since", "100000h")
	if err != nil {
		t.Fatalf("diff failed: %v", err)
	}
	var diff domain.DiffResult
	decodeJSON(t, diffOut, &diff)
	if !hasPhoneDiff(diff.Changes) {
		t.Fatalf("diff changes did not include expected phone change: %#v", diff.Changes)
	}

	watchOut, _, err := runCLI(t, "--home", home, "watch", "add", "--mc", "123456", "--label", "primary lane")
	if err != nil {
		t.Fatalf("watch add failed: %v", err)
	}
	if !strings.Contains(watchOut, "Added MC 123456") {
		t.Fatalf("watch add output = %q", watchOut)
	}
	listOut, _, err := runCLI(t, "--home", home, "--format", "json", "watch", "list")
	if err != nil {
		t.Fatalf("watch list failed: %v", err)
	}
	var watched []struct {
		IdentifierType  string `json:"identifier_type"`
		IdentifierValue string `json:"identifier_value"`
		Label           string `json:"label"`
	}
	decodeJSON(t, listOut, &watched)
	if len(watched) != 1 || watched[0].IdentifierType != "mc" || watched[0].IdentifierValue != "123456" || watched[0].Label != "primary lane" {
		t.Fatalf("watch list = %#v", watched)
	}

	packetOut, _, err := runCLI(t, "--home", home, "--format", "json", "packet", "check", packetFixture, "--mc", "123456", "--fixture", validFixture)
	if err != nil {
		t.Fatalf("packet check failed: %v", err)
	}
	var packetResult packet.CheckResult
	decodeJSON(t, packetOut, &packetResult)
	if packetResult.Summary.Recommendation != "packet_matches_lookup" {
		t.Fatalf("packet recommendation = %q, want packet_matches_lookup", packetResult.Summary.Recommendation)
	}
	if packetResult.Summary.Mismatches != 0 || packetResult.Summary.MissingPacket != 0 || packetResult.Summary.MissingSource != 0 {
		t.Fatalf("packet summary = %#v", packetResult.Summary)
	}
	if packetResult.Extracted.LegalName != "Example Trucking LLC" || packetResult.Extracted.USDOTNumber != "1234567" {
		t.Fatalf("unexpected extracted packet fields: %#v", packetResult.Extracted)
	}
}

func runCLI(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	g := &globals{}
	cmd := rootCommand(g)
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetIn(strings.NewReader(""))
	cmd.SetArgs(args)
	err := cmd.Execute()
	return stdout.String(), stderr.String(), err
}

func fixturePath(t *testing.T, parts ...string) string {
	t.Helper()
	pathParts := append([]string{"..", "..", "examples", "fixtures"}, parts...)
	path, err := filepath.Abs(filepath.Join(pathParts...))
	if err != nil {
		t.Fatalf("fixture path: %v", err)
	}
	return path
}

func decodeJSON(t *testing.T, body string, target any) {
	t.Helper()
	if err := json.Unmarshal([]byte(body), target); err != nil {
		t.Fatalf("decode JSON %q: %v", body, err)
	}
}

func hasPhoneDiff(changes []domain.FieldDiff) bool {
	for _, change := range changes {
		if change.FieldPath == "phone" && change.PreviousValue == "+15555555555" && change.CurrentValue == "+15555010123" {
			return true
		}
	}
	return false
}
