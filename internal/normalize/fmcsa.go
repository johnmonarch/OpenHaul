package normalize

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/openhaulguard/openhaulguard/internal/domain"
	"github.com/openhaulguard/openhaulguard/internal/sources/fmcsa"
)

func FMCSAResponsesToCarrier(inputType, inputValue string, responses []fmcsa.RawResponse, observedAt string) (domain.CarrierProfile, error) {
	var carrier domain.CarrierProfile
	carrier.EntityType = "carrier"
	for _, response := range responses {
		if len(response.Body) == 0 {
			continue
		}
		var payload any
		if err := json.Unmarshal(response.Body, &payload); err != nil {
			return domain.CarrierProfile{}, fmt.Errorf("parse FMCSA response %s: %w", response.Endpoint, err)
		}
		switch {
		case strings.Contains(response.Endpoint, "/authority"):
			carrier.Authority = append(carrier.Authority, extractAuthority(payload, observedAt)...)
		case strings.Contains(response.Endpoint, "/docket-numbers") || strings.Contains(response.Endpoint, "/docket-number/"):
			ids, dot := extractIdentifiers(payload)
			carrier.Identifiers = mergeIdentifiers(carrier.Identifiers, ids)
			if carrier.USDOTNumber == "" {
				carrier.USDOTNumber = dot
			}
		case strings.Contains(response.Endpoint, "/oos"):
			if v := findString(payload, "oosStatus", "outOfServiceStatus", "status"); v != "" {
				carrier.Safety.OutOfServiceStatus = v
			}
		default:
			mergeCarrierFields(&carrier, payload, observedAt)
		}
	}
	if inputType == "dot" && carrier.USDOTNumber == "" {
		carrier.USDOTNumber = inputValue
	}
	if inputType == "mc" || inputType == "mx" || inputType == "ff" {
		carrier.Identifiers = mergeIdentifiers(carrier.Identifiers, []domain.Identifier{{Type: strings.ToUpper(inputType), Value: inputValue, Status: "unknown"}})
	}
	if carrier.USDOTNumber == "" {
		if dot := findStringInResponses(responses, "dotNumber", "usdotNumber", "usdot", "dot"); dot != "" {
			carrier.USDOTNumber = digitsOnly(dot)
		}
	}
	if carrier.USDOTNumber == "" {
		return domain.CarrierProfile{}, fmt.Errorf("FMCSA payload did not include a USDOT number")
	}
	carrier.SourceFirstSeenAt = observedAt
	carrier.LocalFirstSeenAt = observedAt
	carrier.LocalLastSeenAt = observedAt
	return carrier, nil
}

func mergeCarrierFields(carrier *domain.CarrierProfile, payload any, observedAt string) {
	if carrier.USDOTNumber == "" {
		carrier.USDOTNumber = digitsOnly(findString(payload, "dotNumber", "usdotNumber", "usdot", "dot"))
	}
	if carrier.LegalName == "" {
		carrier.LegalName = findString(payload, "legalName", "legal_name", "name", "carrierName")
	}
	if carrier.DBAName == "" {
		carrier.DBAName = findString(payload, "dbaName", "dba", "doingBusinessAs")
	}
	if carrier.PhysicalAddress.Line1 == "" {
		carrier.PhysicalAddress.Line1 = findString(payload, "phyStreet", "physicalStreet", "physicalAddressLine1", "street")
		carrier.PhysicalAddress.Line2 = findString(payload, "phyStreet2", "physicalAddressLine2")
		carrier.PhysicalAddress.City = findString(payload, "phyCity", "physicalCity", "city")
		carrier.PhysicalAddress.State = findString(payload, "phyState", "physicalState", "state")
		carrier.PhysicalAddress.PostalCode = findString(payload, "phyZipcode", "physicalZip", "zipCode", "postalCode")
		carrier.PhysicalAddress.Country = findString(payload, "phyCountry", "physicalCountry", "country")
	}
	if carrier.MailingAddress.Line1 == "" {
		carrier.MailingAddress.Line1 = findString(payload, "mailingStreet", "mailStreet", "mailingAddressLine1")
		carrier.MailingAddress.Line2 = findString(payload, "mailingStreet2", "mailingAddressLine2")
		carrier.MailingAddress.City = findString(payload, "mailingCity", "mailCity")
		carrier.MailingAddress.State = findString(payload, "mailingState", "mailState")
		carrier.MailingAddress.PostalCode = findString(payload, "mailingZip", "mailZip", "mailingPostalCode")
		carrier.MailingAddress.Country = findString(payload, "mailingCountry", "mailCountry")
	}
	if carrier.Contact.Phone == "" {
		carrier.Contact.Phone = Phone(findString(payload, "telephone", "phone", "phoneNumber"))
	}
	if carrier.Contact.Fax == "" {
		carrier.Contact.Fax = Phone(findString(payload, "fax", "faxNumber"))
	}
	if carrier.Contact.Email == "" {
		carrier.Contact.Email = findString(payload, "email", "emailAddress")
	}
	if carrier.Operations.PowerUnits == 0 {
		carrier.Operations.PowerUnits = findInt(payload, "powerUnits", "nbrPowerUnit", "power_unit")
	}
	if carrier.Operations.Drivers == 0 {
		carrier.Operations.Drivers = findInt(payload, "drivers", "driverTotal", "totalDrivers")
	}
	if carrier.Operations.MCS150Date == "" {
		carrier.Operations.MCS150Date = findString(payload, "mcs150Date", "mcs150_date", "mcs150FormDate")
	}
	ids, _ := extractIdentifiers(payload)
	carrier.Identifiers = mergeIdentifiers(carrier.Identifiers, ids)
	if len(carrier.Authority) == 0 {
		carrier.Authority = append(carrier.Authority, extractAuthority(payload, observedAt)...)
	}
}

func extractIdentifiers(payload any) ([]domain.Identifier, string) {
	var ids []domain.Identifier
	dot := digitsOnly(findString(payload, "dotNumber", "usdotNumber", "usdot", "dot"))
	walk(payload, func(key string, value any) {
		v := strings.TrimSpace(fmt.Sprint(value))
		if v == "" || v == "<nil>" {
			return
		}
		normalizedKey := normalizeKey(key)
		switch normalizedKey {
		case "docketnumber", "docketnbr", "mcnumber", "mxnumber", "ffnumber":
			digits := digitsOnly(v)
			if digits == "" {
				return
			}
			typ := "MC"
			if strings.Contains(normalizedKey, "mx") {
				typ = "MX"
			}
			if strings.Contains(normalizedKey, "ff") {
				typ = "FF"
			}
			ids = append(ids, domain.Identifier{Type: typ, Value: digits, Status: findString(payload, "status", "docketStatus")})
		case "prefix", "docketprefix":
			_ = v
		}
	})
	return ids, dot
}

func extractAuthority(payload any, observedAt string) []domain.AuthorityRecord {
	var out []domain.AuthorityRecord
	for _, item := range candidateMaps(payload) {
		status := pickString(item, "authorityStatus", "status", "authStatus")
		typ := pickString(item, "authorityType", "type", "authType")
		if status == "" && typ == "" {
			continue
		}
		out = append(out, domain.AuthorityRecord{
			DocketType:         pickString(item, "docketType", "prefix"),
			DocketNumber:       digitsOnly(pickString(item, "docketNumber", "docketNbr")),
			AuthorityType:      typ,
			AuthorityStatus:    status,
			OriginalAction:     pickString(item, "originalAction", "origAction"),
			OriginalActionDate: pickString(item, "originalActionDate", "origActionDate", "grantedDate"),
			FinalAction:        pickString(item, "finalAction"),
			FinalActionDate:    pickString(item, "finalActionDate"),
			Source:             fmcsa.SourceName,
			ObservedAt:         observedAt,
		})
	}
	if len(out) == 0 {
		status := findString(payload, "authorityStatus", "status", "authStatus")
		if status != "" {
			out = append(out, domain.AuthorityRecord{
				AuthorityStatus:    status,
				AuthorityType:      findString(payload, "authorityType", "type", "authType"),
				OriginalActionDate: findString(payload, "originalActionDate", "origActionDate", "grantedDate"),
				Source:             fmcsa.SourceName,
				ObservedAt:         observedAt,
			})
		}
	}
	return out
}

func findStringInResponses(responses []fmcsa.RawResponse, keys ...string) string {
	for _, response := range responses {
		var payload any
		if json.Unmarshal(response.Body, &payload) == nil {
			if v := findString(payload, keys...); v != "" {
				return v
			}
		}
	}
	return ""
}

func findString(payload any, keys ...string) string {
	wanted := map[string]bool{}
	for _, key := range keys {
		wanted[normalizeKey(key)] = true
	}
	var found string
	walk(payload, func(key string, value any) {
		if found != "" || !wanted[normalizeKey(key)] {
			return
		}
		found = strings.TrimSpace(fmt.Sprint(value))
		if found == "<nil>" {
			found = ""
		}
	})
	return found
}

func findInt(payload any, keys ...string) int {
	v := findString(payload, keys...)
	if v == "" {
		return 0
	}
	i, _ := strconv.Atoi(digitsOnly(v))
	return i
}

func candidateMaps(payload any) []map[string]any {
	var out []map[string]any
	walkMap(payload, func(m map[string]any) {
		out = append(out, m)
	})
	return out
}

func pickString(m map[string]any, keys ...string) string {
	wanted := map[string]bool{}
	for _, key := range keys {
		wanted[normalizeKey(key)] = true
	}
	for key, value := range m {
		if wanted[normalizeKey(key)] {
			return strings.TrimSpace(fmt.Sprint(value))
		}
	}
	return ""
}

func mergeIdentifiers(existing, incoming []domain.Identifier) []domain.Identifier {
	seen := map[string]bool{}
	var out []domain.Identifier
	for _, id := range append(existing, incoming...) {
		typ := strings.ToUpper(strings.TrimSpace(id.Type))
		value := digitsOnly(id.Value)
		if typ == "" || value == "" {
			continue
		}
		key := typ + ":" + value
		if seen[key] {
			continue
		}
		if id.Status == "" {
			id.Status = "unknown"
		}
		id.Type = typ
		id.Value = value
		seen[key] = true
		out = append(out, id)
	}
	return out
}

func walk(payload any, visit func(key string, value any)) {
	switch v := payload.(type) {
	case map[string]any:
		for key, value := range v {
			visit(key, value)
			walk(value, visit)
		}
	case []any:
		for _, item := range v {
			walk(item, visit)
		}
	}
}

func walkMap(payload any, visit func(map[string]any)) {
	switch v := payload.(type) {
	case map[string]any:
		visit(v)
		for _, value := range v {
			walkMap(value, visit)
		}
	case []any:
		for _, item := range v {
			walkMap(item, visit)
		}
	}
}

func normalizeKey(s string) string {
	s = strings.ToLower(s)
	replacer := strings.NewReplacer("_", "", "-", "", " ", "")
	return replacer.Replace(s)
}

func ParseDate(s string) (time.Time, bool) {
	for _, layout := range []string{time.RFC3339, "2006-01-02", "01/02/2006", "20060102"} {
		if t, err := time.Parse(layout, strings.TrimSpace(s)); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
