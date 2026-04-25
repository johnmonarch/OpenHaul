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
	return flags
}

func authorityFlags(carrier domain.CarrierProfile, observedAt time.Time, fallbackObservedAt string) []domain.RiskFlag {
	var flags []domain.RiskFlag
	var notActiveEvidence []domain.Evidence
	var noActiveEvidence []domain.Evidence
	hasStatus := false
	hasActive := false

	for _, authority := range carrier.Authority {
		status := normalizedStatus(authority.AuthorityStatus)
		if status == "" {
			continue
		}
		hasStatus = true
		if activeAuthorityStatus(status) {
			hasActive = true
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
	return flags
}

func safetyFlags(carrier domain.CarrierProfile, observedAt string) []domain.RiskFlag {
	if !currentOOSStatus(carrier.Safety.OutOfServiceStatus) {
		return nil
	}
	return []domain.RiskFlag{{
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
	}}
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

func authorityEvidence(authority domain.AuthorityRecord, fallbackObservedAt string) domain.Evidence {
	return domain.Evidence{
		Field:      "authority.authority_status",
		Value:      authority.AuthorityStatus,
		Source:     evidenceSource(authority.Source),
		ObservedAt: evidenceObservedAt(authority.ObservedAt, fallbackObservedAt),
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
