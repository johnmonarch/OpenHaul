<!--
OpenHaul Guard OSS specification bundle
Generated: 2026-04-25
Audience: coding agents and human maintainers
Primary implementation language: Go
-->
# 00 Product Context

## Product

OpenHaul Guard OSS is a free, open-source command-line and local-service tool for carrier verification, carrier identity review, and freight fraud risk triage.

Working binary name: `ohg`

## Primary audience

- Freight brokers
- Small and midsize 3PLs
- Shippers managing carrier networks
- Carrier compliance teams
- Small fleets monitoring their own public identity
- TMS, WMS, and logistics workflow developers
- Agents operating in compliance or onboarding workflows

## Core value

Given an MC, MX, FF, or USDOT number, OpenHaul Guard should return:

1. A normalized carrier profile
2. Public-record source facts
3. Local snapshot history
4. Deterministic risk flags
5. A human-readable explanation
6. JSON and Markdown outputs for automation and agents

## Key design decision

OpenHaul Guard is lazy-first.

The user must not be forced to download every FMCSA dataset before the first carrier lookup.

The first lookup should:

1. Resolve the entered identifier.
2. Fetch current public source data.
3. Normalize and store it locally.
4. Run risk rules.
5. Return a report.

## Local vs hosted positioning

This spec covers the OSS version only.

The OSS version should support:

- Local SQLite database
- Local raw source payload storage
- Lazy live lookups
- Watchlists
- Packet checks
- Local MCP server
- Agent skill instructions
- Optional full sync

The future hosted API will provide:

- Managed freshness
- Historical snapshots before local first-seen dates
- Webhooks
- Batch scale
- Team dashboards
- Support and SLAs

## Product guardrails

OpenHaul Guard is an evidence engine, not a blacklist.

The system must always separate:

- Source facts
- Normalized facts
- Local observations
- Inferences
- Recommendations

## V1 success criteria

A fresh user can do this successfully within five minutes:

```bash
brew install openhaulguard/tap/ohg
ohg setup
ohg carrier lookup --mc 123456 --format markdown
```

The result must include:

- Carrier name
- USDOT number if resolvable
- MC or docket number if available
- Source freshness
- Local first-seen timestamp
- Risk flags with evidence
- Manual review language when appropriate
