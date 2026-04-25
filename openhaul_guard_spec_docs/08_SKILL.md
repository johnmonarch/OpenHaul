<!--
OpenHaul Guard OSS specification bundle
Generated: 2026-04-25
Audience: coding agents and human maintainers
Primary implementation language: Go
-->
# OpenHaul Guard Skill

## Purpose

Use OpenHaul Guard to verify motor carriers, review public-record identity consistency, detect carrier packet mismatches, explain risk flags, and generate carrier onboarding review reports.

## When to use this skill

Use this skill when the user asks to:

- Verify a carrier
- Check an MC number
- Check a USDOT number
- Review a carrier packet
- Investigate potential freight fraud risk
- Compare packet details against public records
- Monitor carrier authority or identity changes
- Generate a compliance note for manual review

## Tooling

Primary CLI:

```bash
ohg
```

Local MCP server:

```bash
ohg mcp serve
```

## Required setup

Before live lookups, run:

```bash
ohg setup
```

Check installation health:

```bash
ohg doctor
```

## Common commands

Lookup by MC:

```bash
ohg carrier lookup --mc 123456 --format json
```

Lookup by USDOT:

```bash
ohg carrier lookup --dot 1234567 --format json
```

Markdown report:

```bash
ohg carrier lookup --mc 123456 --format markdown
```

Force live refresh:

```bash
ohg carrier lookup --mc 123456 --force-refresh --format json
```

Offline lookup:

```bash
ohg carrier lookup --mc 123456 --offline --format json
```

Diff local observations:

```bash
ohg carrier diff --mc 123456 --since 90d --format markdown
```

Watch a carrier:

```bash
ohg watch add --mc 123456 --label active-carriers
ohg watch sync --label active-carriers
```

Packet check:

```bash
ohg packet check ./carrier-packet.pdf --mc 123456 --format markdown
```

## Interpretation rules

Do not say a carrier is fraudulent based only on OpenHaul Guard output.

Use this language:

- `manual review recommended`
- `the public record shows`
- `the packet appears to differ from`
- `OpenHaul Guard flagged`
- `source freshness is`
- `local history starts on`

Avoid this language:

- `fraudulent`
- `blacklisted`
- `do not use`
- `definitely double brokered`
- `scammer`

## Required answer structure for carrier review

When summarizing a lookup, include:

1. Carrier identity
2. Source freshness
3. Highest severity flags
4. Evidence for each important flag
5. What is a public-record fact vs an inference
6. Suggested manual review action

## Example response

```text
Manual review is recommended for MC 123456.

Public-record facts:
- FMCSA resolved MC 123456 to USDOT 1234567.
- The authority record is active as of the latest lookup.
- The local database first observed this carrier on 2026-04-25.

OpenHaul Guard flags:
- VERY_NEW_AUTHORITY: authority appears to be less than 30 days old.
- PACKET_PHONE_MISMATCH: the packet phone number differs from the FMCSA phone number.

Suggested next step:
Confirm contact details through an independent source before tendering high-value freight.
```

## JSON handling

When using `--format json`, parse these fields first:

```text
lookup.resolved_usdot
carrier.legal_name
carrier.identifiers
freshness
risk_assessment.recommendation
risk_assessment.flags
sources
warnings
```

## Failure handling

If the command returns `OHG_AUTH_FMCSA_MISSING`, tell the user:

```text
Live FMCSA lookup needs a free FMCSA WebKey. Run `ohg setup fmcsa` and follow the guided steps.
```

If the command returns `OHG_OFFLINE_CACHE_MISS`, tell the user:

```text
This carrier is not in the local cache. Run without `--offline` after completing setup.
```

If the command returns `OHG_SOURCE_RATE_LIMITED`, tell the user:

```text
The source rate-limited the request. Try again later or configure a Socrata app token if this happened during bulk sync.
```

## Agent caution

OpenHaul Guard is a review aid. Final onboarding, tendering, and fraud decisions must remain with the user's compliance process.
