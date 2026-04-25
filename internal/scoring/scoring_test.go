package scoring

import (
	"testing"
	"time"

	"github.com/openhaulguard/openhaulguard/internal/domain"
)

func TestVeryNewAuthoritySuppressesNewAuthorityByConstruction(t *testing.T) {
	carrier := domain.CarrierProfile{
		USDOTNumber:      "1234567",
		LocalFirstSeenAt: "2026-04-25T00:00:00Z",
		LocalLastSeenAt:  "2026-04-25T00:00:00Z",
		Authority: []domain.AuthorityRecord{{
			AuthorityStatus:    "ACTIVE",
			OriginalActionDate: "2026-04-01",
			Source:             "fixture",
			ObservedAt:         "2026-04-25T00:00:00Z",
		}},
	}
	assessment := Assess(carrier, Context{ObservationCount: 1, ObservedAt: time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC)})
	if !hasFlag(assessment, "VERY_NEW_AUTHORITY") {
		t.Fatalf("expected VERY_NEW_AUTHORITY flag: %#v", assessment.Flags)
	}
	if hasFlag(assessment, "NEW_AUTHORITY") {
		t.Fatalf("did not expect NEW_AUTHORITY when VERY_NEW_AUTHORITY fires: %#v", assessment.Flags)
	}
}

func TestAuthorityNotActiveIsCritical(t *testing.T) {
	carrier := domain.CarrierProfile{
		USDOTNumber:      "1234567",
		LocalFirstSeenAt: "2026-04-25T00:00:00Z",
		LocalLastSeenAt:  "2026-04-25T00:00:00Z",
		Authority: []domain.AuthorityRecord{{
			AuthorityStatus: "REVOKED",
			Source:          "fixture",
			ObservedAt:      "2026-04-25T00:00:00Z",
		}},
	}
	assessment := Assess(carrier, Context{ObservationCount: 2, ObservedAt: time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC)})
	if !hasFlag(assessment, "AUTHORITY_NOT_ACTIVE") {
		t.Fatalf("expected AUTHORITY_NOT_ACTIVE flag: %#v", assessment.Flags)
	}
	if assessment.Recommendation != "high_priority_manual_review" {
		t.Fatalf("recommendation = %q", assessment.Recommendation)
	}
}

func hasFlag(assessment domain.RiskAssessment, code string) bool {
	for _, flag := range assessment.Flags {
		if flag.Code == code {
			return true
		}
	}
	return false
}
