package packet

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestExtractReportAndFormats(t *testing.T) {
	path := writePacketFixture(t)
	result, err := ExtractReport(context.Background(), path)
	if err != nil {
		t.Fatalf("ExtractReport failed: %v", err)
	}
	if result.ReportType != "packet_extract_report" {
		t.Fatalf("report type = %q, want packet_extract_report", result.ReportType)
	}
	if result.PacketPath != path {
		t.Fatalf("packet path = %q, want %q", result.PacketPath, path)
	}
	if result.Extracted.LegalName != "Example Trucking LLC" || result.Extracted.USDOTNumber != "1234567" {
		t.Fatalf("unexpected extracted fields: %#v", result.Extracted)
	}
	if result.Extracted.Contact.Phone != "+15555555555" || result.Extracted.Contact.Email != "dispatch@exampletrucking.test" {
		t.Fatalf("unexpected contact fields: %#v", result.Extracted.Contact)
	}

	var jsonOut bytes.Buffer
	if err := WriteExtract(&jsonOut, result, "json"); err != nil {
		t.Fatalf("WriteExtract json failed: %v", err)
	}
	var decoded ExtractResult
	if err := json.Unmarshal(jsonOut.Bytes(), &decoded); err != nil {
		t.Fatalf("json output did not decode: %v\n%s", err, jsonOut.String())
	}
	if decoded.Extracted.LegalName != result.Extracted.LegalName {
		t.Fatalf("json extracted legal name = %q", decoded.Extracted.LegalName)
	}

	var markdownOut bytes.Buffer
	if err := WriteExtract(&markdownOut, result, "markdown"); err != nil {
		t.Fatalf("WriteExtract markdown failed: %v", err)
	}
	if !strings.Contains(markdownOut.String(), "# OpenHaul Guard Packet Extract") {
		t.Fatalf("markdown output missing heading: %s", markdownOut.String())
	}

	var tableOut bytes.Buffer
	if err := WriteExtract(&tableOut, result, "table"); err != nil {
		t.Fatalf("WriteExtract table failed: %v", err)
	}
	if !strings.Contains(tableOut.String(), "Legal name: Example Trucking LLC") {
		t.Fatalf("table output missing legal name: %s", tableOut.String())
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
