<!--
OpenHaul Guard OSS specification bundle
Generated: 2026-04-25
Audience: coding agents and human maintainers
Primary implementation language: Go
-->
# 06 Reporting Specification

## 1. Supported formats

Required:

- table
- json
- markdown

Post-MVP:

- html
- pdf

## 2. Report types

```text
carrier_lookup_report
carrier_diff_report
watch_sync_report
packet_check_report
risk_assessment_report
```

## 3. Carrier lookup Markdown report

Template:

```markdown
# OpenHaul Guard Carrier Report

Generated: {timestamp}
Lookup input: {input_type} {input_value}
Resolved USDOT: {usdot_number}
Mode: {live|cache|offline|mirror}

## Carrier

| Field | Value |
|---|---|
| Legal name | {legal_name} |
| DBA | {dba_name} |
| USDOT | {usdot_number} |
| Docket numbers | {identifiers} |
| Authority status | {authority_status} |
| Physical address | {physical_address} |
| Phone | {phone} |
| Power units | {power_units} |
| Drivers | {drivers} |

## Data Freshness

| Source | Freshness | Notes |
|---|---|---|
| FMCSA QCMobile | {fetched_at} | Live lookup |
| DOT DataHub Company Census | {source_date} | Daily source, not real time |
| SMS | {sms_month} | Monthly source |

## Risk Review

Recommendation: {recommendation}
Score: {score}/100

### Flags

{flags}

## Source Facts vs Inferences

Source facts are values returned by public sources. Risk flags are OpenHaul Guard interpretations intended for manual review.

## Disclaimer

OpenHaul Guard does not label carriers as fraudulent and does not make tendering decisions. Use this report as part of a human compliance process.
```

## 4. Flag Markdown block

```markdown
### {severity}: {code}

What we found: {explanation}
Why it matters: {why_it_matters}
Suggested next step: {next_step}

Evidence:

| Field | Value | Source | Observed |
|---|---|---|---|
| {field} | {value} | {source} | {observed_at} |
```

## 5. JSON report

JSON must be stable and schema-versioned.

Top-level:

```json
{
  "schema_version": "1.0",
  "report_type": "carrier_lookup_report",
  "generated_at": "2026-04-25T18:30:00Z",
  "lookup": {},
  "carrier": {},
  "freshness": {},
  "risk_assessment": {},
  "sources": [],
  "warnings": [],
  "disclaimer": "OpenHaul Guard does not label carriers as fraudulent and does not make tendering decisions."
}
```

## 6. Table output

Table output is for quick human terminal reading.

Table output must show:

- Carrier name
- USDOT
- MC/MX/FF identifiers
- Authority status
- Data freshness
- Recommendation
- Highest severity flag
- Count of flags by severity

Do not try to show every evidence item in default table output. Suggest Markdown or JSON for full details.

## 7. Diff report

Required sections:

- Carrier identity
- Date range
- Observation count
- Material changes
- Non-material changes if `--strict`
- Related risk flags

Material fields:

```text
legal_name
dba_name
physical_address
mailing_address
phone
email
authority.status
authority.type
insurance.status
insurance.policy_number
boc3.agent_name
power_units
drivers
```

## 8. Packet check report

Required sections:

- Packet path and hash
- Carrier lookup identity
- Extracted fields
- Public-record comparison
- Mismatches
- Risk flags
- Manual review recommendation

Comparison statuses:

```text
match
fuzzy_match
missing_in_packet
missing_in_public_record
conflict
not_checked
```

## 9. Redaction

Default reports may show public-record fields. Packet-origin fields should be included in local reports but not uploaded anywhere.

For sharing:

```bash
ohg carrier lookup --mc 123456 --format markdown --redact contact
```

Redaction profiles:

```text
none
contact
packet_sensitive
all_sensitive
```

MVP redaction can be limited to contact and packet-sensitive fields.
