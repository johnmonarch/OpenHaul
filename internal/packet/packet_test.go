package packet

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/openhaulguard/openhaulguard/internal/domain"
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
	if result.Extracted.Insurance == nil {
		t.Fatalf("expected insurance fields")
	}
	if result.Extracted.Insurance.Carrier != "Acme Insurance Company" ||
		result.Extracted.Insurance.PolicyNumber != "POL-123-45" ||
		result.Extracted.Insurance.EffectiveDate != "2026-01-01" ||
		result.Extracted.Insurance.ExpirationDate != "2027-01-01" {
		t.Fatalf("unexpected insurance fields: %#v", result.Extracted.Insurance)
	}
	if result.Extracted.CertificateHolder != "Big Broker LLC" ||
		result.Extracted.RemitTo != "Example Trucking LLC, PO Box 100" ||
		result.Extracted.Payee != "Example Trucking LLC" ||
		result.Extracted.FactoringCompany != "Apex Factoring Inc." {
		t.Fatalf("unexpected packet party fields: %#v", result.Extracted)
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
	if !strings.Contains(markdownOut.String(), "| Insurance carrier | Acme Insurance Company |") {
		t.Fatalf("markdown output missing insurance carrier: %s", markdownOut.String())
	}

	var tableOut bytes.Buffer
	if err := WriteExtract(&tableOut, result, "table"); err != nil {
		t.Fatalf("WriteExtract table failed: %v", err)
	}
	if !strings.Contains(tableOut.String(), "Legal name: Example Trucking LLC") {
		t.Fatalf("table output missing legal name: %s", tableOut.String())
	}
	if !strings.Contains(tableOut.String(), "Factoring company: Apex Factoring Inc.") {
		t.Fatalf("table output missing factoring company: %s", tableOut.String())
	}
}

func TestExtractFieldsFromMultilinePacketSections(t *testing.T) {
	text := `Carrier Packet

INSURER A : Great West Casualty Company
Policy #: GW-998877
Policy Period: 6/1/2025 - 6/1/2026

CERTIFICATE HOLDER
ACME Logistics Inc.
123 Warehouse Way
CANCELLATION

Remit-to:
Apex Factoring LLC
PO Box 900

Factoring Company: Apex Factoring LLC
`
	extracted := ExtractFields(text)
	if extracted.Insurance == nil {
		t.Fatalf("expected insurance fields")
	}
	if extracted.Insurance.Carrier != "Great West Casualty Company" {
		t.Fatalf("insurance carrier = %q", extracted.Insurance.Carrier)
	}
	if extracted.Insurance.PolicyNumber != "GW-998877" {
		t.Fatalf("policy number = %q", extracted.Insurance.PolicyNumber)
	}
	if extracted.Insurance.EffectiveDate != "2025-06-01" || extracted.Insurance.ExpirationDate != "2026-06-01" {
		t.Fatalf("insurance dates = %q / %q", extracted.Insurance.EffectiveDate, extracted.Insurance.ExpirationDate)
	}
	if extracted.CertificateHolder != "ACME Logistics Inc., 123 Warehouse Way" {
		t.Fatalf("certificate holder = %q", extracted.CertificateHolder)
	}
	if extracted.RemitTo != "Apex Factoring LLC, PO Box 900" {
		t.Fatalf("remit to = %q", extracted.RemitTo)
	}
	if extracted.FactoringCompany != "Apex Factoring LLC" {
		t.Fatalf("factoring company = %q", extracted.FactoringCompany)
	}
}

func TestCompareInsuranceFields(t *testing.T) {
	packetFields := ExtractedFields{
		Insurance: &ExtractedInsurance{
			Carrier:        "Great West Casualty Company",
			PolicyNumber:   "GW-998877",
			EffectiveDate:  "06/01/2025",
			ExpirationDate: "06/01/2026",
		},
	}
	carrier := domain.CarrierProfile{
		Insurance: []domain.InsuranceRecord{
			{
				InsurerName:         "Great West Casualty Company",
				PolicyNumber:        "GW998877",
				EffectiveDate:       "2025-06-01",
				CancelEffectiveDate: "2026-06-01",
			},
		},
	}

	comparisons := comparisonsByField(Compare(packetFields, carrier))
	for _, field := range []string{
		"insurance.carrier",
		"insurance.policy_number",
		"insurance.effective_date",
		"insurance.expiration_date",
	} {
		comparison, ok := comparisons[field]
		if !ok {
			t.Fatalf("missing comparison for %s", field)
		}
		if comparison.Status != "match" {
			t.Fatalf("%s status = %q, comparison: %#v", field, comparison.Status, comparison)
		}
	}
	if comparisons["insurance.effective_date"].PacketValue != "2025-06-01" {
		t.Fatalf("effective date was not normalized: %#v", comparisons["insurance.effective_date"])
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
Insurance Carrier: Acme Insurance Company
Policy Number: POL-123-45
Effective Date: 01/01/2026
Expiration Date: 01/01/2027
Certificate Holder: Big Broker LLC
Remit To:
Example Trucking LLC
PO Box 100
Payee: Example Trucking LLC
Factoring Company: Apex Factoring Inc.
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func comparisonsByField(comparisons []FieldComparison) map[string]FieldComparison {
	out := map[string]FieldComparison{}
	for _, comparison := range comparisons {
		out[comparison.Field] = comparison
	}
	return out
}
