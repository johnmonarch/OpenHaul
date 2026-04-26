package fmcsa

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/openhaulguard/openhaulguard/internal/domain"
)

const SourceName = "fmcsa_qcmobile"

type Client struct {
	BaseURL    string
	WebKey     string
	HTTPClient *http.Client
	UserAgent  string
}

type RawResponse struct {
	Endpoint string
	Body     []byte
	Fetch    domain.SourceFetchResult
}

func New(webKey, version string) *Client {
	return &Client{
		BaseURL:    "https://mobile.fmcsa.dot.gov/qc/services",
		WebKey:     webKey,
		HTTPClient: &http.Client{Timeout: 15 * time.Second},
		UserAgent:  "OpenHaulGuard/" + version + " (+https://github.com/openhaulguard/openhaulguard)",
	}
}

func (c *Client) ValidateWebKey(ctx context.Context) error {
	_, err := c.get(ctx, "/carriers/name/greyhound")
	return err
}

func (c *Client) Docket(ctx context.Context, docket string) (RawResponse, error) {
	return c.get(ctx, "/carriers/docket-number/"+url.PathEscape(docket)+"/")
}

func (c *Client) Carrier(ctx context.Context, dot string) (RawResponse, error) {
	return c.get(ctx, "/carriers/"+url.PathEscape(dot))
}

func (c *Client) Basics(ctx context.Context, dot string) (RawResponse, error) {
	return c.get(ctx, "/carriers/"+url.PathEscape(dot)+"/basics")
}

func (c *Client) Authority(ctx context.Context, dot string) (RawResponse, error) {
	return c.get(ctx, "/carriers/"+url.PathEscape(dot)+"/authority")
}

func (c *Client) DocketNumbers(ctx context.Context, dot string) (RawResponse, error) {
	return c.get(ctx, "/carriers/"+url.PathEscape(dot)+"/docket-numbers")
}

func (c *Client) OOS(ctx context.Context, dot string) (RawResponse, error) {
	return c.get(ctx, "/carriers/"+url.PathEscape(dot)+"/oos")
}

func (c *Client) get(ctx context.Context, endpoint string) (RawResponse, error) {
	start := time.Now()
	u, err := url.Parse(strings.TrimRight(c.BaseURL, "/") + endpoint)
	if err != nil {
		return RawResponse{}, err
	}
	q := u.Query()
	q.Set("webKey", c.WebKey)
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return RawResponse{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.UserAgent)
	resp, err := c.HTTPClient.Do(req)
	fetch := domain.SourceFetchResult{
		SourceName:         SourceName,
		Endpoint:           endpoint,
		RequestMethod:      http.MethodGet,
		RequestURLRedacted: redactWebKey(u.String()),
		FetchedAt:          time.Now().UTC().Format(time.RFC3339),
		DurationMS:         time.Since(start).Milliseconds(),
	}
	if err != nil {
		fetch.ErrorCode = "OHG_SOURCE_UNAVAILABLE"
		fetch.ErrorMessage = err.Error()
		return RawResponse{Endpoint: endpoint, Fetch: fetch}, err
	}
	defer resp.Body.Close()
	fetch.StatusCode = resp.StatusCode
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	fetch.ResponseHash = hashRaw(body)
	if readErr != nil {
		fetch.ErrorCode = "OHG_SOURCE_UNAVAILABLE"
		fetch.ErrorMessage = readErr.Error()
		return RawResponse{Endpoint: endpoint, Body: body, Fetch: fetch}, readErr
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		fetch.ErrorCode = "OHG_AUTH_FMCSA_INVALID"
		fetch.ErrorMessage = fmt.Sprintf("FMCSA returned HTTP %d", resp.StatusCode)
		return RawResponse{Endpoint: endpoint, Body: body, Fetch: fetch}, fmt.Errorf("%s", fetch.ErrorMessage)
	}
	if resp.StatusCode == http.StatusNotFound {
		fetch.ErrorCode = "OHG_SOURCE_NOT_FOUND"
		fetch.ErrorMessage = "carrier not found"
		return RawResponse{Endpoint: endpoint, Body: body, Fetch: fetch}, fmt.Errorf("%s", fetch.ErrorMessage)
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		fetch.ErrorCode = "OHG_SOURCE_RATE_LIMITED"
		fetch.ErrorMessage = "FMCSA rate limited the request"
		return RawResponse{Endpoint: endpoint, Body: body, Fetch: fetch}, fmt.Errorf("%s", fetch.ErrorMessage)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		fetch.ErrorCode = "OHG_SOURCE_UNAVAILABLE"
		fetch.ErrorMessage = fmt.Sprintf("FMCSA returned HTTP %d", resp.StatusCode)
		return RawResponse{Endpoint: endpoint, Body: body, Fetch: fetch}, fmt.Errorf("%s", fetch.ErrorMessage)
	}
	return RawResponse{Endpoint: endpoint, Body: body, Fetch: fetch}, nil
}

func hashRaw(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func redactWebKey(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return strings.ReplaceAll(raw, "webKey=", "webKey=REDACTED")
	}
	q := u.Query()
	if q.Has("webKey") {
		q.Set("webKey", "REDACTED")
	}
	u.RawQuery = q.Encode()
	return u.String()
}
