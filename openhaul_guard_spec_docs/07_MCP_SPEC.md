<!--
OpenHaul Guard OSS specification bundle
Generated: 2026-04-25
Audience: coding agents and human maintainers
Primary implementation language: Go
-->
# 07 MCP Specification

## 1. Purpose

The MCP server lets coding agents and workflow agents call OpenHaul Guard safely and consistently.

Command:

```bash
ohg mcp serve
```

Default transport:

```text
stdio
```

Optional post-MVP transport:

```text
http on 127.0.0.1 only by default
```

## 2. Design requirements

- MCP tools must call the same application service layer as the CLI.
- MCP responses must use the same JSON models as CLI JSON output.
- MCP must not make hosted calls unless configured.
- MCP must not expose secrets.
- MCP must include safety language in tool descriptions.

## 3. Tools

### 3.1 carrier_lookup

Description:

Lookup a carrier by MC, MX, FF, DOT, or name. Returns public-record facts, local freshness, and risk flags for manual review.

Input schema:

```json
{
  "type": "object",
  "properties": {
    "identifier_type": {"type": "string", "enum": ["mc", "mx", "ff", "dot", "name"]},
    "identifier_value": {"type": "string"},
    "force_refresh": {"type": "boolean", "default": false},
    "offline": {"type": "boolean", "default": false},
    "max_age": {"type": "string", "default": "24h"}
  },
  "required": ["identifier_type", "identifier_value"]
}
```

Output:

Same as `ohg carrier lookup --format json`.

### 3.2 carrier_diff

Input:

```json
{
  "type": "object",
  "properties": {
    "identifier_type": {"type": "string", "enum": ["mc", "mx", "ff", "dot"]},
    "identifier_value": {"type": "string"},
    "since": {"type": "string"},
    "strict": {"type": "boolean", "default": false}
  },
  "required": ["identifier_type", "identifier_value", "since"]
}
```

### 3.3 carrier_explain_flags

Input:

```json
{
  "type": "object",
  "properties": {
    "assessment_id": {"type": "string"},
    "flag_codes": {"type": "array", "items": {"type": "string"}}
  }
}
```

Behavior:

Return plain-English explanations and evidence for requested flags. If no assessment ID is provided, explain flags from the latest observation for the carrier when identifier is supplied.

### 3.4 watchlist_add

Input:

```json
{
  "type": "object",
  "properties": {
    "identifier_type": {"type": "string", "enum": ["mc", "mx", "ff", "dot"]},
    "identifier_value": {"type": "string"},
    "label": {"type": "string"}
  },
  "required": ["identifier_type", "identifier_value"]
}
```

### 3.5 watchlist_sync

Input:

```json
{
  "type": "object",
  "properties": {
    "label": {"type": "string"},
    "force_refresh": {"type": "boolean", "default": false}
  }
}
```

### 3.6 packet_check

Input:

```json
{
  "type": "object",
  "properties": {
    "path": {"type": "string"},
    "identifier_type": {"type": "string", "enum": ["mc", "mx", "ff", "dot"]},
    "identifier_value": {"type": "string"}
  },
  "required": ["path", "identifier_type", "identifier_value"]
}
```

### 3.7 generate_report

Input:

```json
{
  "type": "object",
  "properties": {
    "report_type": {"type": "string", "enum": ["carrier_lookup", "carrier_diff", "packet_check"]},
    "identifier_type": {"type": "string"},
    "identifier_value": {"type": "string"},
    "format": {"type": "string", "enum": ["markdown", "json", "html"], "default": "markdown"}
  },
  "required": ["report_type", "identifier_type", "identifier_value"]
}
```

## 4. MCP safety instructions

Every tool description should include:

```text
Do not use this tool to declare a carrier fraudulent. Use returned evidence and risk flags for manual review only.
```

## 5. Agent interpretation rules

Agents using OpenHaul Guard must:

- Cite risk flag evidence from tool output.
- Separate public-record facts from OpenHaul Guard inferences.
- Avoid blacklisting language.
- Ask for manual review where serious mismatches exist.
- Mention data freshness.
- Mention first-seen limitations for local-only history.

## 6. Example MCP agent interaction

User:

```text
Check MC 123456 and draft a note to compliance if anything looks off.
```

Agent tool call:

```json
{
  "tool": "carrier_lookup",
  "arguments": {
    "identifier_type": "mc",
    "identifier_value": "123456",
    "force_refresh": true
  }
}
```

Agent response should not say:

```text
This is fraud.
```

Agent response may say:

```text
Manual review is recommended. The packet phone number does not match the public record, and the authority appears to be less than 30 days old. I would confirm contact details through an independent source before tendering freight.
```
