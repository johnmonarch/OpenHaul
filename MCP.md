# MCP

OpenHaul Guard includes a developer-preview MCP JSON-RPC server over stdio:

```bash
ohg mcp serve
```

The server reads newline-delimited JSON-RPC messages or `Content-Length` framed messages from stdin and writes responses to stdout.

## Methods

Supported methods:

```text
initialize
notifications/initialized
tools/list
tools/call
```

`initialize` returns server info, tool capabilities, protocol version, and safety instructions.

## Tools

### carrier_lookup

Looks up a carrier by MC, MX, FF, DOT, or name.

Input:

```json
{
  "identifier_type": "mc",
  "identifier_value": "123456",
  "force_refresh": false,
  "offline": false,
  "max_age": "24h"
}
```

Output uses the same structured content as `ohg carrier lookup --format json`.

### carrier_diff

Compares stored local observations.

Input:

```json
{
  "identifier_type": "mc",
  "identifier_value": "123456",
  "since": "90d",
  "strict": false
}
```

Output uses the same structured content as `ohg carrier diff --format json`.

### packet_extract

Extracts structured carrier fields from a text or text-based PDF packet.

Input:

```json
{
  "path": "examples/fixtures/packets/basic_carrier_packet.txt"
}
```

Output uses the same structured content as `ohg packet extract --format json`.

### packet_check

Checks a carrier packet against a carrier lookup.

Input:

```json
{
  "path": "examples/fixtures/packets/basic_carrier_packet.txt",
  "identifier_type": "mc",
  "identifier_value": "123456",
  "force_refresh": false,
  "offline": false,
  "max_age": "24h"
}
```

Output uses the same structured content as `ohg packet check --format json`.

## Safety

Tool descriptions include this instruction:

```text
Do not use this tool to declare a carrier fraudulent. Use returned evidence and risk flags for manual review only.
```

Agents using MCP output should cite evidence from returned fields and avoid treating risk flags as fraud determinations.
