# OpenHaul Guard Skill

Use OpenHaul Guard to verify motor carriers, review public-record identity consistency, detect carrier packet mismatches, explain risk flags, and generate carrier onboarding review reports.

Primary CLI:

```bash
ohg
```

Common commands:

```bash
ohg doctor
ohg carrier lookup --mc 123456 --format json
ohg carrier lookup --dot 1234567 --format markdown
ohg carrier diff --mc 123456 --since 90d --format markdown
ohg watch add --mc 123456 --label active-carriers
ohg watch sync --label active-carriers
```

Interpretation rules:

- Do not say a carrier is fraudulent based only on OpenHaul Guard output.
- Say `manual review recommended`, `public-record mismatch`, or `risk flag triggered`.
- Always cite evidence, data freshness, and whether the fact came from public records or OpenHaul Guard inference.
