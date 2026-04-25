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

func Assess(carrier domain.CarrierProfile, ctx Context) domain.RiskAssessment {
	if ctx.ObservedAt.IsZero() {
		ctx.ObservedAt = time.Now().UTC()
	}
	var flags []domain.RiskFlag
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
	flags = append(flags, authorityFlags(carrier, ctx.ObservedAt)...)
	if strings.Contains(strings.ToLower(carrier.Safety.OutOfServiceStatus), "out") {
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
				ObservedAt: carrier.LocalLastSeenAt,
			}},
		})
	}
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

func authorityFlags(carrier domain.CarrierProfile, observedAt time.Time) []domain.RiskFlag {
	var flags []domain.RiskFlag
	for _, authority := range carrier.Authority {
		status := strings.ToLower(authority.AuthorityStatus)
		if status != "" && (strings.Contains(status, "inactive") || strings.Contains(status, "revoked") || strings.Contains(status, "pending") || strings.Contains(status, "not authorized")) {
			flags = append(flags, domain.RiskFlag{
				Code:         "AUTHORITY_NOT_ACTIVE",
				Severity:     "critical",
				Category:     "authority",
				Explanation:  "Operating authority is not active according to the source record.",
				WhyItMatters: "Inactive, revoked, pending, or not-authorized authority requires compliance review before use.",
				NextStep:     "Check the current authority record directly and confirm the required operation type.",
				Confidence:   "high",
				Evidence: []domain.Evidence{{
					Field:      "authority.authority_status",
					Value:      authority.AuthorityStatus,
					Source:     authority.Source,
					ObservedAt: authority.ObservedAt,
				}},
			})
		}
		if authority.OriginalActionDate == "" {
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
		if ageDays < 30 {
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
					Value:      authority.OriginalActionDate,
					Source:     authority.Source,
					ObservedAt: authority.ObservedAt,
				}},
			})
		} else if ageDays < 90 {
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
					Value:      authority.OriginalActionDate,
					Source:     authority.Source,
					ObservedAt: authority.ObservedAt,
				}},
			})
		}
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
