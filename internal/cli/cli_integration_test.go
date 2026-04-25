package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openhaulguard/openhaulguard/internal/app"
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
	reportOut, _, err := runCLI(t, "--home", home, "--format", "json", "watch", "report", "--since", "100000h", "--label", "primary")
	if err != nil {
		t.Fatalf("watch report failed: %v", err)
	}
	var watchReport struct {
		Total   int `json:"total"`
		Changed int `json:"changed"`
		Items   []struct {
			IdentifierType  string             `json:"identifier_type"`
			IdentifierValue string             `json:"identifier_value"`
			Label           string             `json:"label"`
			Status          string             `json:"status"`
			Changes         []domain.FieldDiff `json:"changes"`
		} `json:"items"`
	}
	decodeJSON(t, reportOut, &watchReport)
	if watchReport.Total != 1 || watchReport.Changed != 1 || len(watchReport.Items) != 1 {
		t.Fatalf("watch report summary = %#v", watchReport)
	}
	if watchReport.Items[0].IdentifierType != "mc" || watchReport.Items[0].IdentifierValue != "123456" || watchReport.Items[0].Label != "primary lane" || watchReport.Items[0].Status != "changed" {
		t.Fatalf("watch report item = %#v", watchReport.Items[0])
	}
	if !hasPhoneDiff(watchReport.Items[0].Changes) {
		t.Fatalf("watch report changes did not include expected phone change: %#v", watchReport.Items[0].Changes)
	}
	exportOut, _, err := runCLI(t, "--home", home, "--format", "json", "watch", "export")
	if err != nil {
		t.Fatalf("watch export failed: %v", err)
	}
	var watchExport app.WatchExportResult
	decodeJSON(t, exportOut, &watchExport)
	if watchExport.Total != 1 || len(watchExport.Items) != 1 {
		t.Fatalf("watch export summary = %#v", watchExport)
	}
	if watchExport.Items[0].IdentifierType != "mc" || watchExport.Items[0].IdentifierValue != "123456" || watchExport.Items[0].Label != "primary lane" {
		t.Fatalf("watch export item = %#v", watchExport.Items[0])
	}
	removeOut, _, err := runCLI(t, "--home", home, "watch", "remove", "--mc", "123456")
	if err != nil {
		t.Fatalf("watch remove failed: %v", err)
	}
	if !strings.Contains(removeOut, "Removed MC 123456") {
		t.Fatalf("watch remove output = %q", removeOut)
	}
	listAfterRemoveOut, _, err := runCLI(t, "--home", home, "--format", "json", "watch", "list")
	if err != nil {
		t.Fatalf("watch list after remove failed: %v", err)
	}
	watched = nil
	decodeJSON(t, listAfterRemoveOut, &watched)
	if len(watched) != 0 {
		t.Fatalf("watch list after remove = %#v", watched)
	}

	extractOut, _, err := runCLI(t, "--home", home, "--format", "json", "packet", "extract", packetFixture)
	if err != nil {
		t.Fatalf("packet extract failed: %v", err)
	}
	var extractResult packet.ExtractResult
	decodeJSON(t, extractOut, &extractResult)
	if extractResult.ReportType != "packet_extract_report" {
		t.Fatalf("packet extract report type = %q", extractResult.ReportType)
	}
	if extractResult.Extracted.LegalName != "Example Trucking LLC" || extractResult.Extracted.USDOTNumber != "1234567" {
		t.Fatalf("unexpected packet extract fields: %#v", extractResult.Extracted)
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

func TestSetupDefaultAndInitAlias(t *testing.T) {
	home := t.TempDir()

	setupOut, _, err := runCLI(t, "--home", home, "setup")
	if err != nil {
		t.Fatalf("default setup failed: %v", err)
	}
	if !strings.Contains(setupOut, "private OpenHaul Guard folder") {
		t.Fatalf("setup output did not include guided copy: %q", setupOut)
	}
	if !strings.Contains(setupOut, "ohg setup fmcsa") {
		t.Fatalf("setup output did not include live setup next step: %q", setupOut)
	}

	resumeOut, _, err := runCLI(t, "--home", home, "setup")
	if err != nil {
		t.Fatalf("resumed setup failed: %v", err)
	}
	if !strings.Contains(resumeOut, "Found earlier setup progress") {
		t.Fatalf("setup did not report resumable progress: %q", resumeOut)
	}

	initHome := t.TempDir()
	initOut, _, err := runCLI(t, "--home", initHome, "--format", "json", "init")
	if err != nil {
		t.Fatalf("init alias failed: %v", err)
	}
	var initResult map[string]any
	decodeJSON(t, initOut, &initResult)
	if initResult["status"] != "ok" || initResult["command"] != "init" {
		t.Fatalf("init result = %#v, want ok init", initResult)
	}
	progress, ok := initResult["progress"].(map[string]any)
	if !ok || progress["quick_setup_complete"] != true {
		t.Fatalf("init progress = %#v, want quick_setup_complete", initResult["progress"])
	}
}

func TestMirrorImportAndLookup(t *testing.T) {
	home := t.TempDir()
	mirrorFixture := fixturePath(t, "mirror", "carriers.json")
	censusFixture := fixturePath(t, "socrata", "company_census_rows.json")
	builtMirror := filepath.Join(home, "built-mirror.json")

	if _, _, err := runCLI(t, "--home", home, "--format", "json", "setup", "--quick"); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	buildOut, _, err := runCLI(t, "--home", home, "--format", "json", "mirror", "build", censusFixture, "--output", builtMirror, "--generated-at", "2026-04-25T12:00:00Z", "--source-timestamp", "2026-04-24")
	if err != nil {
		t.Fatalf("mirror build failed: %v", err)
	}
	var buildStatus struct {
		Path         string `json:"path"`
		CarrierCount int    `json:"carrier_count"`
		GeneratedAt  string `json:"generated_at"`
	}
	decodeJSON(t, buildOut, &buildStatus)
	if buildStatus.Path != builtMirror || buildStatus.CarrierCount != 2 || buildStatus.GeneratedAt != "2026-04-25T12:00:00Z" {
		t.Fatalf("mirror build status = %#v", buildStatus)
	}
	importOut, _, err := runCLI(t, "--home", home, "--format", "json", "mirror", "import", mirrorFixture)
	if err != nil {
		t.Fatalf("mirror import failed: %v", err)
	}
	var status struct {
		Available    bool `json:"available"`
		CarrierCount int  `json:"carrier_count"`
	}
	decodeJSON(t, importOut, &status)
	if !status.Available || status.CarrierCount != 1 {
		t.Fatalf("mirror status after import = %#v", status)
	}
	lookupOut, _, err := runCLI(t, "--home", home, "--format", "json", "carrier", "lookup", "--mc", "123456")
	if err != nil {
		t.Fatalf("mirror lookup failed: %v", err)
	}
	var lookup domain.LookupResult
	decodeJSON(t, lookupOut, &lookup)
	if lookup.Lookup.Mode != "mirror" {
		t.Fatalf("lookup mode = %q, want mirror", lookup.Lookup.Mode)
	}
	if lookup.Carrier.USDOTNumber != "1234567" {
		t.Fatalf("lookup USDOT = %q, want 1234567", lookup.Carrier.USDOTNumber)
	}
	if len(lookup.Warnings) == 0 || lookup.Warnings[0].Code != "OHG_MIRROR_MODE" {
		t.Fatalf("lookup warnings = %#v, want mirror warning", lookup.Warnings)
	}
}

func TestServeRefusesNonLoopbackWithoutToken(t *testing.T) {
	_, _, err := runCLI(t, "--home", t.TempDir(), "serve", "--listen", "0.0.0.0:8787")
	if err == nil {
		t.Fatal("serve without token on non-loopback address succeeded, want error")
	}
	if !strings.Contains(err.Error(), "refusing to serve without an API token") {
		t.Fatalf("serve error = %v", err)
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
