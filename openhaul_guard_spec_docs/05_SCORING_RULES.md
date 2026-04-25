<!--
OpenHaul Guard OSS specification bundle
Generated: 2026-04-25
Audience: coding agents and human maintainers
Primary implementation language: Go
-->
# 05 Scoring Rules Specification

## 1. Purpose

OpenHaul Guard scoring is a deterministic rules engine. It exists to surface evidence-backed review flags, not to declare fraud.

## 2. Output language rules

Allowed recommendation values:

```text
no_obvious_issue
monitor
manual_review_recommended
high_priority_manual_review
insufficient_data
```

Disallowed output language:

```text
fraudulent
blacklisted
do not use
guaranteed fraud
definite double broker
```

## 3. Severity levels

```text
info
low
medium
high
critical
```

Critical should be rare and reserved for objective facts such as revoked authority or out-of-service status when supported by source data.

## 4. Rule object format

Rules may be defined in YAML or JSON. YAML is preferred for human editing.

```yaml
code: NEW_AUTHORITY
version: 1
category: authority
severity: medium
enabled: true
condition:
  type: days_since_less_than
  field: authority.first_active_date
  days: 90
explanation: Operating authority appears to be less than 90 days old.
recommendation: manual_review_recommended
confidence: medium
evidence_fields:
  - authority.first_active_date
  - authority.status
```

## 5. Risk score

The score is secondary to the flags.

Suggested scoring:

```text
info = 0
low = 5
medium = 15
high = 30
critical = 60
```

Cap total score at 100.

Recommendation mapping:

```text
0 to 9: no_obvious_issue
10 to 24: monitor
25 to 59: manual_review_recommended
60 to 100: high_priority_manual_review
```

If source data is incomplete, `insufficient_data` may override score-based recommendation.

## 6. MVP rules

### RULE-001 NO_LOCAL_HISTORY

Severity: info

Trigger:

- Carrier has only one local observation.

Explanation:

```text
This is the first local observation for this carrier. Local change detection starts now.
```

### RULE-002 NEW_AUTHORITY

Severity: medium

Trigger:

- Authority first active date is known and less than 90 days old.

Evidence:

- Authority status
- Original action date
- Source

### RULE-003 VERY_NEW_AUTHORITY

Severity: high

Trigger:

- Authority first active date is known and less than 30 days old.

Do not also emit `NEW_AUTHORITY` if this rule fires.

### RULE-004 AUTHORITY_NOT_ACTIVE

Severity: critical

Trigger:

- Current authority status is inactive, revoked, pending, or not authorized for required operation type.

Recommendation:

`high_priority_manual_review`

### RULE-005 RECENT_AUTHORITY_CHANGE

Severity: high

Trigger:

- Local diff or upstream history indicates authority status changed within 30 days.

### RULE-006 RECENT_NAME_CHANGE

Severity: high

Trigger:

- Local diff or upstream history indicates legal name changed within 30 days.

### RULE-007 RECENT_ADDRESS_CHANGE

Severity: high

Trigger:

- Local diff indicates physical address changed within 30 days.

### RULE-008 RECENT_PHONE_CHANGE

Severity: medium

Trigger:

- Local diff indicates phone changed within 30 days.

### RULE-009 PACKET_LEGAL_NAME_MISMATCH

Severity: high

Trigger:

- Packet legal name does not fuzzy-match public-record legal name or DBA.

### RULE-010 PACKET_DOT_MISMATCH

Severity: critical

Trigger:

- Packet contains a USDOT number that differs from the lookup carrier USDOT.

### RULE-011 PACKET_DOCKET_MISMATCH

Severity: critical

Trigger:

- Packet contains an MC/MX/FF number that maps to a different USDOT than the lookup carrier.

### RULE-012 PACKET_PHONE_MISMATCH

Severity: medium

Trigger:

- Packet phone differs from public-record phone after normalization.

### RULE-013 PACKET_EMAIL_DOMAIN_SUSPICIOUS

Severity: medium

Trigger:

- Packet email domain is a public/free email provider or does not match known company website domain if website data is later available.

MVP note:

Only fire this as medium and do not overstate. Many legitimate small carriers use public email providers.

### RULE-014 INSURANCE_RECENTLY_CANCELLED

Severity: high

Trigger:

- Insurance history indicates cancellation effective within last 30 days without evidence of replacement.

### RULE-015 REJECTED_INSURANCE_FORM_RECENT

Severity: medium

Trigger:

- Rejected insurance filing within last 90 days.

### RULE-016 BOC3_MISSING_OR_UNCONFIRMED

Severity: medium

Trigger:

- BOC-3 record is unavailable when authority type normally requires it.

### RULE-017 OOS_CURRENT

Severity: critical

Trigger:

- Current out-of-service order is present in source data.

### RULE-018 SMS_DATA_STALE

Severity: info

Trigger:

- Latest SMS data is older than expected monthly cadence plus buffer.

### RULE-019 LOW_ACTIVITY_PROFILE

Severity: low

Trigger:

- No recent inspections or very low public activity for a carrier claiming substantial operations.

MVP note:

This rule should be conservative. Public records may lag or be incomplete.

## 7. Rule suppression

Some rules supersede others.

Examples:

```text
VERY_NEW_AUTHORITY suppresses NEW_AUTHORITY
PACKET_DOT_MISMATCH suppresses PACKET_DOCKET_MISMATCH only if same evidence path
AUTHORITY_NOT_ACTIVE supersedes NEW_AUTHORITY for recommendation mapping
```

## 8. Evidence requirements

Every flag must include at least one evidence object:

```json
{
  "field": "carrier.phone",
  "source_value": "+15555555555",
  "comparison_value": "+15555550123",
  "source": "packet_check",
  "observed_at": "2026-04-25T18:30:00Z"
}
```

## 9. User-facing explanation pattern

Use this pattern:

```text
What we found: [plain-English source fact]
Why it matters: [review reason]
Suggested next step: [manual action]
```

Example:

```text
What we found: The phone number in the packet does not match the phone number returned by FMCSA.
Why it matters: Contact mismatches can be benign, but they are worth checking before tendering freight.
Suggested next step: Confirm the phone number through an independent source before proceeding.
```

## 10. Testing rules

Each rule must have:

- Positive fixture
- Negative fixture
- Boundary fixture when dates are involved
- Markdown snapshot test
- JSON snapshot test
