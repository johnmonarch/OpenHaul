# OpenHaul Guard OSS PRD

**Product:** OpenHaul Guard OSS  
**Artifact:** Product Requirements Document  
**Version:** 0.1 draft  
**Date:** 2026-04-25  
**Primary mode:** Local-first CLI with optional MCP server and agent skill  
**License target:** Apache-2.0 or AGPL-3.0, final choice TBD  
**Working binary name:** `ohg`

---

## 1. Executive Summary

OpenHaul Guard OSS is a free, open-source carrier verification and freight fraud risk-assessment tool for the trucking industry. The OSS product is designed to work locally, from a terminal, without requiring a hosted SaaS account. A user can enter an MC, MX, FF, or USDOT number and receive a normalized carrier profile, current public-record status, evidence-backed risk flags, and a local point-in-time snapshot for future diffing.

The core design principle is **lazy-first ingestion**:

```bash
ohg carrier lookup --mc 123456
```

The tool should perform the lookup, normalize the public data, store raw and normalized snapshots locally, compute risk flags, and return a human-readable report. Users should not need to download the entire FMCSA universe before checking a carrier.

Power users can optionally run watchlists, scheduled syncs, packet checks, full data syncs, and an MCP server for agentic workflows.

---

## 2. Problem Statement

The trucking industry has a trust problem: fraud, double brokering, spoofed identities, stolen MC/DOT identities, mismatched carrier packets, and sudden changes to authority/contact/insurance data. Many organizations rely on manual checks across FMCSA, SAFER, Licensing & Insurance pages, carrier packets, email threads, and spreadsheets.

Existing commercial tools may be useful, but they are generally closed, expensive, and opaque. Smaller brokers, small fleets, and independent operators need transparent tooling that can run locally and explain why a carrier deserves manual review.

OpenHaul Guard OSS should become the open carrier verification engine: transparent, local-first, scriptable, and agent-friendly.

---

## 3. Goals

### 3.1 Product Goals

1. Let a user verify a carrier from the terminal using MC/MX/FF/USDOT.
2. Resolve MC/MX/FF docket numbers to USDOT numbers.
3. Fetch current public carrier data from official sources where possible.
4. Normalize public data into a stable local schema.
5. Store raw source payloads and normalized snapshots locally.
6. Detect local changes over time.
7. Produce explainable, deterministic risk flags.
8. Support JSON, Markdown, table, and PDF/HTML report output.
9. Provide a local MCP server so agents can call carrier verification tools.
10. Include a `SKILL.md` so coding/workflow agents can reliably use the CLI and interpret results.
11. Minimize registration/API-key friction for new users.

### 3.2 Business/Community Goals

1. Become the credibility layer for a future hosted API.
2. Encourage adoption by compliance teams, brokers, 3PLs, TMS vendors, and small fleets.
3. Allow carriers to self-monitor their own public identity.
4. Avoid black-box scoring and avoid positioning the tool as a blacklist.
5. Make it easy for developers to add new connectors and rules.

---

## 4. Non-Goals

1. Do not declare carriers “fraudulent.”
2. Do not create a blacklist.
3. Do not make automated tender/no-tender decisions.
4. Do not scrape sites aggressively or violate terms of service.
5. Do not require full FMCSA data ingestion before the first useful lookup.
6. Do not include proprietary data in the default OSS distribution.
7. Do not expose private user lookup history unless the user chooses to share/export it.
8. Do not attempt to replace a human compliance process.

---

## 5. Target Users

### 5.1 Primary Users

- Freight brokers onboarding a carrier.
- Small and mid-sized 3PLs.
- Carrier compliance teams.
- Shippers with direct carrier networks.
- Small fleets monitoring their public identity.
- Developers building TMS/onboarding workflows.

### 5.2 Secondary Users

- Researchers analyzing public carrier data.
- Insurance/compliance consultants.
- Logistics-focused agent/workflow builders.
- Open-source contributors.

---

## 6. Core User Stories

### 6.1 Carrier Lookup

As a broker, I want to enter an MC number and get a carrier profile with evidence-backed risk notes so that I can decide whether manual review is needed.

```bash
ohg carrier lookup --mc 123456
```

Acceptance criteria:

- Resolves MC number to USDOT number when possible.
- Fetches current carrier data.
- Stores a local snapshot.
- Returns a readable report.
- Supports `--format json`, `--format markdown`, and `--format table`.

### 6.2 Local History

As a compliance user, I want every lookup to store a local snapshot so that I can see what changed since I first checked the carrier.

```bash
ohg carrier diff --mc 123456 --since 90d
```

Acceptance criteria:

- Shows changed fields.
- Shows prior and current values.
- Shows observed timestamps.
- Separates local observation history from upstream historical public datasets.

### 6.3 Watchlist

As a broker, I want to monitor carriers I use frequently so that I can be alerted when public records change.

```bash
ohg watch add --mc 123456
ohg watch sync
ohg watch report
```

Acceptance criteria:

- Watchlist entries can be added by MC/MX/FF/USDOT.
- Sync refreshes records.
- Risk flags are re-run.
- Diffs are stored.
- Alert output is available as table, JSON, Markdown, email script hook, and webhook script hook.

### 6.4 Packet Check

As an onboarding user, I want to compare a carrier packet against public records so that mismatches are visible before tendering freight.

```bash
ohg packet check ./carrier-packet.pdf --mc 123456
```

Acceptance criteria:

- Extracts legal name, DBA, address, phone, email, insurance names, policy numbers when possible.
- Compares extracted fields to public records.
- Marks each mismatch as exact, fuzzy, missing, or conflicting.
- Does not claim fraud solely from a mismatch.
- Provides a manual review recommendation.

### 6.5 Agent Use

As an agent workflow builder, I want a local MCP server so that an LLM agent can verify a carrier and generate a compliance note.

```bash
ohg mcp serve
```

Acceptance criteria:

- Provides MCP tools for carrier lookup, risk explanation, packet check, watchlist management, and report generation.
- Supports local-only operation.
- Can be configured to use hosted API mode later without changing the agent interface.

---

## 7. UX Principles

1. **Useful in five minutes.**
2. **No full sync required.**
3. **Every risk flag must cite evidence.**
4. **Always separate source facts from OpenHaul Guard inferences.**
5. **Make data freshness obvious.**
6. **Never hide behind a score.**
7. **Default to conservative language.**
8. **Support power users without burdening first-time users.**

---

## 8. Onboarding and Idiot-Proof Setup

OpenHaul Guard must treat onboarding as a core product feature. The target user may be comfortable enough to open a terminal, but should not need to know what FMCSA QCMobile, Socrata, SODA, or WebKeys are before getting value.

The preferred first-run command is:

```bash
ohg setup
```

The setup wizard should be interactive, forgiving, and resumable. It should explain what the user needs, open the required web page when possible, pause while the user creates the key, accept pasted credentials through a masked prompt, validate the credentials immediately, and store them securely.

Design goal:

```text
A non-technical trucking operator should be able to install the tool, press Enter a few times, paste keys when asked, and complete a first carrier lookup without reading external documentation.
```

---

### 8.1 Setup Modes

The CLI must support three setup paths.

#### 8.1.1 Quick Setup, No Keys

```bash
ohg setup --quick
```

Purpose: fastest possible path to first lookup.

Behavior:

1. Creates local config and SQLite database.
2. Downloads the OpenHaul public mirror/bootstrap index if available.
3. Skips FMCSA WebKey setup.
4. Skips Socrata app token setup.
5. Runs `ohg doctor`.
6. Suggests a first command:

```bash
ohg carrier lookup --mc 123456
```

Expected terminal copy:

```text
Welcome to OpenHaul Guard.

We'll set up the fastest local mode first.
No government API keys are required for this path.

[1/4] Creating local database... done
[2/4] Downloading public bootstrap index... done
[3/4] Checking local search index... done
[4/4] Running health check... done

You can now try:
  ohg carrier lookup --mc 123456

For fresher live lookups later, run:
  ohg setup keys
```

#### 8.1.2 Guided Key Setup

```bash
ohg setup keys
```

Purpose: guide users through FMCSA and Socrata credential setup.

Behavior:

1. Explains which keys are optional vs. recommended.
2. Offers to open the correct pages in the browser.
3. Pauses for the user to complete account/key creation.
4. Accepts pasted keys.
5. Validates keys with test requests.
6. Stores keys in the OS keychain where available.
7. Falls back to encrypted config or plaintext config with a warning.

#### 8.1.3 Advanced/Headless Setup

```bash
ohg setup --no-browser
ohg setup keys --no-browser
ohg config set fmcsa.web_key <WEBKEY>
ohg config set socrata.app_token <APP_TOKEN>
```

Purpose: support SSH sessions, servers, CI, Docker containers, and locked-down environments.

Behavior:

- Never assumes a desktop browser is available.
- Prints direct URLs and short instructions.
- Supports environment variables:

```bash
export OHG_FMCSA_WEB_KEY="..."
export OHG_SOCRATA_APP_TOKEN="..."
```

- Supports Docker secrets or mounted config files in self-hosted deployments.

---

### 8.2 Browser-Opening UX

When the setup wizard needs the user to visit a site, it should ask:

```text
Open the FMCSA Developer login page now? [Press Enter to open, type s to skip]
```

If the user presses Enter, the CLI should open the URL using the OS default browser:

| Platform | Open command |
|---|---|
| macOS | `open <url>` |
| Windows | `rundll32 url.dll,FileProtocolHandler <url>` or `start <url>` |
| Linux | `xdg-open <url>` |
| WSL | try Windows browser handoff, otherwise print URL |
| Headless/SSH | print URL only |

If browser opening fails, print:

```text
I couldn't open a browser from this terminal.
Copy and paste this URL into your browser:
https://mobile.fmcsa.dot.gov/
```

Do not treat browser-open failure as setup failure.

---

### 8.3 FMCSA WebKey Guided Flow

FMCSA QCMobile requires a WebKey for API access. The setup flow should make this feel like a guided checklist, not documentation.

Command:

```bash
ohg setup fmcsa
```

Expected terminal flow:

```text
FMCSA WebKey setup

A FMCSA WebKey lets OpenHaul Guard run live carrier lookups against the QCMobile API.
You only need to do this once.

What you'll do:
1. Sign in with Login.gov.
2. Open "My WebKeys".
3. Click "Get a new WebKey".
4. Fill out a short form.
5. Copy the WebKey and paste it here.

Press Enter to open the FMCSA Developer site, or type s to skip:
> 
```

Open:

```text
https://mobile.fmcsa.dot.gov/
```

After browser open:

```text
In your browser:

1. Sign in with Login.gov.
2. Click "My WebKeys".
3. Click "Get a new WebKey".
4. Suggested form values:
   - Application Name: OpenHaul Guard Local
   - Type of Application: Non-commercial, Commercial, or Academic, based on your actual use
   - How many users: Choose the closest estimate. If this is only you, choose the smallest option.
   - Application Description: Local carrier verification and compliance lookup tool
   - Client Secret: Enter a unique phrase. Save it somewhere if FMCSA requires it later. OpenHaul Guard does not need it.
5. Click Create.
6. Copy the WebKey shown on screen.

When you have the WebKey, come back here and paste it.
```

Prompt:

```text
Paste FMCSA WebKey. Input will be hidden:
> 
```

Validation:

```text
Testing your FMCSA WebKey with a small QCMobile request...
```

Validation request:

```text
GET https://mobile.fmcsa.dot.gov/qc/services/carriers/name/greyhound?webKey=<WEBKEY>
```

Success:

```text
FMCSA WebKey works.
Stored in macOS Keychain as ohg/fmcsa.web_key.
```

Failure:

```text
That WebKey did not work.

FMCSA returned: 401 Unauthorized
Common fixes:
- Make sure you copied the WebKey, not the client secret.
- Make sure there are no spaces before or after the key.
- Wait a minute and try again if the key was just created.

Paste again, type b to reopen the browser, or type s to skip for now:
> 
```

Implementation requirements:

- Never log the WebKey.
- Never print the full WebKey after paste.
- Display only a short fingerprint, for example `...A92F`, if confirmation is needed.
- Do not ask the user to paste the FMCSA client secret. The tool only needs the WebKey.
- Provide `ohg setup fmcsa --reset` to replace a bad key.
- Provide `ohg config unset fmcsa.web_key` to remove it.

---

### 8.4 Socrata/DataHub App Token Guided Flow

A Socrata/Data & Insights app token should be optional, but strongly recommended for reliable DOT DataHub access and bulk/full-sync usage.

Command:

```bash
ohg setup socrata
```

Expected terminal flow:

```text
Socrata/DataHub app token setup

OpenHaul Guard can read public DOT DataHub datasets without turning them into a SaaS dependency.
An app token gives your local install its own request identity and better reliability.

This is optional for quick local use, but recommended for full syncs and repeated API queries.

Press Enter to open the Data & Insights account page, or type s to skip:
> 
```

Open a best-effort page. Preferred:

```text
https://data.transportation.gov/profile/edit/developer_settings
```

Fallback if direct profile URL is not available:

```text
https://data.transportation.gov/
```

Browser instructions:

```text
In your browser:

1. Create or sign into a free Data & Insights / Socrata account.
2. Open your profile or account menu.
3. Go to "Developer Settings".
4. Click "Create New App Token".
5. Suggested values:
   - Name: OpenHaul Guard Local
   - Description: Local DOT DataHub lookups and carrier dataset sync
   - Callback Prefix: leave blank unless the page requires it
6. Save the app token.
7. Copy the app token and return to this terminal.
```

Prompt:

```text
Paste Socrata app token. Input will be hidden:
> 
```

Validation request:

```text
GET https://data.transportation.gov/resource/az4n-8mr2.json?$limit=1
X-App-Token: <APP_TOKEN>
```

Success:

```text
Socrata app token works.
Stored in macOS Keychain as ohg/socrata.app_token.
```

Failure:

```text
That token did not work for DOT DataHub.

Common fixes:
- Make sure you created an App Token, not an API key secret.
- Make sure you copied the token value exactly.
- Try signing into data.transportation.gov specifically, then create the token from Developer Settings.

Paste again, type b to reopen the browser, or type s to skip for now:
> 
```

Implementation requirements:

- Socrata app token setup must never block quick setup.
- App token validation should be a small `$limit=1` request.
- Use the `X-App-Token` header, not URL query parameters, by default.
- Provide `ohg setup socrata --reset`.
- Provide `ohg config unset socrata.app_token`.

---

### 8.5 `ohg doctor` Requirements

`ohg doctor` must be the user's setup safety net.

Command:

```bash
ohg doctor
```

Output should be plain-English and actionable:

```text
OpenHaul Guard health check

Local database: OK
Config file: OK
OS keychain: OK
Public mirror: OK, manifest fetched 2026-04-25
FMCSA WebKey: OK, live QCMobile lookup succeeded
Socrata app token: Missing, optional but recommended for full sync
MCP server: OK, stdio mode available

Suggested next step:
  ohg carrier lookup --mc 123456
```

If something fails, show the exact fix:

```text
FMCSA WebKey: Missing
Why it matters: enables live carrier lookups from FMCSA QCMobile.
Fix:
  ohg setup fmcsa

Skip if you only want public mirror mode.
```

---

### 8.6 First Lookup Handoff

At the end of setup, the CLI should immediately offer a first lookup.

```text
Setup complete.

Try a carrier lookup now?
Enter MC, MX, FF, or USDOT number, or press Enter to skip:
> 
```

If user enters `MC123456` or `123456` with selected type:

```text
Running:
  ohg carrier lookup --mc 123456
```

This avoids the common failure where installation succeeds but the user does not know what to do next.

---

### 8.7 Installation UX Requirements

The project should support these install paths, in this order of priority:

#### Homebrew

```bash
brew install openhaulguard/tap/ohg
ohg setup
```

#### One-Line Installer

```bash
curl -fsSL https://github.com/johnmonarch/OpenHaul/releases/latest/download/install.sh | sh
```

Installer requirements:

- Detect OS and architecture.
- Install the latest stable binary.
- Verify checksum/signature.
- Print exactly what it installed.
- Immediately suggest `ohg setup`.

#### Docker

```bash
docker run --rm -it \
  -v ohg-data:/home/ohg/.openhaulguard \
  ghcr.io/openhaulguard/ohg:latest setup
```

#### Manual Binary

GitHub Releases must provide:

```text
ohg_darwin_arm64.tar.gz
ohg_darwin_amd64.tar.gz
ohg_linux_amd64.tar.gz
ohg_linux_arm64.tar.gz
ohg_windows_amd64.zip
checksums.txt
checksums.txt.sig
```

---

### 8.8 Setup Copy and Tone

The terminal copy should be direct, non-technical, and explicit.

Use:

```text
You now need a FMCSA WebKey.
Press Enter and I'll open the page where you get it.
When the WebKey appears, copy it and paste it here.
```

Avoid:

```text
Configure provider credentials.
Set fmcsa.web_key.
Authenticate against QCMobile.
```

The tool should explain acronyms the first time they appear.

Example:

```text
MC means Motor Carrier docket number.
USDOT means U.S. Department of Transportation number.
```

---

### 8.9 Resumable Setup State

Setup must be resumable. If a user quits halfway through key creation, rerunning `ohg setup` should continue where it left off.

Store setup state locally:

```text
~/.openhaulguard/setup_state.json
```

State examples:

```json
{
  "database_initialized": true,
  "mirror_bootstrap_complete": true,
  "fmcsa_key_validated": false,
  "socrata_token_validated": false,
  "first_lookup_complete": false
}
```

The state file must not contain secrets.

---

### 8.10 Credential Storage Requirements

Credential storage priority:

1. OS keychain/credential manager.
2. Encrypted local secret store if available.
3. Environment variables.
4. Plaintext config only with explicit warning.

Warning if plaintext is used:

```text
Warning: this environment does not expose a supported keychain.
OpenHaul Guard can save the key in ~/.openhaulguard/config.toml, but it will be readable by your user account.

Save anyway? [y/N]
```

Never store secrets in:

- Raw payloads.
- Reports.
- MCP responses.
- Debug logs.
- Crash reports.

---

### 8.11 Acceptance Criteria for Onboarding

- A first-time user can run `ohg setup --quick` and complete a cached/mirror-backed lookup without any API keys.
- A first-time user can run `ohg setup fmcsa`, press Enter to open the FMCSA Developer site, paste a WebKey, and have it validated.
- A first-time user can run `ohg setup socrata`, press Enter to open DataHub/Data & Insights, paste an app token, and have it validated.
- Failed credential validation provides a plain-English explanation and a retry path.
- Setup can be resumed without deleting local state.
- The tool provides a first lookup prompt at the end of setup.
- `ohg doctor` gives specific next commands, not generic failure messages.

---

## 9. CLI Commands

### 9.1 Setup

```bash
ohg setup
ohg setup --quick
ohg setup keys
ohg setup fmcsa
ohg setup socrata
ohg doctor
```

Non-interactive alternatives:

```bash
ohg init --no-interactive
ohg config set fmcsa.web_key <key>
ohg config set socrata.app_token <token>
ohg config set mode local
ohg doctor
```

Setup should default to the guided wizard described in Section 8. Direct `config set` commands are for advanced/headless users only.

### 9.2 Lookup

```bash
ohg carrier lookup --mc 123456
ohg carrier lookup --dot 1234567
ohg carrier lookup --name "Example Trucking LLC"
ohg carrier lookup --mc 123456 --format json
ohg carrier lookup --mc 123456 --offline
ohg carrier lookup --mc 123456 --force-refresh
ohg carrier lookup --mc 123456 --max-age 6h
```

### 9.3 History and Diff

```bash
ohg carrier history --mc 123456
ohg carrier diff --mc 123456 --since 30d
ohg carrier timeline --dot 1234567
```

### 9.4 Watchlist

```bash
ohg watch add --mc 123456
ohg watch remove --mc 123456
ohg watch list
ohg watch sync
ohg watch report --format markdown
```

### 9.5 Packet Check

```bash
ohg packet check ./packet.pdf --mc 123456
ohg packet check ./packet.pdf --dot 1234567 --format markdown
ohg packet extract ./packet.pdf --format json
```

### 9.6 Full Sync

```bash
ohg sync census
ohg sync authority
ohg sync insurance
ohg sync sms
ohg sync oos
ohg sync full
ohg sync status
```

### 9.7 MCP

```bash
ohg mcp serve
ohg mcp serve --transport stdio
ohg mcp serve --transport http --port 8787
```

---

## 10. Ingestion Strategy

OpenHaul Guard OSS should support three ingestion modes.

---

### 10.1 Mode A: Lazy Lookup

Default mode.

```bash
ohg carrier lookup --mc 123456
```

Flow:

1. Normalize input.
2. Check local identifier index.
3. If unknown, resolve MC/MX/FF docket number to USDOT.
4. Fetch current public data.
5. Normalize into local schema.
6. Store raw payload and normalized observation.
7. Run risk rules.
8. Return report.

This mode gives instant utility and begins local history from the first lookup.

---

### 10.2 Mode B: Watchlist Sync

User-selected carriers are refreshed on demand or by scheduler.

```bash
ohg watch sync
```

Flow:

1. Load watchlist.
2. Refresh each carrier.
3. Compare against last local observation.
4. Store new snapshot.
5. Emit changed-field events.
6. Recompute risk flags.
7. Generate report/alerts.

---

### 10.3 Mode C: Full Local Sync

Power-user mode.

```bash
ohg sync full
```

Flow:

1. Download selected public datasets.
2. Store raw files with date/source metadata.
3. Parse into staging tables.
4. Normalize to canonical tables.
5. Build lookup indexes.
6. Compute historical risk signals.
7. Vacuum/analyze local DB.

Full sync is optional and should never block lazy lookup.

---

## 11. Data Sources and API Requirements

### 11.1 Source Summary

| Source | Use | Best For | Registration Required? | Notes |
|---|---|---:|---:|---|
| FMCSA QCMobile API | Real-time-ish carrier lookup by name, USDOT, docket/MC | Lazy lookup | Yes, FMCSA developer account and WebKey | Best default official lookup source when user has a key |
| DOT DataHub / Socrata SODA | Bulk and targeted queries against FMCSA open datasets | Full sync, fallback lookup | App token strongly recommended; SODA3 requires identification | Public datasets remain open, but tokens reduce throttling |
| FMCSA SAFER Company Snapshot | Human-readable single-carrier snapshot | Manual verification, fallback display | No | Free ad-hoc query, but not the preferred machine API |
| FMCSA SMS Downloads / DataHub | SMS input/output files | Monthly safety performance context | Usually no account for downloads; app token helpful for API | Use carefully with FMCSA caution language |
| IANA RDAP Bootstrap + RDAP servers | Domain registration age/status for packet/email checks | Email/domain risk flags | No standard registration | Respect rate limits; RDAP data may be redacted |
| Optional OCR/local PDF extraction | Packet checking | Local document comparison | No external API required | Keep local by default |

---

### 11.2 FMCSA QCMobile API

**Purpose:** Lazy carrier lookup and current carrier detail refresh.

**Base URL:**

```text
https://mobile.fmcsa.dot.gov/qc/services/
```

**Authentication:** Required `webKey` query parameter.

**Registration required:** Yes.

**User steps:**

1. Create/log into FMCSA Developer account using Login.gov.
2. Go to My WebKeys.
3. Request a new WebKey.
4. Configure locally:

```bash
ohg config set fmcsa.web_key <WEBKEY>
```

**Relevant endpoints:**

```text
GET /carriers/name/:name?webKey=<key>
GET /carriers/:dotNumber?webKey=<key>
GET /carriers/docket-number/:docketNumber?webKey=<key>
GET /carriers/:dotNumber/basics?webKey=<key>
GET /carriers/:dotNumber/cargo-carried?webKey=<key>
GET /carriers/:dotNumber/operation-classification?webKey=<key>
GET /carriers/:dotNumber/oos?webKey=<key>
GET /carriers/:dotNumber/docket-numbers?webKey=<key>
GET /carriers/:dotNumber/authority?webKey=<key>
```

**Use in OSS:**

- Preferred source for lazy lookup when `fmcsa.web_key` is configured.
- Docket endpoint resolves MC/MX/FF to USDOT.
- Store raw JSON responses per endpoint.

**Limitations:**

- Requires a web key, which adds onboarding friction.
- Should not share or embed a project-wide WebKey in the OSS repo.
- Data freshness and scope may differ from DataHub files or SAFER pages.

---

### 11.3 DOT DataHub / Socrata SODA

**Purpose:** Full sync, historical operating authority datasets, daily-difference sync, fallback lookup.

**Base domain:**

```text
https://data.transportation.gov
```

**API patterns:**

```text
https://data.transportation.gov/resource/<dataset_id>.json
https://data.transportation.gov/api/views/<dataset_id>/rows.csv?accessType=DOWNLOAD
```

**Authentication/identification:**

- Public datasets are open.
- SODA 2.x may allow simple unauthenticated queries but with lower shared throttling.
- SODA3 requires either user authentication, API key/secret, or an app token for public datasets.
- App token is strongly recommended.

**User configuration:**

```bash
ohg config set socrata.app_token <APP_TOKEN>
```

**Header:**

```http
X-App-Token: <APP_TOKEN>
```

**Primary dataset IDs:**

| Dataset | Dataset ID | Use |
|---|---:|---|
| Company Census File | `az4n-8mr2` | Current entity/census data, DOT identity, operations, equipment, drivers |
| InsHist - All With History | `6sqe-dvqs` | Previous insurance policies |
| Carrier - All With History | `6eyk-hxee` | Operating authority entity records with history |
| Rejected - All With History | `96tg-4mhf` | Rejected insurance forms |
| BOC3 - All With History | `2emp-mxtb` | Process agent records |
| AuthHist - All With History | `9mw4-x3tu` | Authority grant/revocation history |
| ActPendInsur - All With History | `qh9u-swkp` | Active/pending insurance implementation dates |
| Insur - All With History | `ypjt-5ydn` | Active/pending insurance policies |
| Revocation - All With History | `sa6p-acbp` | Revoked authorities |
| InsHist daily diff | `xkmg-ff2t` | Daily changed insurance history |
| Carrier daily diff | `6qg9-x4f8` | Daily changed authority carrier records |
| Rejected daily diff | `t3zq-c6n3` | Daily rejected insurance forms |
| BOC3 daily diff | `fb8g-ngam` | Daily BOC-3 changes |
| AuthHist daily diff | `sn3k-dnx7` | Daily authority history changes |
| ActPendInsur daily diff | `chgs-tx6x` | Daily active/pending insurance changes |
| Insur daily diff | `mzmm-6xep` | Daily insurance policy changes |
| Revocation daily diff | `pivg-szje` | Daily revocation changes |
| New Entrant Out of Service Orders | `p2mt-9ige` | New Entrant OOS order checks |

**Recommended usage:**

- Lazy fallback: query Company Census and/or Carrier datasets by docket/USDOT when QCMobile key is missing.
- Full sync: download CSV files and build local indexes.
- Incremental sync: apply daily-difference datasets after baseline sync.

**Example targeted SODA query:**

```http
GET https://data.transportation.gov/resource/az4n-8mr2.json?$limit=10&dot_number=1234567
X-App-Token: <APP_TOKEN>
```

Field names must be confirmed from each dataset’s data dictionary during implementation.

---

### 11.4 FMCSA SAFER Company Snapshot

**Purpose:** Human-readable fallback and manual validation.

**URL:**

```text
https://safer.fmcsa.dot.gov/CompanySnapshot.aspx
```

**Registration required:** No.

**Capabilities:**

- Search by DOT number, MC/MX number, or company name.
- Shows identification, size, commodity info, safety rating, roadside out-of-service inspection summary, and crash information.

**Use in OSS:**

- Link to SAFER in reports.
- Optional manual-open command:

```bash
ohg carrier open-safer --mc 123456
```

**Implementation note:**

Avoid making SAFER scraping a core dependency. Prefer QCMobile and DataHub APIs. If an HTML parser is ever added, it must be conservative, rate-limited, and compliant with site terms.

---

### 11.5 SMS / CSA Data

**Purpose:** Safety context, not automatic disqualification.

**Source:** FMCSA SMS Downloads and DataHub files.

**Refresh cadence:** Monthly.

**Use in OSS:**

- Add safety context to reports.
- Show data vintage clearly.
- Use caution language.
- Do not infer overall safety fitness solely from SMS metrics.

**Required disclaimer behavior:**

Reports must include language that SMS data should not be used alone to draw conclusions about a carrier’s overall safety condition unless the carrier has an unsatisfactory safety rating or has been ordered to discontinue operations.

---

### 11.6 RDAP Domain Checks

**Purpose:** Optional risk flags for packet/email/domain verification.

**Sources:**

- IANA RDAP Bootstrap for TLD-to-RDAP server discovery.
- TLD registry RDAP servers.

**Registration required:** Usually no.

**Use cases:**

- Domain creation date, if available.
- Domain status.
- Registrar.
- Nameserver patterns.
- Compare carrier packet email domain to legal company name/domain.

**Commands:**

```bash
ohg domain check examplecarrier.com
ohg packet check ./packet.pdf --mc 123456 --check-domain
```

**Limitations:**

- RDAP data may be redacted.
- Not all TLDs expose the same event fields.
- A young domain is only a review signal, not a fraud conclusion.

---

## 12. Reducing User Friction

The friction problem is real: QCMobile requires a FMCSA WebKey, and Socrata performs better with an app token. The OSS product should support multiple low-friction paths. The setup wizard in Section 8 is the primary user-facing mechanism for reducing this friction: it should open the right pages, pause for the user, validate pasted keys, and recover gracefully when a key is missing or wrong.

### 12.1 Frictionless Path A: Public Mirror / Bootstrap Index

Future option: create a project-maintained **OpenHaul Public Mirror**.

No hosted OpenHaul mirror or project domain exists today. For a future hosted mirror, use GitHub Releases, Cloudflare R2, or another reviewed distribution channel:

```text
https://github.com/johnmonarch/OpenHaul/releases
```

Publish compressed normalized snapshots generated by project CI:

```text
mc_dot_index.latest.parquet.zst
company_census.latest.parquet.zst
authority_index.latest.parquet.zst
insurance_index.latest.parquet.zst
dataset_manifest.json
checksums.txt
```

First-run flow:

```bash
ohg init
```

Prompt:

```text
No FMCSA WebKey found.

Choose data mode:
1. Use OpenHaul public mirror. No registration, easiest.
2. Use your own FMCSA/Socrata keys. Best freshness/control.
3. Local/offline only. Import files manually.
```

Recommended default:

```bash
ohg init --quick
```

Downloads only:

- MC/MX/FF to USDOT index.
- Company Census light index.
- Authority status light index.
- Manifest/checksums.

This allows instant MC lookup without requiring any user registration.

**Important:** Redistribution rights and attribution requirements must be reviewed before launch. FMCSA open data is public and no-cost, but the project must confirm DataHub/Socrata terms for republishing normalized mirrors.

### 12.2 Frictionless Path B: Bring-Your-Own Key Later

Do not block first use on keys. Let users add keys when they want better freshness.

```bash
ohg carrier lookup --mc 123456
```

If no keys are set:

1. Try local/mirror index.
2. Use DataHub no-token call only for light usage if available.
3. Explain that a FMCSA WebKey improves live lookups.
4. Offer `ohg setup keys`.

### 12.3 Frictionless Path C: Hosted Community Lookup Proxy

Future option: offer a free, rate-limited community API for OSS users.

No hosted OpenHaul community API exists today. A future community API URL must be documented only after the domain, hosting, rate limits, and data policy exist.

Use only for:

- MC/MX/FF to USDOT resolution.
- Data manifest discovery.
- Update checks.
- Very low-volume current carrier lookup.

Do not use this to secretly create a SaaS dependency. The CLI must remain useful offline and self-hostable.

### 12.4 Frictionless Path D: File Import

Allow manual import of CSV exports:

```bash
ohg import census ./Company_Census_File.csv
ohg import authority ./Carrier_All_With_History.csv
```

This is useful for locked-down enterprise users.

---

## 13. Local Storage Design

Default local path:

```text
~/.openhaulguard/
  config.toml
  ohg.db
  raw/
  cache/
  reports/
  skills/
```

### 13.1 Database

Default database: SQLite.

Optional analytics engine: DuckDB for large full-sync workflows.

Core tables:

```sql
carriers
carrier_identifiers
carrier_observations
carrier_source_payloads
authority_records
authority_history
insurance_records
insurance_history
boc3_records
oos_orders
sms_snapshots
packet_documents
packet_extracted_fields
risk_flags
risk_events
watchlist
sync_runs
source_manifests
```

### 13.2 Observation Model

Each lookup stores an observation.

```sql
carrier_observations (
  id TEXT PRIMARY KEY,
  usdot_number TEXT NOT NULL,
  observed_at TEXT NOT NULL,
  source TEXT NOT NULL,
  legal_name TEXT,
  dba_name TEXT,
  physical_address TEXT,
  mailing_address TEXT,
  phone TEXT,
  email TEXT,
  authority_status TEXT,
  safety_rating TEXT,
  normalized_hash TEXT NOT NULL,
  raw_payload_id TEXT
)
```

### 13.3 Raw Payload Storage

Store raw data for auditability.

```text
raw/
  fmcsa_qcmobile/
    1234567/
      2026-04-25T23-10-00Z.carrier.json
      2026-04-25T23-10-01Z.authority.json
  datahub/
    az4n-8mr2/
      2026-04-25.rows.csv.zst
```

---

## 14. Risk Scoring

### 14.1 Principle

Risk scoring must be deterministic, explainable, and evidence-based. The report should lead with specific flags, not a mysterious score.

### 14.2 Risk Output

```json
{
  "risk_summary": {
    "level": "manual_review_recommended",
    "score": 62,
    "confidence": "medium"
  },
  "flags": [
    {
      "code": "NEW_AUTHORITY",
      "severity": "medium",
      "evidence": "Authority first observed as active 31 days ago.",
      "source": "FMCSA Authority",
      "recommendation": "Confirm carrier identity before tendering high-value freight."
    }
  ]
}
```

### 14.3 MVP Rules

| Code | Severity | Trigger |
|---|---:|---|
| `NO_LOCAL_HISTORY` | Info | First time this local install has seen the carrier |
| `NEW_AUTHORITY` | Medium | Authority appears active for less than configured threshold |
| `RECENT_AUTHORITY_CHANGE` | Medium/High | Authority changed in recent local or public history |
| `RECENT_ADDRESS_CHANGE` | Medium/High | Address changed in recent local or public history |
| `RECENT_NAME_CHANGE` | Medium/High | Legal name/DBA changed recently |
| `RECENT_INSURANCE_CHANGE` | Medium | Insurance filing changed recently |
| `REVOKED_AUTHORITY_HISTORY` | Medium | Public history shows prior revocation |
| `OOS_ORDER_FOUND` | High | New Entrant OOS order found |
| `PACKET_NAME_MISMATCH` | Medium/High | Packet name conflicts with public record |
| `PACKET_PHONE_MISMATCH` | Medium/High | Packet phone conflicts with public record |
| `PACKET_ADDRESS_MISMATCH` | Medium/High | Packet address conflicts with public record |
| `EMAIL_DOMAIN_RECENT` | Medium | RDAP suggests young email domain |
| `EMAIL_DOMAIN_MISMATCH` | Low/Medium | Email domain does not appear connected to carrier identity |
| `SAFETY_UNSATISFACTORY` | High | Safety rating is unsatisfactory, if available |
| `DATA_STALE` | Info/Medium | Local source data older than configured freshness threshold |

### 14.4 Language Rules

The tool must say:

- “Manual review recommended.”
- “Public records show…”
- “OpenHaul Guard detected a mismatch…”
- “This is a risk signal, not proof of fraud.”

The tool must not say:

- “This carrier is fraudulent.”
- “Do not use this carrier.”
- “Blacklisted.”
- “Safe carrier” unless this is carefully defined as “No configured risk flags detected.”

---

## 15. MCP Server Requirements

### 15.1 Command

```bash
ohg mcp serve
```

### 15.2 Tools

| MCP Tool | Description |
|---|---|
| `carrier_lookup` | Lookup by MC/MX/FF/USDOT/name |
| `carrier_risk` | Return risk flags and evidence |
| `carrier_explain` | Plain-English explanation of flags |
| `carrier_history` | Local and public history timeline |
| `carrier_diff` | Compare snapshots over time |
| `packet_extract` | Extract fields from a carrier packet |
| `packet_check` | Compare packet to public data |
| `watchlist_add` | Add carrier to watchlist |
| `watchlist_sync` | Refresh watchlist |
| `report_generate` | Generate Markdown/JSON/PDF report |

### 15.3 Security

- Default transport: stdio.
- HTTP transport disabled unless explicitly enabled.
- No public bind by default.
- If HTTP is enabled, bind to `127.0.0.1` unless user opts into LAN.
- API key required for non-stdio HTTP mode.
- Never expose stored FMCSA/Socrata credentials through MCP responses.

---

## 16. SKILL.md Requirements

The repository must include:

```text
skills/openhaulguard/SKILL.md
```

The skill should document:

1. Purpose.
2. When to use.
3. CLI commands.
4. MCP tools.
5. Interpretation rules.
6. Safety/legal language.
7. Output examples.
8. Common failure modes.
9. How to cite evidence in generated reports.

Required interpretation rules:

```markdown
- Do not say a carrier is fraudulent based only on OpenHaul Guard flags.
- Separate public-record facts from OpenHaul Guard inferences.
- Prefer "manual review recommended" over "reject."
- Always cite specific mismatches or changes.
- Mention source freshness.
- If data is stale or missing, say so clearly.
```

---

## 17. Reports

### 17.1 Report Types

```bash
ohg report carrier --mc 123456 --format markdown
ohg report carrier --mc 123456 --format pdf
ohg report packet ./packet.pdf --mc 123456 --format markdown
```

### 17.2 Report Sections

1. Carrier identity.
2. Data freshness.
3. Authority status.
4. Insurance/BOC-3 summary.
5. Safety context.
6. Local observation history.
7. Public history timeline.
8. Packet comparison, if applicable.
9. Risk flags.
10. Recommended manual review actions.
11. Source links and disclaimers.

---

## 18. Data Freshness Rules

Every output must show freshness.

Example:

```text
Data freshness:
- QCMobile carrier profile: fetched 2026-04-25 23:10:00 UTC
- Company Census File: generated from 24-hour-old database, local copy dated 2026-04-25
- SMS data: monthly dataset, current through 2026-03
- Local history: first observed by this install on 2026-04-25
```

Default cache behavior:

| Source | Default TTL |
|---|---:|
| QCMobile carrier lookup | 24 hours |
| QCMobile authority | 24 hours |
| DataHub Company Census | 24 hours |
| DataHub authority daily diffs | 24 hours |
| SMS data | 30 days |
| RDAP domain data | 7 days |
| Packet extraction | Until file hash changes |

---

## 19. Architecture

### 19.1 Repository Layout

```text
openhaulguard/
  cmd/ohg/
  internal/
    ingest/
    sources/
      fmcsa_qcmobile/
      datahub_socrata/
      safer/
      rdap/
    normalize/
    risk/
    diff/
    packet/
    report/
    storage/
    mcp/
  schemas/
  docs/
  skills/
    openhaulguard/
      SKILL.md
  examples/
  docker/
  testdata/
```

### 19.2 Recommended Stack

- Language: Go.
- CLI: Cobra.
- Local DB: SQLite.
- Optional analytics: DuckDB.
- HTTP client: standard Go client with retry/backoff.
- PDF/text extraction: local libraries first.
- MCP: Go MCP SDK or lightweight JSON-RPC implementation.
- Report generation: Markdown first, HTML/PDF optional.

### 19.3 Config

```toml
[mode]
default = "local"

[fmcsa]
web_key = ""

[socrata]
app_token = ""

[mirror]
enabled = true
local_path = "~/.openhaulguard/mirror/carriers.json"

[cache]
qcmobile_ttl = "24h"
datahub_ttl = "24h"
sms_ttl = "30d"

[mcp]
transport = "stdio"
http_bind = "127.0.0.1"
http_port = 8787
```

---

## 20. API and Source Client Requirements

### 20.1 Source Interface

Each source connector must implement:

```go
type SourceClient interface {
    Name() string
    FetchCarrierByDOT(ctx context.Context, dot string) (*RawPayload, error)
    FetchCarrierByDocket(ctx context.Context, docket string) (*RawPayload, error)
    FetchCarrierByName(ctx context.Context, name string, limit int) ([]*RawPayload, error)
    SupportsLazyLookup() bool
    RequiresCredential() bool
}
```

### 20.2 Normalizer Interface

```go
type Normalizer interface {
    SourceName() string
    NormalizeCarrier(payload RawPayload) (*CarrierObservation, error)
    NormalizeAuthority(payload RawPayload) ([]AuthorityRecord, error)
    NormalizeInsurance(payload RawPayload) ([]InsuranceRecord, error)
}
```

### 20.3 Risk Rule Interface

```go
type RiskRule interface {
    Code() string
    Evaluate(ctx RiskContext) ([]RiskFlag, error)
}
```

---

## 21. Privacy and Security

1. Local lookup history stays local.
2. Credentials stored in OS keychain where available.
3. Plaintext config fallback must be warned about.
4. No telemetry by default.
5. Optional update checks must be disclosed.
6. Community mirror requests should not include packet data.
7. Packet documents remain local unless the user explicitly configures hosted mode later.
8. MCP HTTP mode must not bind publicly by default.
9. Redact credentials from logs.
10. Raw payloads may contain addresses/phone numbers from public data. Treat local DB as sensitive operational data.

---

## 22. Compliance, Legal, and Safety Notes

1. This product is an evidence and review tool, not a legal determination engine.
2. Reports must not claim a carrier is fraudulent.
3. SMS data must be accompanied by FMCSA caution language.
4. SAFER, FMCSA, DataHub, and Socrata usage must comply with applicable terms.
5. Redistribution via public mirror requires review before launch.
6. Packet-checking output must distinguish OCR/extraction uncertainty from true mismatch.
7. Contributors must avoid adding non-public personal information sources.

---

## 23. MVP Scope

### 23.1 MVP Included

- `ohg setup`
- `ohg init`
- `ohg carrier lookup --mc`
- `ohg carrier lookup --dot`
- QCMobile connector
- DataHub/Socrata connector for Company Census fallback
- SQLite local DB
- Raw payload storage
- Normalized carrier observations
- Basic risk rules
- JSON/table/Markdown output
- `ohg carrier diff`
- `ohg watch add/list/sync`
- MCP stdio server
- `SKILL.md`
- Docker image

### 23.2 MVP Deferred

- PDF report generation.
- Advanced packet extraction.
- Full SMS ingestion.
- Full authority historical sync.
- Cloud-hosted API mode.
- Enterprise team management.
- TMS plugins.
- Browser UI.

---

## 24. Post-MVP Roadmap

### Phase 2

- Packet checker MVP.
- RDAP domain checks.
- Full authority and insurance sync.
- Public mirror bootstrap index.
- HTML/PDF reports.
- Homebrew install.

### Phase 3

- Full SMS context.
- Daily-diff incremental sync.
- Rule pack system.
- Self-hosted web dashboard.
- TMS webhook examples.
- Hosted API client mode.

### Phase 4

- Enterprise Docker Compose stack.
- PostgreSQL backend option.
- Multi-user/team mode.
- Private mirror support.
- Policy-as-code review workflows.

---

## 25. Success Metrics

### OSS Adoption

- GitHub stars.
- CLI downloads.
- Docker pulls.
- Number of contributors.
- Number of third-party integrations.

### Product Utility

- Time to first carrier lookup under 5 minutes.
- First lookup success rate above 90%.
- False-positive complaints tracked by rule.
- Packet mismatch precision improved over time.

### Technical Quality

- Lookup p95 under 3 seconds when cached.
- Lazy lookup p95 under 10 seconds with live API.
- Local DB can handle 100,000 watched carriers.
- Full sync can resume after interruption.
- MCP tool calls return structured errors.

---

## 26. Open Questions

1. Can the project legally redistribute normalized FMCSA/DataHub data through a public mirror?
2. Should the default license be Apache-2.0 for maximal adoption or AGPL-3.0 to protect the hosted business model?
3. Should QCMobile be required for “live” current-state lookup, or should DataHub be the default zero-key path?
4. How much packet extraction should be included before the project becomes too complex?
5. Should the risk score be hidden by default and only show flags?
6. Should the first hosted offering be a community mirror or a full commercial API?
7. What is the right retention policy for raw local payloads?

---

## 27. Recommended Implementation Sequence

1. Build local schema and storage.
2. Build guided `ohg setup`, `ohg setup fmcsa`, `ohg setup socrata`, and `ohg doctor`.
3. Build QCMobile client.
4. Build DataHub/Socrata Company Census connector.
5. Implement MC/DOT resolver.
6. Implement lazy lookup.
7. Implement report output.
8. Implement raw payload storage.
9. Implement first 8 risk rules.
10. Implement local diff.
11. Implement watchlist sync.
12. Implement MCP stdio server.
13. Write `SKILL.md`.
14. Add public mirror support.
15. Add packet checker.

---

## 28. References

- FMCSA QCMobile API documentation: https://mobile.fmcsa.dot.gov/QCDevsite/docs/qcApi
- FMCSA API access / WebKey documentation: https://mobile.fmcsa.dot.gov/QCDevsite/docs/apiAccess
- FMCSA Open Data Program: https://www.fmcsa.dot.gov/registration/fmcsa-data-dissemination-program
- FMCSA SAFER Company Snapshot: https://safer.fmcsa.dot.gov/CompanySnapshot.aspx
- FMCSA SMS Tools: https://ai.fmcsa.dot.gov/SMS/Tools/Index.aspx
- Socrata application tokens: https://dev.socrata.com/docs/app-tokens.html
- Socrata SODA getting started: https://dev.socrata.com/consumers/getting-started.html
- Socrata SODA3 API update: https://support.socrata.com/hc/en-us/articles/34730618169623-SODA3-API
