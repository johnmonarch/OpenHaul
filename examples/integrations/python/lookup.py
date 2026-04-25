#!/usr/bin/env python3
import json
import os
import urllib.error
import urllib.request


BASE_URL = os.environ.get("OHG_BASE_URL", "http://127.0.0.1:8787")
TOKEN = os.environ.get("OHG_API_TOKEN")
IDENTIFIER_TYPE = os.environ.get("OHG_IDENTIFIER_TYPE", "mc")
IDENTIFIER_VALUE = os.environ.get("OHG_IDENTIFIER_VALUE", "123456")


def request(path, method="GET", payload=None):
    headers = {"Accept": "application/json"}
    body = None

    if payload is not None:
        body = json.dumps(payload).encode("utf-8")
        headers["Content-Type"] = "application/json"

    if TOKEN:
        headers["Authorization"] = f"Bearer {TOKEN}"

    req = urllib.request.Request(
        f"{BASE_URL}{path}",
        data=body,
        headers=headers,
        method=method,
    )

    try:
        with urllib.request.urlopen(req, timeout=30) as response:
            return json.loads(response.read().decode("utf-8"))
    except urllib.error.HTTPError as exc:
        error_body = exc.read().decode("utf-8")
        raise RuntimeError(f"{exc.code} {exc.reason}: {error_body}") from exc


health = request("/health")
print("health", json.dumps(health, indent=2))

lookup = request(
    "/v1/carrier/lookup",
    method="POST",
    payload={
        "identifier_type": IDENTIFIER_TYPE,
        "identifier_value": IDENTIFIER_VALUE,
        "max_age": "24h",
    },
)

print(
    "lookup",
    json.dumps(
        {
            "report_type": lookup.get("report_type"),
            "input": lookup.get("lookup"),
            "carrier": {
                "usdot_number": lookup.get("carrier", {}).get("usdot_number"),
                "legal_name": lookup.get("carrier", {}).get("legal_name"),
            },
            "risk_assessment": lookup.get("risk_assessment"),
        },
        indent=2,
    ),
)
