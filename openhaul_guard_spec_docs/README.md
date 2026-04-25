<!--
OpenHaul Guard OSS specification bundle
Generated: 2026-04-25
Audience: coding agents and human maintainers
Primary implementation language: Go
-->
# OpenHaul Guard OSS Spec Bundle

This bundle contains implementation-ready specification documents for OpenHaul Guard OSS, a local-first carrier verification and freight fraud risk review tool for the trucking industry.

The specs are written for an implementation agent. They prioritize exact commands, schemas, file paths, acceptance criteria, and staged build tasks over marketing language.

## Document order

1. `00_PRODUCT_CONTEXT.md`
2. `01_ENGINEERING_SPEC.md`
3. `02_CLI_SPEC.md`
4. `03_DATA_SOURCES_AND_INGESTION.md`
5. `04_DATABASE_SCHEMA.md`
6. `05_SCORING_RULES.md`
7. `06_REPORTING_SPEC.md`
8. `07_MCP_SPEC.md`
9. `08_SKILL.md`
10. `09_TESTING_AND_ACCEPTANCE.md`
11. `10_PACKAGING_AND_RELEASE.md`
12. `11_AGENT_BUILD_PLAN.md`

## Build principle

The first usable version must not require full data ingestion. A user should be able to install the binary, run `ohg setup`, paste a required key if needed, and perform a first MC lookup.

```bash
ohg setup
ohg carrier lookup --mc 123456
```

## Source of truth order

When documents conflict, use this order:

1. `01_ENGINEERING_SPEC.md`
2. `03_DATA_SOURCES_AND_INGESTION.md`
3. `04_DATABASE_SCHEMA.md`
4. `02_CLI_SPEC.md`
5. Other supporting docs

## Non-negotiable language rule

The tool must not label a carrier as fraudulent. It may say:

- `manual review recommended`
- `evidence of mismatch`
- `recent public-record change`
- `identity consistency issue`
- `risk flag triggered`

It must not say:

- `this carrier is fraud`
- `blacklisted`
- `do not use`
- `definitely double brokered`
