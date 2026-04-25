package normalize

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/openhaulguard/openhaulguard/internal/domain"
	"github.com/openhaulguard/openhaulguard/internal/sources/fmcsa"
)

func TestFMCSAFixtureToCarrier(t *testing.T) {
	body, err := os.ReadFile("../../examples/fixtures/fmcsa_qcmobile/fmcsa_qcmobile_carrier_valid.json")
	if err != nil {
		t.Fatal(err)
	}
	observedAt := time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
	carrier, err := FMCSAResponsesToCarrier("mc", "123456", []fmcsa.RawResponse{{Endpoint: "fixture", Body: body}}, observedAt)
	if err != nil {
		t.Fatal(err)
	}
	if carrier.USDOTNumber != "1234567" {
		t.Fatalf("USDOT = %q", carrier.USDOTNumber)
	}
	if carrier.Contact.Phone != "+15555555555" {
		t.Fatalf("phone = %q", carrier.Contact.Phone)
	}
	if len(carrier.Authority) == 0 {
		t.Fatalf("expected authority record")
	}
}

func TestFMCSANestedFixtureToCarrier(t *testing.T) {
	body, err := os.ReadFile("../../examples/fixtures/fmcsa_qcmobile/fmcsa_qcmobile_carrier_nested.json")
	if err != nil {
		t.Fatal(err)
	}
	observedAt := time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
	carrier, err := FMCSAResponsesToCarrier("ff", "765432", []fmcsa.RawResponse{
		{Endpoint: "fixture", Body: body},
		{Endpoint: "/carriers/2345678/oos", Body: body},
	}, observedAt)
	if err != nil {
		t.Fatal(err)
	}
	if carrier.USDOTNumber != "2345678" {
		t.Fatalf("USDOT = %q", carrier.USDOTNumber)
	}
	if carrier.PhysicalAddress.Line1 != "200 Warehouse Avenue" || carrier.PhysicalAddress.PostalCode != "75201" {
		t.Fatalf("physical address = %+v", carrier.PhysicalAddress)
	}
	if carrier.Contact.Phone != "+12145550101" {
		t.Fatalf("phone = %q", carrier.Contact.Phone)
	}
	if !hasIdentifier(carrier.Identifiers, "FF", "765432") {
		t.Fatalf("identifiers = %+v", carrier.Identifiers)
	}
	if len(carrier.Authority) != 1 || carrier.Authority[0].DocketType != "FF" || carrier.Authority[0].AuthorityStatus != "ACTIVE" {
		t.Fatalf("authority = %+v", carrier.Authority)
	}
	if carrier.Safety.OutOfServiceStatus != "No" {
		t.Fatalf("oos = %q", carrier.Safety.OutOfServiceStatus)
	}
}

func TestSocrataCompanyCensusFixtureToCarrier(t *testing.T) {
	body, err := os.ReadFile("../../examples/fixtures/socrata/company_census_rows.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []map[string]any
	if err := json.Unmarshal(body, &rows); err != nil {
		t.Fatal(err)
	}
	rowBody, err := json.Marshal(rows[1])
	if err != nil {
		t.Fatal(err)
	}
	observedAt := time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
	carrier, err := FMCSAResponsesToCarrier("mc", "123456", []fmcsa.RawResponse{{
		Endpoint: "fixture:az4n-8mr2",
		Body:     rowBody,
		Fetch:    domain.SourceFetchResult{SourceName: "dot_datahub_socrata"},
	}}, observedAt)
	if err != nil {
		t.Fatal(err)
	}
	if carrier.USDOTNumber != "1234567" {
		t.Fatalf("USDOT = %q", carrier.USDOTNumber)
	}
	if carrier.PhysicalAddress.PostalCode != "38103" || carrier.MailingAddress.PostalCode != "38101" {
		t.Fatalf("addresses = physical %+v mailing %+v", carrier.PhysicalAddress, carrier.MailingAddress)
	}
	if carrier.Operations.PowerUnits != 12 || carrier.Operations.Drivers != 14 {
		t.Fatalf("operations = %+v", carrier.Operations)
	}
	if !hasAuthority(carrier.Authority, "COMMON", "ACTIVE", "dot_datahub_socrata") {
		t.Fatalf("authority = %+v", carrier.Authority)
	}
}

func hasIdentifier(ids []domain.Identifier, typ, value string) bool {
	for _, id := range ids {
		if id.Type == typ && id.Value == value {
			return true
		}
	}
	return false
}

func hasAuthority(records []domain.AuthorityRecord, typ, status, source string) bool {
	for _, record := range records {
		if record.AuthorityType == typ && record.AuthorityStatus == status && record.Source == source {
			return true
		}
	}
	return false
}
