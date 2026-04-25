package mirror

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/openhaulguard/openhaulguard/internal/domain"
	"github.com/openhaulguard/openhaulguard/internal/normalize"
)

const CompanyCensusAttribution = "U.S. Department of Transportation Federal Motor Carrier Safety Administration company census data via Socrata/DataHub."

type CompanyCensusRow = map[string]any

type BuildOptions struct {
	GeneratedAt     time.Time
	SourceTimestamp string
	Attribution     string
}

func BuildCompanyCensusJSON(body []byte, opts BuildOptions) (Index, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()

	var rows []CompanyCensusRow
	if err := dec.Decode(&rows); err != nil {
		return Index{}, fmt.Errorf("parse company census rows: %w", err)
	}
	return BuildCompanyCensusIndex(rows, opts)
}

func BuildCompanyCensusIndex(rows []CompanyCensusRow, opts BuildOptions) (Index, error) {
	generatedAt, err := generatedAtString(opts)
	if err != nil {
		return Index{}, err
	}
	sourceTimestamp := strings.TrimSpace(opts.SourceTimestamp)
	observedAt := observedAtString(sourceTimestamp, generatedAt)

	carriers := make([]domain.CarrierProfile, 0, len(rows))
	for i, row := range rows {
		carrier, err := CompanyCensusRowToCarrier(row, observedAt)
		if err != nil {
			return Index{}, fmt.Errorf("convert company census row %d: %w", i, err)
		}
		carriers = append(carriers, carrier)
	}
	sort.SliceStable(carriers, func(i, j int) bool {
		return carrierSortKey(carriers[i]) < carrierSortKey(carriers[j])
	})

	attribution := strings.TrimSpace(opts.Attribution)
	if attribution == "" {
		attribution = CompanyCensusAttribution
	}
	return Index{
		SchemaVersion:   domain.SchemaVersion,
		GeneratedAt:     generatedAt,
		SourceTimestamp: sourceTimestamp,
		Attribution:     attribution,
		Carriers:        carriers,
	}, nil
}

func CompanyCensusRowToCarrier(row CompanyCensusRow, observedAt string) (domain.CarrierProfile, error) {
	usdot := digitsOnly(rowString(row, "dot_number", "dotNumber", "usdot_number", "usdotNumber", "usdot_no", "usdotNo", "usdot", "dot"))
	if usdot == "" {
		return domain.CarrierProfile{}, fmt.Errorf("missing USDOT number")
	}

	docketType, docketNumber := parseDocket(rowString(row, "docket_number", "docketNumber", "docket_nbr", "docketNbr", "docket_no", "docketNo", "docket"))
	authority := companyCensusAuthority(row, docketType, docketNumber, observedAt)

	carrier := domain.CarrierProfile{
		USDOTNumber:       usdot,
		LegalName:         rowString(row, "legal_name", "legalName", "carrier_name", "carrierName", "name"),
		DBAName:           rowString(row, "dba_name", "dbaName", "dba", "doing_business_as", "doingBusinessAs"),
		PhysicalAddress:   companyCensusAddress(row, "physical"),
		MailingAddress:    companyCensusAddress(row, "mailing"),
		Contact:           companyCensusContact(row),
		Operations:        companyCensusOperations(row),
		Authority:         authority,
		SourceFirstSeenAt: observedAt,
	}
	if docketNumber != "" {
		carrier.Identifiers = []domain.Identifier{{
			Type:   docketType,
			Value:  docketNumber,
			Status: identifierStatus(authority),
		}}
	}
	return carrier, nil
}

func generatedAtString(opts BuildOptions) (string, error) {
	if !opts.GeneratedAt.IsZero() {
		return opts.GeneratedAt.UTC().Format(time.RFC3339), nil
	}
	if sourceTimestamp := strings.TrimSpace(opts.SourceTimestamp); sourceTimestamp != "" {
		if t, ok := normalize.ParseDate(sourceTimestamp); ok {
			return t.UTC().Format(time.RFC3339), nil
		}
	}
	return "", fmt.Errorf("generated_at is required")
}

func observedAtString(sourceTimestamp, generatedAt string) string {
	if t, ok := normalize.ParseDate(sourceTimestamp); ok {
		return t.UTC().Format(time.RFC3339)
	}
	if t, err := time.Parse(time.RFC3339, strings.TrimSpace(generatedAt)); err == nil {
		return t.UTC().Format(time.RFC3339)
	}
	return strings.TrimSpace(generatedAt)
}

func companyCensusAddress(row CompanyCensusRow, typ string) domain.Address {
	var streetKeys, street2Keys, cityKeys, stateKeys, postalKeys, countryKeys []string
	switch typ {
	case "physical":
		streetKeys = []string{"phy_street", "phyStreet", "physical_street", "physicalStreet", "physical_address_line1", "physicalAddressLine1", "street"}
		street2Keys = []string{"phy_street2", "phyStreet2", "physical_street2", "physicalStreet2", "physical_address_line2", "physicalAddressLine2"}
		cityKeys = []string{"phy_city", "phyCity", "physical_city", "physicalCity", "city"}
		stateKeys = []string{"phy_state", "phyState", "physical_state", "physicalState", "state"}
		postalKeys = []string{"phy_zip", "phyZip", "phy_zipcode", "phyZipcode", "physical_zip", "physicalZip", "physical_postal_code", "physicalPostalCode", "zip", "zip_code", "zipCode", "postal_code", "postalCode"}
		countryKeys = []string{"phy_country", "phyCountry", "physical_country", "physicalCountry", "country"}
	case "mailing":
		streetKeys = []string{"mailing_street", "mailingStreet", "mail_street", "mailStreet", "mailing_address_line1", "mailingAddressLine1"}
		street2Keys = []string{"mailing_street2", "mailingStreet2", "mail_street2", "mailStreet2", "mailing_address_line2", "mailingAddressLine2"}
		cityKeys = []string{"mailing_city", "mailingCity", "mail_city", "mailCity"}
		stateKeys = []string{"mailing_state", "mailingState", "mail_state", "mailState"}
		postalKeys = []string{"mailing_zip", "mailingZip", "mailing_zipcode", "mailingZipcode", "mail_zip", "mailZip", "mailing_postal_code", "mailingPostalCode"}
		countryKeys = []string{"mailing_country", "mailingCountry", "mail_country", "mailCountry"}
	default:
		return domain.Address{}
	}

	addr := domain.Address{
		Line1:      rowString(row, streetKeys...),
		Line2:      rowString(row, street2Keys...),
		City:       rowString(row, cityKeys...),
		State:      strings.ToUpper(rowString(row, stateKeys...)),
		PostalCode: rowString(row, postalKeys...),
		Country:    strings.ToUpper(rowString(row, countryKeys...)),
	}
	if addressPresent(addr) && addr.Country == "" {
		addr.Country = "US"
	}
	return addr
}

func addressPresent(addr domain.Address) bool {
	return addr.Line1 != "" || addr.Line2 != "" || addr.City != "" || addr.State != "" || addr.PostalCode != "" || addr.Country != ""
}

func companyCensusContact(row CompanyCensusRow) domain.Contact {
	return domain.Contact{
		Phone: normalize.Phone(rowString(row, "telephone", "phone", "phone_number", "phoneNumber", "tel_num", "telNum")),
		Email: strings.ToLower(rowString(row, "email_address", "emailAddress", "email")),
	}
}

func companyCensusOperations(row CompanyCensusRow) domain.Operations {
	return domain.Operations{
		PowerUnits: intFromRow(row, "nbr_power_unit", "nbrPowerUnit", "power_units", "powerUnits", "power_unit", "powerUnit"),
		Drivers:    intFromRow(row, "driver_total", "driverTotal", "drivers", "total_drivers", "totalDrivers"),
		MCS150Date: canonicalDate(rowString(row, "mcs_150_date", "mcs150_date", "mcs150Date", "mcs150_form_date", "mcs150FormDate")),
	}
}

func companyCensusAuthority(row CompanyCensusRow, docketType, docketNumber, observedAt string) []domain.AuthorityRecord {
	var out []domain.AuthorityRecord
	for _, spec := range []struct {
		typ  string
		keys []string
	}{
		{"COMMON", []string{"common_authority", "commonAuthority"}},
		{"CONTRACT", []string{"contract_authority", "contractAuthority"}},
		{"BROKER", []string{"broker_authority", "brokerAuthority"}},
	} {
		status := companyCensusAuthorityStatus(rowString(row, spec.keys...))
		if status == "" {
			continue
		}
		out = append(out, domain.AuthorityRecord{
			DocketType:      docketType,
			DocketNumber:    docketNumber,
			AuthorityType:   spec.typ,
			AuthorityStatus: status,
			Source:          SourceName,
			ObservedAt:      observedAt,
		})
	}
	return out
}

func companyCensusAuthorityStatus(code string) string {
	switch strings.ToUpper(strings.TrimSpace(code)) {
	case "":
		return ""
	case "A", "ACTIVE", "AUTHORIZED":
		return "ACTIVE"
	case "I", "INACTIVE":
		return "INACTIVE"
	case "N", "NONE", "NO":
		return "NONE"
	default:
		return strings.ToUpper(strings.TrimSpace(code))
	}
}

func identifierStatus(records []domain.AuthorityRecord) string {
	seen := map[string]bool{}
	for _, record := range records {
		seen[strings.ToUpper(strings.TrimSpace(record.AuthorityStatus))] = true
	}
	switch {
	case seen["ACTIVE"]:
		return "active"
	case seen["INACTIVE"]:
		return "inactive"
	case seen["NONE"]:
		return "none"
	default:
		return "unknown"
	}
}

func parseDocket(raw string) (string, string) {
	value := digitsOnly(raw)
	if value == "" {
		return "", ""
	}
	upper := strings.ToUpper(strings.TrimSpace(raw))
	upper = strings.TrimPrefix(upper, "#")
	for _, typ := range []string{"MC", "MX", "FF"} {
		if upper == typ || strings.HasPrefix(upper, typ+" ") || strings.HasPrefix(upper, typ+"-") || strings.HasPrefix(upper, typ) {
			return typ, value
		}
	}
	return "MC", value
}

func intFromRow(row CompanyCensusRow, keys ...string) int {
	value := digitsOnly(rowString(row, keys...))
	if value == "" {
		return 0
	}
	out, _ := strconv.Atoi(value)
	return out
}

func canonicalDate(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if t, ok := normalize.ParseDate(s); ok {
		return t.UTC().Format("2006-01-02")
	}
	return s
}

func rowString(row CompanyCensusRow, keys ...string) string {
	if len(row) == 0 || len(keys) == 0 {
		return ""
	}
	rowKeys := make([]string, 0, len(row))
	for key := range row {
		rowKeys = append(rowKeys, key)
	}
	sort.Strings(rowKeys)

	for _, wanted := range keys {
		normalizedWanted := normalizeKey(wanted)
		for _, key := range rowKeys {
			if normalizeKey(key) == normalizedWanted {
				return scalarString(row[key])
			}
		}
	}
	return ""
}

func scalarString(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case json.Number:
		return strings.TrimSpace(v.String())
	case float64:
		return strings.TrimSpace(strconv.FormatFloat(v, 'f', -1, 64))
	case float32:
		return strings.TrimSpace(strconv.FormatFloat(float64(v), 'f', -1, 32))
	case int, int64, int32, uint, uint64, uint32, bool:
		return strings.TrimSpace(fmt.Sprint(v))
	default:
		return ""
	}
}

func normalizeKey(s string) string {
	s = strings.ToLower(s)
	replacer := strings.NewReplacer("_", "", "-", "", " ", "")
	return replacer.Replace(s)
}

func carrierSortKey(carrier domain.CarrierProfile) string {
	return dotSortKey(carrier.USDOTNumber) + "\x00" + normalize.ComparableString(carrier.LegalName) + "\x00" + normalize.ComparableString(carrier.DBAName)
}

func dotSortKey(dot string) string {
	dot = digitsOnly(dot)
	if dot == "" {
		return "~"
	}
	dot = strings.TrimLeft(dot, "0")
	if dot == "" {
		dot = "0"
	}
	if len(dot) >= 20 {
		return dot
	}
	return strings.Repeat("0", 20-len(dot)) + dot
}
