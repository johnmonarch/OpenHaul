package mirror

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/openhaulguard/openhaulguard/internal/domain"
)

func TestBuildCompanyCensusJSONConvertsFixture(t *testing.T) {
	body := readFixture(t, "socrata", "company_census_rows.json")
	index, err := BuildCompanyCensusJSON(body, testBuildOptions())
	if err != nil {
		t.Fatal(err)
	}

	if index.SchemaVersion != "1.0" {
		t.Fatalf("schema_version = %q", index.SchemaVersion)
	}
	if index.GeneratedAt != "2026-04-25T14:30:00Z" {
		t.Fatalf("generated_at = %q", index.GeneratedAt)
	}
	if index.SourceTimestamp != "2026-04-24" {
		t.Fatalf("source_timestamp = %q", index.SourceTimestamp)
	}
	if index.Attribution != "test census attribution" {
		t.Fatalf("attribution = %q", index.Attribution)
	}
	if len(index.Carriers) != 2 {
		t.Fatalf("carriers = %d", len(index.Carriers))
	}

	carrier := index.Carriers[0]
	if carrier.USDOTNumber != "1234567" {
		t.Fatalf("first carrier USDOT = %q", carrier.USDOTNumber)
	}
	if carrier.LegalName != "EXAMPLE TRUCKING LLC" || carrier.DBAName != "EXAMPLE HAUL" {
		t.Fatalf("names = legal %q dba %q", carrier.LegalName, carrier.DBAName)
	}
	if carrier.PhysicalAddress.Line1 != "100 Main Street" || carrier.PhysicalAddress.City != "Memphis" || carrier.PhysicalAddress.State != "TN" || carrier.PhysicalAddress.PostalCode != "38103" || carrier.PhysicalAddress.Country != "US" {
		t.Fatalf("physical address = %+v", carrier.PhysicalAddress)
	}
	if carrier.MailingAddress.Line1 != "PO Box 100" || carrier.MailingAddress.PostalCode != "38101" || carrier.MailingAddress.Country != "US" {
		t.Fatalf("mailing address = %+v", carrier.MailingAddress)
	}
	if carrier.Contact.Phone != "+15555555555" || carrier.Contact.Email != "dispatch@exampletrucking.test" {
		t.Fatalf("contact = %+v", carrier.Contact)
	}
	if carrier.Operations.PowerUnits != 12 || carrier.Operations.Drivers != 14 || carrier.Operations.MCS150Date != "2026-03-15" {
		t.Fatalf("operations = %+v", carrier.Operations)
	}
	if carrier.SourceFirstSeenAt != "2026-04-24T00:00:00Z" {
		t.Fatalf("source_first_seen_at = %q", carrier.SourceFirstSeenAt)
	}
	if !hasIdentifier(carrier.Identifiers, "MC", "123456", "active") {
		t.Fatalf("identifiers = %+v", carrier.Identifiers)
	}
	if !hasAuthority(carrier.Authority, "MC", "123456", "COMMON", "ACTIVE") {
		t.Fatalf("authority = %+v", carrier.Authority)
	}
	if !hasAuthority(carrier.Authority, "MC", "123456", "CONTRACT", "NONE") {
		t.Fatalf("authority = %+v", carrier.Authority)
	}
	if !hasAuthority(carrier.Authority, "MC", "123456", "BROKER", "NONE") {
		t.Fatalf("authority = %+v", carrier.Authority)
	}
}

func TestCompanyCensusAuthorityStatusMapping(t *testing.T) {
	for _, tc := range []struct {
		name string
		code string
		want string
	}{
		{name: "active code", code: "A", want: "ACTIVE"},
		{name: "inactive code", code: "I", want: "INACTIVE"},
		{name: "none code", code: "N", want: "NONE"},
		{name: "active word", code: "active", want: "ACTIVE"},
		{name: "unknown passthrough upper", code: "pending", want: "PENDING"},
		{name: "blank", code: " ", want: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := companyCensusAuthorityStatus(tc.code); got != tc.want {
				t.Fatalf("status = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCompanyCensusIdentifiersFromDocket(t *testing.T) {
	carrier, err := CompanyCensusRowToCarrier(CompanyCensusRow{
		"dot_number":         " 001234567 ",
		"docket_number":      "MX-987654",
		"contract_authority": "I",
	}, "2026-04-24T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if carrier.USDOTNumber != "001234567" {
		t.Fatalf("USDOT = %q", carrier.USDOTNumber)
	}
	if !hasIdentifier(carrier.Identifiers, "MX", "987654", "inactive") {
		t.Fatalf("identifiers = %+v", carrier.Identifiers)
	}
	if !hasAuthority(carrier.Authority, "MX", "987654", "CONTRACT", "INACTIVE") {
		t.Fatalf("authority = %+v", carrier.Authority)
	}
}

func TestCompanyCensusContactNormalization(t *testing.T) {
	carrier, err := CompanyCensusRowToCarrier(CompanyCensusRow{
		"dot_number":    "1234567",
		"telephone":     "1 (901) 555-0199",
		"email_address": " DISPATCH@ExampleTrucking.Test ",
	}, "2026-04-24T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if carrier.Contact.Phone != "+19015550199" {
		t.Fatalf("phone = %q", carrier.Contact.Phone)
	}
	if carrier.Contact.Email != "dispatch@exampletrucking.test" {
		t.Fatalf("email = %q", carrier.Contact.Email)
	}
}

func TestBuildCompanyCensusIndexStableLookup(t *testing.T) {
	body := readFixture(t, "socrata", "company_census_rows.json")
	index, err := BuildCompanyCensusJSON(body, testBuildOptions())
	if err != nil {
		t.Fatal(err)
	}

	var reversedRows []CompanyCensusRow
	if err := json.Unmarshal(body, &reversedRows); err != nil {
		t.Fatal(err)
	}
	for i, j := 0, len(reversedRows)-1; i < j; i, j = i+1, j-1 {
		reversedRows[i], reversedRows[j] = reversedRows[j], reversedRows[i]
	}
	reversedIndex, err := BuildCompanyCensusIndex(reversedRows, testBuildOptions())
	if err != nil {
		t.Fatal(err)
	}
	got, err := json.Marshal(index)
	if err != nil {
		t.Fatal(err)
	}
	want, err := json.Marshal(reversedIndex)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("index output changed after row reordering\ngot  %s\nwant %s", got, want)
	}

	path := filepath.Join(t.TempDir(), "carriers.json")
	if err := os.WriteFile(path, got, 0o600); err != nil {
		t.Fatal(err)
	}
	observedAt := time.Date(2026, 4, 26, 9, 0, 0, 0, time.UTC)
	carrier, fetch, ok, err := Lookup(context.Background(), path, "mc", "MC-123456", observedAt)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("lookup did not find MC-123456")
	}
	if carrier.USDOTNumber != "1234567" || carrier.Contact.Phone != "+15555555555" {
		t.Fatalf("lookup carrier = %+v", carrier)
	}
	if fetch.SourceName != SourceName || fetch.FetchedAt != "2026-04-26T09:00:00Z" || fetch.ResponseHash == "" {
		t.Fatalf("fetch = %+v", fetch)
	}
}

func testBuildOptions() BuildOptions {
	return BuildOptions{
		GeneratedAt:     time.Date(2026, 4, 25, 14, 30, 0, 0, time.UTC),
		SourceTimestamp: "2026-04-24",
		Attribution:     "test census attribution",
	}
}

func readFixture(t *testing.T, parts ...string) []byte {
	t.Helper()
	pathParts := append([]string{"..", "..", "..", "examples", "fixtures"}, parts...)
	body, err := os.ReadFile(filepath.Join(pathParts...))
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func hasIdentifier(ids []domain.Identifier, typ, value, status string) bool {
	for _, id := range ids {
		if id.Type == typ && id.Value == value && id.Status == status {
			return true
		}
	}
	return false
}

func hasAuthority(records []domain.AuthorityRecord, docketType, docketNumber, authorityType, authorityStatus string) bool {
	for _, record := range records {
		if record.DocketType == docketType && record.DocketNumber == docketNumber && record.AuthorityType == authorityType && record.AuthorityStatus == authorityStatus && record.Source == SourceName && record.ObservedAt == "2026-04-24T00:00:00Z" {
			return true
		}
	}
	return false
}
