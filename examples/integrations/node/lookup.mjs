const baseUrl = process.env.OHG_BASE_URL ?? "http://127.0.0.1:8787";
const token = process.env.OHG_API_TOKEN;
const identifierType = process.env.OHG_IDENTIFIER_TYPE ?? "mc";
const identifierValue = process.env.OHG_IDENTIFIER_VALUE ?? "123456";

const headers = {
  "Content-Type": "application/json",
};

if (token) {
  headers.Authorization = `Bearer ${token}`;
}

async function request(path, options = {}) {
  const response = await fetch(`${baseUrl}${path}`, {
    ...options,
    headers: {
      ...headers,
      ...options.headers,
    },
  });
  const body = await response.text();

  if (!response.ok) {
    throw new Error(`${response.status} ${response.statusText}: ${body}`);
  }

  return body ? JSON.parse(body) : null;
}

const health = await request("/health", { headers: {} });
console.log("health", health);

const lookup = await request("/v1/carrier/lookup", {
  method: "POST",
  body: JSON.stringify({
    identifier_type: identifierType,
    identifier_value: identifierValue,
    max_age: "24h",
  }),
});

console.log("lookup", {
  report_type: lookup.report_type,
  input: lookup.lookup,
  carrier: {
    usdot_number: lookup.carrier?.usdot_number,
    legal_name: lookup.carrier?.legal_name,
  },
  risk_assessment: lookup.risk_assessment,
});
