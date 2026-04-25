package scoring

import (
	"strings"
	"time"

	"github.com/openhaulguard/openhaulguard/internal/domain"
	norm "github.com/openhaulguard/openhaulguard/internal/normalize"
)

type Context struct {
	ObservationCount int
	ObservedAt       time.Time
}

const (
	profileSource    = "carrier_profile"
	mcs150StaleYears = 2
)

func Assess(carrier domain.CarrierProfile, ctx Context) domain.RiskAssessment {
	if ctx.ObservedAt.IsZero() {
		ctx.ObservedAt = time.Now().UTC()
	}
	var flags []domain.RiskFlag
	observedAt := profileObservedAt(carrier, ctx.ObservedAt)
	if ctx.ObservationCount <= 1 {
		flags = append(flags, domain.RiskFlag{
			Code:         "NO_LOCAL_HISTORY",
			Severity:     "info",
			Category:     "history",
			Explanation:  "This is the first local observation for this carrier. Local change detection starts now.",
			WhyItMatters: "OpenHaul Guard can only detect local changes after it has observed a carrier more than once.",
			NextStep:     "Run future lookups or add this carrier to a watchlist to build local history.",
			Confidence:   "high",
			Evidence: []domain.Evidence{{
				Field:      "carrier.local_first_seen_at",
				Value:      carrier.LocalFirstSeenAt,
				Source:     "local_observation",
				ObservedAt: carrier.LocalLastSeenAt,
			}},
		})
	}
	flags = append(flags, identityFlags(carrier, observedAt)...)
	flags = append(flags, authorityFlags(carrier, ctx.ObservedAt, observedAt)...)
	flags = append(flags, operationsFlags(carrier, ctx.ObservedAt, observedAt)...)
	flags = append(flags, contactFlags(carrier, observedAt)...)
	flags = append(flags, safetyFlags(carrier, observedAt)...)
	score := 0
	for _, flag := range flags {
		score += severityScore(flag.Severity)
	}
	if score > 100 {
		score = 100
	}
	return domain.RiskAssessment{
		Score:          score,
		Recommendation: recommendation(score, flags),
		Flags:          flags,
	}
}

func identityFlags(carrier domain.CarrierProfile, observedAt string) []domain.RiskFlag {
	var flags []domain.RiskFlag
	if blank(carrier.USDOTNumber) {
		flags = append(flags, domain.RiskFlag{
			Code:         "MISSING_USDOT",
			Severity:     "high",
			Category:     "identity",
			Explanation:  "The carrier profile does not include a USDOT number.",
			WhyItMatters: "A USDOT number is a core public-record identifier for carrier validation.",
			NextStep:     "Confirm the carrier identifier directly in FMCSA records before proceeding.",
			Confidence:   "high",
			Evidence: []domain.Evidence{{
				Field:      "carrier.usdot_number",
				Value:      carrier.USDOTNumber,
				Source:     profileSource,
				ObservedAt: observedAt,
			}},
		})
	}
	if blank(carrier.LegalName) {
		flags = append(flags, domain.RiskFlag{
			Code:         "MISSING_LEGAL_NAME",
			Severity:     "medium",
			Category:     "identity",
			Explanation:  "The carrier profile does not include a legal name.",
			WhyItMatters: "A legal name is needed to compare public records, packets, and onboarding documents.",
			NextStep:     "Confirm the legal name through FMCSA records or the carrier packet.",
			Confidence:   "high",
			Evidence: []domain.Evidence{{
				Field:      "carrier.legal_name",
				Value:      carrier.LegalName,
				Source:     profileSource,
				ObservedAt: observedAt,
			}},
		})
	}
	flags = append(flags, identifierMismatchFlags(carrier, observedAt)...)
	return flags
}

func authorityFlags(carrier domain.CarrierProfile, observedAt time.Time, fallbackObservedAt string) []domain.RiskFlag {
	var flags []domain.RiskFlag
	var notActiveEvidence []domain.Evidence
	var noActiveEvidence []domain.Evidence
	var missingTypeEvidence []domain.Evidence
	var docketMismatchEvidence []domain.Evidence
	identifierValues := identifierValuesByType(carrier.Identifiers)
	hasStatus := false
	hasActive := false

	if len(carrier.Authority) == 0 {
		if evidence := docketIdentifierEvidence(carrier.Identifiers, fallbackObservedAt); len(evidence) > 0 {
			flags = append(flags, domain.RiskFlag{
				Code:         "AUTHORITY_RECORDS_MISSING",
				Severity:     "medium",
				Category:     "authority",
				Explanation:  "The carrier profile includes docket identifiers but no authority records.",
				WhyItMatters: "Authority records are needed to confirm the current operating authority attached to a docket.",
				NextStep:     "Check the current FMCSA authority record for the listed docket identifier before proceeding.",
				Confidence:   "high",
				Evidence:     evidence,
			})
		}
		return flags
	}

	for _, authority := range carrier.Authority {
		status := normalizedStatus(authority.AuthorityStatus)
		if status == "" {
			continue
		}
		hasStatus = true
		if evidence, ok := authorityDocketMismatchEvidence(authority, identifierValues, fallbackObservedAt); ok {
			docketMismatchEvidence = append(docketMismatchEvidence, evidence)
		}
		if activeAuthorityStatus(status) {
			hasActive = true
			if blank(authority.AuthorityType) {
				missingTypeEvidence = append(missingTypeEvidence, domain.Evidence{
					Field:      "authority.authority_type",
					Value:      authority.AuthorityType,
					Source:     evidenceSource(authority.Source),
					ObservedAt: evidenceObservedAt(authority.ObservedAt, fallbackObservedAt),
				})
			}
			continue
		}
		evidence := authorityEvidence(authority, fallbackObservedAt)
		noActiveEvidence = append(noActiveEvidence, evidence)
		if notActiveAuthorityStatus(status) {
			notActiveEvidence = append(notActiveEvidence, evidence)
		}
	}
	if len(notActiveEvidence) > 0 {
		flags = append(flags, domain.RiskFlag{
			Code:         "AUTHORITY_NOT_ACTIVE",
			Severity:     "critical",
			Category:     "authority",
			Explanation:  "Operating authority is not active according to the source record.",
			WhyItMatters: "Inactive, revoked, pending, or not-authorized authority requires compliance review before use.",
			NextStep:     "Check the current authority record directly and confirm the required operation type.",
			Confidence:   "high",
			Evidence:     notActiveEvidence,
		})
	}
	if len(notActiveEvidence) == 0 && len(carrier.Authority) > 0 && hasStatus && !hasActive {
		flags = append(flags, domain.RiskFlag{
			Code:         "ACTIVE_AUTHORITY_NOT_FOUND",
			Severity:     "high",
			Category:     "authority",
			Explanation:  "Authority records are present, but none show active authority.",
			WhyItMatters: "A carrier without a current active authority record needs manual compliance review before use.",
			NextStep:     "Confirm the current authority status directly in FMCSA records and verify the required operation type.",
			Confidence:   "high",
			Evidence:     noActiveEvidence,
		})
	}
	if len(docketMismatchEvidence) > 0 {
		flags = append(flags, domain.RiskFlag{
			Code:         "AUTHORITY_DOCKET_MISMATCH",
			Severity:     "high",
			Category:     "authority",
			Explanation:  "An authority record docket number differs from the carrier identifier for the same docket type.",
			WhyItMatters: "Docket mismatches inside the public profile should be reconciled before relying on the authority record.",
			NextStep:     "Confirm the docket number and authority record directly in FMCSA records.",
			Confidence:   "high",
			Evidence:     docketMismatchEvidence,
		})
	}
	if len(missingTypeEvidence) > 0 {
		flags = append(flags, domain.RiskFlag{
			Code:         "ACTIVE_AUTHORITY_TYPE_MISSING",
			Severity:     "low",
			Category:     "authority",
			Explanation:  "An active authority record does not include an authority type.",
			WhyItMatters: "The authority type helps confirm whether the record applies to the requested operation.",
			NextStep:     "Confirm the active authority type in FMCSA records before relying on this profile.",
			Confidence:   "high",
			Evidence:     missingTypeEvidence,
		})
	}

	var youngestAuthority *domain.AuthorityRecord
	var youngestAgeDays int
	for i := range carrier.Authority {
		authority := &carrier.Authority[i]
		status := normalizedStatus(authority.AuthorityStatus)
		if status != "" && !activeAuthorityStatus(status) {
			continue
		}
		if blank(authority.OriginalActionDate) {
			continue
		}
		grantedAt, ok := norm.ParseDate(authority.OriginalActionDate)
		if !ok {
			continue
		}
		ageDays := int(observedAt.Sub(grantedAt).Hours() / 24)
		if ageDays < 0 {
			continue
		}
		if youngestAuthority == nil || ageDays < youngestAgeDays {
			youngestAuthority = authority
			youngestAgeDays = ageDays
		}
	}
	if youngestAuthority == nil {
		return flags
	}
	if youngestAgeDays < 30 {
		flags = append(flags, domain.RiskFlag{
			Code:         "VERY_NEW_AUTHORITY",
			Severity:     "high",
			Category:     "authority",
			Explanation:  "Operating authority appears to be less than 30 days old.",
			WhyItMatters: "Very new authority can be legitimate, but onboarding teams usually review it more carefully.",
			NextStep:     "Confirm operating authority details and supporting documents before tendering freight.",
			Confidence:   "medium",
			Evidence: []domain.Evidence{{
				Field:      "authority.original_action_date",
				Value:      youngestAuthority.OriginalActionDate,
				Source:     evidenceSource(youngestAuthority.Source),
				ObservedAt: evidenceObservedAt(youngestAuthority.ObservedAt, fallbackObservedAt),
			}},
		})
	} else if youngestAgeDays < 90 {
		flags = append(flags, domain.RiskFlag{
			Code:         "NEW_AUTHORITY",
			Severity:     "medium",
			Category:     "authority",
			Explanation:  "Operating authority appears to be less than 90 days old.",
			WhyItMatters: "New authority can be legitimate, but it is a useful signal for manual onboarding review.",
			NextStep:     "Review the carrier packet and confirm authority details through public records.",
			Confidence:   "medium",
			Evidence: []domain.Evidence{{
				Field:      "authority.original_action_date",
				Value:      youngestAuthority.OriginalActionDate,
				Source:     evidenceSource(youngestAuthority.Source),
				ObservedAt: evidenceObservedAt(youngestAuthority.ObservedAt, fallbackObservedAt),
			}},
		})
	}
	return flags
}

func operationsFlags(carrier domain.CarrierProfile, observedAt time.Time, fallbackObservedAt string) []domain.RiskFlag {
	var flags []domain.RiskFlag
	if len(carrier.Operations.OperationClassification) == 0 && hasOperationalFootprint(carrier.Operations) {
		flags = append(flags, domain.RiskFlag{
			Code:         "OPERATION_CLASSIFICATION_MISSING",
			Severity:     "low",
			Category:     "operations",
			Explanation:  "The carrier profile has operating details but no operation classification.",
			WhyItMatters: "Operation classification helps confirm how the carrier reports its operating type.",
			NextStep:     "Review the FMCSA operation classification record with the carrier profile.",
			Confidence:   "high",
			Evidence: []domain.Evidence{{
				Field:      "operations.operation_classification",
				Value:      carrier.Operations.OperationClassification,
				Source:     profileSource,
				ObservedAt: fallbackObservedAt,
			}},
		})
	}
	if !blank(carrier.Operations.MCS150Date) {
		mcs150Date, ok := norm.ParseDate(carrier.Operations.MCS150Date)
		if ok && mcs150Date.Before(observedAt.AddDate(-mcs150StaleYears, 0, 0)) {
			flags = append(flags, domain.RiskFlag{
				Code:         "STALE_MCS150",
				Severity:     "low",
				Category:     "operations",
				Explanation:  "The MCS-150 date in the carrier profile is more than two years old.",
				WhyItMatters: "Older operational profile data can lag the carrier's current operating footprint.",
				NextStep:     "Review the current FMCSA profile and ask the carrier to confirm its latest operating details.",
				Confidence:   "high",
				Evidence: []domain.Evidence{{
					Field:           "operations.mcs150_date",
					Value:           carrier.Operations.MCS150Date,
					ComparisonValue: observedAt.AddDate(-mcs150StaleYears, 0, 0).Format("2006-01-02"),
					Source:          profileSource,
					ObservedAt:      fallbackObservedAt,
				}},
			})
		}
	}
	var evidence []domain.Evidence
	if carrier.Operations.PowerUnits == 0 {
		evidence = append(evidence, domain.Evidence{
			Field:      "operations.power_units",
			Value:      carrier.Operations.PowerUnits,
			Source:     profileSource,
			ObservedAt: fallbackObservedAt,
		})
	}
	if carrier.Operations.Drivers == 0 {
		evidence = append(evidence, domain.Evidence{
			Field:      "operations.drivers",
			Value:      carrier.Operations.Drivers,
			Source:     profileSource,
			ObservedAt: fallbackObservedAt,
		})
	}
	if len(evidence) > 0 {
		flags = append(flags, domain.RiskFlag{
			Code:         "ZERO_POWER_UNITS_OR_DRIVERS",
			Severity:     "medium",
			Category:     "operations",
			Explanation:  "The carrier profile records zero power units or zero drivers.",
			WhyItMatters: "A zero operating footprint may be valid, but it should be reconciled before relying on the carrier profile.",
			NextStep:     "Confirm the current power unit and driver counts through FMCSA records or carrier documentation.",
			Confidence:   "high",
			Evidence:     evidence,
		})
	}
	return flags
}

func contactFlags(carrier domain.CarrierProfile, observedAt string) []domain.RiskFlag {
	var flags []domain.RiskFlag
	if emptyAddress(carrier.PhysicalAddress) {
		flags = append(flags, domain.RiskFlag{
			Code:         "MISSING_PHYSICAL_ADDRESS",
			Severity:     "medium",
			Category:     "contact",
			Explanation:  "The carrier profile does not include a physical address.",
			WhyItMatters: "A physical address is useful for validating the carrier against public records and onboarding documents.",
			NextStep:     "Confirm the physical address in FMCSA records or directly with the carrier.",
			Confidence:   "high",
			Evidence: []domain.Evidence{{
				Field:      "carrier.physical_address",
				Value:      carrier.PhysicalAddress,
				Source:     profileSource,
				ObservedAt: observedAt,
			}},
		})
	}
	if blank(carrier.Contact.Phone) {
		flags = append(flags, domain.RiskFlag{
			Code:         "MISSING_PHONE",
			Severity:     "low",
			Category:     "contact",
			Explanation:  "The carrier profile does not include a phone number.",
			WhyItMatters: "Missing phone data limits independent contact verification.",
			NextStep:     "Confirm the carrier's dispatch or safety contact number through an independent source.",
			Confidence:   "high",
			Evidence: []domain.Evidence{{
				Field:      "contact.phone",
				Value:      carrier.Contact.Phone,
				Source:     profileSource,
				ObservedAt: observedAt,
			}},
		})
	}
	if blank(carrier.Contact.Email) {
		flags = append(flags, domain.RiskFlag{
			Code:         "MISSING_EMAIL",
			Severity:     "low",
			Category:     "contact",
			Explanation:  "The carrier profile does not include an email address.",
			WhyItMatters: "Missing email data reduces the contact details available for onboarding verification.",
			NextStep:     "Confirm the carrier's preferred email contact through an independent source.",
			Confidence:   "high",
			Evidence: []domain.Evidence{{
				Field:      "contact.email",
				Value:      carrier.Contact.Email,
				Source:     profileSource,
				ObservedAt: observedAt,
			}},
		})
	}
	return flags
}

func safetyFlags(carrier domain.CarrierProfile, observedAt string) []domain.RiskFlag {
	var flags []domain.RiskFlag
	if staleSMSMonth(carrier.Safety.SMSMonth, observedAt) {
		flags = append(flags, domain.RiskFlag{
			Code:         "SMS_DATA_STALE",
			Severity:     "info",
			Category:     "safety",
			Explanation:  "The latest SMS month in the carrier profile is older than the expected monthly cadence plus buffer.",
			WhyItMatters: "Stale safety snapshot data should be treated as context, not as a complete current safety picture.",
			NextStep:     "Check the latest FMCSA safety data before relying on this snapshot.",
			Confidence:   "high",
			Evidence: []domain.Evidence{{
				Field:      "safety.sms_month",
				Value:      carrier.Safety.SMSMonth,
				Source:     profileSource,
				ObservedAt: observedAt,
			}},
		})
	}
	if currentOOSStatus(carrier.Safety.OutOfServiceStatus) {
		flags = append(flags, domain.RiskFlag{
			Code:         "OOS_CURRENT",
			Severity:     "critical",
			Category:     "safety",
			Explanation:  "The source data indicates a current out-of-service status.",
			WhyItMatters: "An active out-of-service condition is an objective compliance issue that requires review.",
			NextStep:     "Confirm the out-of-service status in FMCSA records before proceeding.",
			Confidence:   "high",
			Evidence: []domain.Evidence{{
				Field:      "safety.out_of_service_status",
				Value:      carrier.Safety.OutOfServiceStatus,
				Source:     "fmcsa_qcmobile_oos",
				ObservedAt: observedAt,
			}},
		})
	}
	return flags
}

func severityScore(severity string) int {
	switch severity {
	case "low":
		return 5
	case "medium":
		return 15
	case "high":
		return 30
	case "critical":
		return 60
	default:
		return 0
	}
}

func identifierMismatchFlags(carrier domain.CarrierProfile, observedAt string) []domain.RiskFlag {
	var flags []domain.RiskFlag
	var usdotMismatchEvidence []domain.Evidence
	seenByType := map[string]string{}
	var conflictEvidence []domain.Evidence
	for _, id := range carrier.Identifiers {
		typ := identifierType(id.Type)
		value := digitsOnly(id.Value)
		if typ == "" || value == "" {
			continue
		}
		if typ == "USDOT" && !blank(carrier.USDOTNumber) && value != digitsOnly(carrier.USDOTNumber) {
			usdotMismatchEvidence = append(usdotMismatchEvidence, domain.Evidence{
				Field:           "identifiers.usdot.value",
				SourceValue:     value,
				ComparisonValue: carrier.USDOTNumber,
				Source:          profileSource,
				ObservedAt:      observedAt,
			})
			continue
		}
		if typ == "USDOT" {
			continue
		}
		if previous, ok := seenByType[typ]; ok && previous != value {
			conflictEvidence = append(conflictEvidence, domain.Evidence{
				Field:           "identifiers." + strings.ToLower(typ) + ".value",
				SourceValue:     previous,
				ComparisonValue: value,
				Source:          profileSource,
				ObservedAt:      observedAt,
			})
			continue
		}
		seenByType[typ] = value
	}
	if len(usdotMismatchEvidence) > 0 {
		flags = append(flags, domain.RiskFlag{
			Code:         "USDOT_IDENTIFIER_MISMATCH",
			Severity:     "high",
			Category:     "identity",
			Explanation:  "A USDOT identifier in the carrier profile differs from the profile USDOT number.",
			WhyItMatters: "Conflicting USDOT identifiers should be reconciled before relying on the profile.",
			NextStep:     "Confirm the USDOT number directly in FMCSA records.",
			Confidence:   "high",
			Evidence:     usdotMismatchEvidence,
		})
	}
	if len(conflictEvidence) > 0 {
		flags = append(flags, domain.RiskFlag{
			Code:         "IDENTIFIER_VALUE_CONFLICT",
			Severity:     "medium",
			Category:     "identity",
			Explanation:  "The carrier profile includes multiple values for the same docket identifier type.",
			WhyItMatters: "Conflicting docket identifiers can cause authority and onboarding records to be matched to the wrong profile.",
			NextStep:     "Confirm the current docket identifier directly in FMCSA records.",
			Confidence:   "high",
			Evidence:     conflictEvidence,
		})
	}
	return flags
}

func authorityEvidence(authority domain.AuthorityRecord, fallbackObservedAt string) domain.Evidence {
	return domain.Evidence{
		Field:      "authority.authority_status",
		Value:      authority.AuthorityStatus,
		Source:     evidenceSource(authority.Source),
		ObservedAt: evidenceObservedAt(authority.ObservedAt, fallbackObservedAt),
	}
}

func authorityDocketMismatchEvidence(authority domain.AuthorityRecord, identifiers map[string][]string, fallbackObservedAt string) (domain.Evidence, bool) {
	typ := identifierType(authority.DocketType)
	if typ == "" || typ == "USDOT" || blank(authority.DocketNumber) {
		return domain.Evidence{}, false
	}
	values := identifiers[typ]
	if len(values) != 1 {
		return domain.Evidence{}, false
	}
	docketNumber := digitsOnly(authority.DocketNumber)
	if docketNumber == "" || values[0] == docketNumber {
		return domain.Evidence{}, false
	}
	return domain.Evidence{
		Field:           "authority.docket_number",
		SourceValue:     docketNumber,
		ComparisonValue: values[0],
		Source:          evidenceSource(authority.Source),
		ObservedAt:      evidenceObservedAt(authority.ObservedAt, fallbackObservedAt),
	}, true
}

func docketIdentifierEvidence(identifiers []domain.Identifier, observedAt string) []domain.Evidence {
	var evidence []domain.Evidence
	for _, id := range identifiers {
		typ := identifierType(id.Type)
		if typ == "" || typ == "USDOT" || blank(id.Value) {
			continue
		}
		evidence = append(evidence, domain.Evidence{
			Field:      "identifiers." + strings.ToLower(typ) + ".value",
			Value:      digitsOnly(id.Value),
			Source:     profileSource,
			ObservedAt: observedAt,
		})
	}
	return evidence
}

func identifierValuesByType(identifiers []domain.Identifier) map[string][]string {
	out := map[string][]string{}
	seen := map[string]bool{}
	for _, id := range identifiers {
		typ := identifierType(id.Type)
		value := digitsOnly(id.Value)
		if typ == "" || value == "" {
			continue
		}
		key := typ + ":" + value
		if seen[key] {
			continue
		}
		seen[key] = true
		out[typ] = append(out[typ], value)
	}
	return out
}

func identifierType(raw string) string {
	typ := strings.ToUpper(strings.TrimSpace(raw))
	switch typ {
	case "DOT", "USDOT":
		return "USDOT"
	case "MC", "MX", "FF":
		return typ
	default:
		return ""
	}
}

func activeAuthorityStatus(status string) bool {
	if strings.Contains(status, "not authorized") {
		return false
	}
	return status == "active" || status == "authorized" || strings.Contains(status, "authorized for")
}

func notActiveAuthorityStatus(status string) bool {
	return strings.Contains(status, "inactive") ||
		strings.Contains(status, "revoked") ||
		strings.Contains(status, "pending") ||
		strings.Contains(status, "not authorized")
}

func currentOOSStatus(status string) bool {
	status = normalizedStatus(status)
	if status == "" {
		return false
	}
	if status == "no" || status == "n" || status == "none" || status == "not found" ||
		hasStatusWord(status, "no") ||
		hasStatusWord(status, "not") ||
		hasStatusWord(status, "none") {
		return false
	}
	return status == "yes" ||
		status == "y" ||
		status == "out" ||
		status == "oos" ||
		strings.Contains(status, "out of service") ||
		strings.Contains(status, "oos order")
}

func staleSMSMonth(raw, observedAt string) bool {
	if blank(raw) {
		return false
	}
	smsMonth, ok := parseSMSMonth(raw)
	if !ok {
		return false
	}
	observed, ok := norm.ParseDate(observedAt)
	if !ok {
		return false
	}
	return smsMonth.Before(observed.AddDate(0, -3, 0))
}

func parseSMSMonth(raw string) (time.Time, bool) {
	value := strings.TrimSpace(raw)
	for _, layout := range []string{"2006-01", "200601", "01/2006", "2006-01-02", time.RFC3339} {
		if t, err := time.Parse(layout, value); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func hasOperationalFootprint(operations domain.Operations) bool {
	return operations.PowerUnits > 0 || operations.Drivers > 0 || len(operations.CargoCarried) > 0 || !blank(operations.MCS150Date)
}

func emptyAddress(address domain.Address) bool {
	return blank(address.Line1) &&
		blank(address.Line2) &&
		blank(address.City) &&
		blank(address.State) &&
		blank(address.PostalCode) &&
		blank(address.Country)
}

func normalizedStatus(s string) string {
	replacer := strings.NewReplacer("_", " ", "-", " ", "/", " ", ":", " ", ";", " ", ",", " ", ".", " ")
	return strings.Join(strings.Fields(strings.ToLower(replacer.Replace(strings.TrimSpace(s)))), " ")
}

func hasStatusWord(status, word string) bool {
	for _, field := range strings.Fields(status) {
		if field == word {
			return true
		}
	}
	return false
}

func blank(s string) bool {
	return strings.TrimSpace(s) == ""
}

func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func profileObservedAt(carrier domain.CarrierProfile, observedAt time.Time) string {
	return evidenceObservedAt(carrier.LocalLastSeenAt, observedAt.UTC().Format(time.RFC3339))
}

func evidenceObservedAt(value, fallback string) string {
	if blank(value) {
		return fallback
	}
	return value
}

func evidenceSource(source string) string {
	if blank(source) {
		return profileSource
	}
	return source
}

func recommendation(score int, flags []domain.RiskFlag) string {
	for _, flag := range flags {
		if flag.Severity == "critical" {
			return "high_priority_manual_review"
		}
	}
	switch {
	case score <= 9:
		return "no_obvious_issue"
	case score <= 24:
		return "monitor"
	case score <= 59:
		return "manual_review_recommended"
	default:
		return "high_priority_manual_review"
	}
}
