package scoring

import (
	"testing"
	"time"

	"github.com/openhaulguard/openhaulguard/internal/domain"
)

func TestVeryNewAuthoritySuppressesNewAuthorityByConstruction(t *testing.T) {
	carrier := baselineCarrier()
	carrier.Authority = append(carrier.Authority, domain.AuthorityRecord{
		DocketType:         "MC",
		DocketNumber:       "123456",
		AuthorityType:      "COMMON",
		AuthorityStatus:    "ACTIVE",
		OriginalActionDate: "2026-02-15",
		Source:             "fixture",
		ObservedAt:         "2026-04-25T00:00:00Z",
	})
	carrier.Authority[0] = domain.AuthorityRecord{
		DocketType:         "MC",
		DocketNumber:       "123456",
		AuthorityType:      "COMMON",
		AuthorityStatus:    "ACTIVE",
		OriginalActionDate: "2026-04-01",
		Source:             "fixture",
		ObservedAt:         "2026-04-25T00:00:00Z",
	}
	assessment := Assess(carrier, testContext())
	if countFlag(assessment, "VERY_NEW_AUTHORITY") != 1 {
		t.Fatalf("expected one VERY_NEW_AUTHORITY flag: %#v", assessment.Flags)
	}
	if hasFlag(assessment, "NEW_AUTHORITY") {
		t.Fatalf("did not expect NEW_AUTHORITY when VERY_NEW_AUTHORITY fires: %#v", assessment.Flags)
	}
	if assessment.Score != 30 || assessment.Recommendation != "manual_review_recommended" {
		t.Fatalf("score/recommendation = %d/%q", assessment.Score, assessment.Recommendation)
	}
}

func TestAuthorityNotActiveIsCritical(t *testing.T) {
	carrier := baselineCarrier()
	carrier.Authority = []domain.AuthorityRecord{{
		AuthorityStatus: "REVOKED",
		Source:          "fixture",
		ObservedAt:      "2026-04-25T00:00:00Z",
	}}
	assessment := Assess(carrier, testContext())
	if !hasFlag(assessment, "AUTHORITY_NOT_ACTIVE") {
		t.Fatalf("expected AUTHORITY_NOT_ACTIVE flag: %#v", assessment.Flags)
	}
	if hasFlag(assessment, "ACTIVE_AUTHORITY_NOT_FOUND") {
		t.Fatalf("did not expect overlapping ACTIVE_AUTHORITY_NOT_FOUND flag: %#v", assessment.Flags)
	}
	if assessment.Score != 60 {
		t.Fatalf("score = %d", assessment.Score)
	}
	if assessment.Recommendation != "high_priority_manual_review" {
		t.Fatalf("recommendation = %q", assessment.Recommendation)
	}
}

func TestMissingIdentityAndContactFlagsUseEvidence(t *testing.T) {
	carrier := baselineCarrier()
	carrier.USDOTNumber = " "
	carrier.LegalName = ""
	carrier.PhysicalAddress = domain.Address{}
	carrier.Contact.Phone = " "

	assessment := Assess(carrier, testContext())
	for _, code := range []string{"MISSING_USDOT", "MISSING_LEGAL_NAME", "MISSING_PHYSICAL_ADDRESS", "MISSING_PHONE"} {
		flag := flagByCode(assessment, code)
		if flag == nil {
			t.Fatalf("expected %s flag: %#v", code, assessment.Flags)
		}
		if len(flag.Evidence) == 0 {
			t.Fatalf("expected %s to include evidence", code)
		}
	}
	if assessment.Score != 65 || assessment.Recommendation != "high_priority_manual_review" {
		t.Fatalf("score/recommendation = %d/%q", assessment.Score, assessment.Recommendation)
	}
}

func TestIdentifierMismatchFlagsUseProfileEvidence(t *testing.T) {
	carrier := baselineCarrier()
	carrier.Identifiers = []domain.Identifier{
		{Type: "USDOT", Value: "7654321"},
		{Type: "MC", Value: "123456"},
		{Type: "MC", Value: "654321"},
	}

	assessment := Assess(carrier, testContext())
	for _, code := range []string{"USDOT_IDENTIFIER_MISMATCH", "IDENTIFIER_VALUE_CONFLICT"} {
		flag := flagByCode(assessment, code)
		if flag == nil {
			t.Fatalf("expected %s flag: %#v", code, assessment.Flags)
		}
		if len(flag.Evidence) == 0 {
			t.Fatalf("expected %s to include evidence", code)
		}
	}
	if assessment.Score != 45 || assessment.Recommendation != "manual_review_recommended" {
		t.Fatalf("score/recommendation = %d/%q", assessment.Score, assessment.Recommendation)
	}
}

func TestMissingActiveAuthorityWhenRecordsExist(t *testing.T) {
	carrier := baselineCarrier()
	carrier.Authority = []domain.AuthorityRecord{{
		AuthorityType:   "COMMON",
		AuthorityStatus: "NONE",
		Source:          "fixture",
		ObservedAt:      "2026-04-25T00:00:00Z",
	}}

	assessment := Assess(carrier, testContext())
	if !hasFlag(assessment, "ACTIVE_AUTHORITY_NOT_FOUND") {
		t.Fatalf("expected ACTIVE_AUTHORITY_NOT_FOUND flag: %#v", assessment.Flags)
	}
	if hasFlag(assessment, "AUTHORITY_NOT_ACTIVE") {
		t.Fatalf("did not expect AUTHORITY_NOT_ACTIVE for NONE status: %#v", assessment.Flags)
	}
	if assessment.Score != 30 || assessment.Recommendation != "manual_review_recommended" {
		t.Fatalf("score/recommendation = %d/%q", assessment.Score, assessment.Recommendation)
	}
}

func TestAuthorityRecordGapsAndDocketMismatches(t *testing.T) {
	t.Run("missing authority records", func(t *testing.T) {
		carrier := baselineCarrier()
		carrier.Authority = nil

		assessment := Assess(carrier, testContext())
		if !hasFlag(assessment, "AUTHORITY_RECORDS_MISSING") {
			t.Fatalf("expected AUTHORITY_RECORDS_MISSING flag: %#v", assessment.Flags)
		}
		if assessment.Score != 15 || assessment.Recommendation != "monitor" {
			t.Fatalf("score/recommendation = %d/%q", assessment.Score, assessment.Recommendation)
		}
	})

	t.Run("docket mismatch", func(t *testing.T) {
		carrier := baselineCarrier()
		carrier.Authority[0].DocketNumber = "999999"

		assessment := Assess(carrier, testContext())
		if !hasFlag(assessment, "AUTHORITY_DOCKET_MISMATCH") {
			t.Fatalf("expected AUTHORITY_DOCKET_MISMATCH flag: %#v", assessment.Flags)
		}
		if assessment.Score != 30 || assessment.Recommendation != "manual_review_recommended" {
			t.Fatalf("score/recommendation = %d/%q", assessment.Score, assessment.Recommendation)
		}
	})

	t.Run("active authority type missing", func(t *testing.T) {
		carrier := baselineCarrier()
		carrier.Authority[0].AuthorityType = ""

		assessment := Assess(carrier, testContext())
		if !hasFlag(assessment, "ACTIVE_AUTHORITY_TYPE_MISSING") {
			t.Fatalf("expected ACTIVE_AUTHORITY_TYPE_MISSING flag: %#v", assessment.Flags)
		}
		if assessment.Score != 5 || assessment.Recommendation != "no_obvious_issue" {
			t.Fatalf("score/recommendation = %d/%q", assessment.Score, assessment.Recommendation)
		}
	})
}

func TestOperationsFlagsAffectMonitorRecommendation(t *testing.T) {
	carrier := baselineCarrier()
	carrier.Operations.PowerUnits = 0
	carrier.Operations.Drivers = 0
	carrier.Operations.MCS150Date = "2024-04-24"

	assessment := Assess(carrier, testContext())
	if !hasFlag(assessment, "ZERO_POWER_UNITS_OR_DRIVERS") {
		t.Fatalf("expected ZERO_POWER_UNITS_OR_DRIVERS flag: %#v", assessment.Flags)
	}
	if !hasFlag(assessment, "STALE_MCS150") {
		t.Fatalf("expected STALE_MCS150 flag: %#v", assessment.Flags)
	}
	if countFlag(assessment, "ZERO_POWER_UNITS_OR_DRIVERS") != 1 {
		t.Fatalf("expected a single operations count flag: %#v", assessment.Flags)
	}
	if assessment.Score != 20 || assessment.Recommendation != "monitor" {
		t.Fatalf("score/recommendation = %d/%q", assessment.Score, assessment.Recommendation)
	}
}

func TestOperationClassificationAndEmailGapsAffectMonitorRecommendation(t *testing.T) {
	carrier := baselineCarrier()
	carrier.Operations.OperationClassification = nil
	carrier.Contact.Email = ""

	assessment := Assess(carrier, testContext())
	if !hasFlag(assessment, "OPERATION_CLASSIFICATION_MISSING") {
		t.Fatalf("expected OPERATION_CLASSIFICATION_MISSING flag: %#v", assessment.Flags)
	}
	if !hasFlag(assessment, "MISSING_EMAIL") {
		t.Fatalf("expected MISSING_EMAIL flag: %#v", assessment.Flags)
	}
	if assessment.Score != 10 || assessment.Recommendation != "monitor" {
		t.Fatalf("score/recommendation = %d/%q", assessment.Score, assessment.Recommendation)
	}
}

func TestMCS150BoundaryIsNotStaleAtTwoYears(t *testing.T) {
	carrier := baselineCarrier()
	carrier.Operations.MCS150Date = "2024-04-25"

	assessment := Assess(carrier, testContext())
	if hasFlag(assessment, "STALE_MCS150") {
		t.Fatalf("did not expect STALE_MCS150 at the two-year boundary: %#v", assessment.Flags)
	}
}

func TestStaleSMSDataIsInformational(t *testing.T) {
	carrier := baselineCarrier()
	carrier.Safety.SMSMonth = "2025-12"

	assessment := Assess(carrier, testContext())
	if !hasFlag(assessment, "SMS_DATA_STALE") {
		t.Fatalf("expected SMS_DATA_STALE flag: %#v", assessment.Flags)
	}
	if assessment.Score != 0 || assessment.Recommendation != "no_obvious_issue" {
		t.Fatalf("score/recommendation = %d/%q", assessment.Score, assessment.Recommendation)
	}
}

func TestOOSStatusIsCriticalButNoStatusDoesNotFlag(t *testing.T) {
	t.Run("current", func(t *testing.T) {
		carrier := baselineCarrier()
		for _, status := range []string{"Out", "Out of Service"} {
			carrier.Safety.OutOfServiceStatus = status

			assessment := Assess(carrier, testContext())
			if !hasFlag(assessment, "OOS_CURRENT") {
				t.Fatalf("expected OOS_CURRENT flag for %q: %#v", status, assessment.Flags)
			}
			if assessment.Score != 60 || assessment.Recommendation != "high_priority_manual_review" {
				t.Fatalf("score/recommendation for %q = %d/%q", status, assessment.Score, assessment.Recommendation)
			}
		}
	})

	t.Run("not current", func(t *testing.T) {
		carrier := baselineCarrier()
		for _, status := range []string{"Not Out of Service", "OOS order not found"} {
			carrier.Safety.OutOfServiceStatus = status

			assessment := Assess(carrier, testContext())
			if hasFlag(assessment, "OOS_CURRENT") {
				t.Fatalf("did not expect OOS_CURRENT flag for %q: %#v", status, assessment.Flags)
			}
			if assessment.Score != 0 || assessment.Recommendation != "no_obvious_issue" {
				t.Fatalf("score/recommendation for %q = %d/%q", status, assessment.Score, assessment.Recommendation)
			}
		}
	})
}

func hasFlag(assessment domain.RiskAssessment, code string) bool {
	return flagByCode(assessment, code) != nil
}

func flagByCode(assessment domain.RiskAssessment, code string) *domain.RiskFlag {
	for i := range assessment.Flags {
		if assessment.Flags[i].Code == code {
			return &assessment.Flags[i]
		}
	}
	return nil
}

func countFlag(assessment domain.RiskAssessment, code string) int {
	count := 0
	for _, flag := range assessment.Flags {
		if flag.Code == code {
			count++
		}
	}
	return count
}

func testContext() Context {
	return Context{ObservationCount: 2, ObservedAt: time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC)}
}

func baselineCarrier() domain.CarrierProfile {
	return domain.CarrierProfile{
		USDOTNumber:      "1234567",
		LegalName:        "Example Carrier LLC",
		LocalFirstSeenAt: "2026-04-20T00:00:00Z",
		LocalLastSeenAt:  "2026-04-25T00:00:00Z",
		PhysicalAddress: domain.Address{
			Line1:      "123 Main St",
			City:       "Austin",
			State:      "TX",
			PostalCode: "78701",
			Country:    "US",
		},
		Operations: domain.Operations{
			PowerUnits:              1,
			Drivers:                 1,
			OperationClassification: []string{"interstate"},
			MCS150Date:              "2026-01-15",
		},
		Contact: domain.Contact{Phone: "+15555555555", Email: "dispatch@example.test"},
		Identifiers: []domain.Identifier{{
			Type:   "MC",
			Value:  "123456",
			Status: "active",
		}},
		Authority: []domain.AuthorityRecord{{
			DocketType:         "MC",
			DocketNumber:       "123456",
			AuthorityType:      "COMMON",
			AuthorityStatus:    "ACTIVE",
			OriginalActionDate: "2025-01-01",
			Source:             "fixture",
			ObservedAt:         "2026-04-25T00:00:00Z",
		}},
	}
}
