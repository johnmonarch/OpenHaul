package socrata

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/openhaulguard/openhaulguard/internal/domain"
	"github.com/openhaulguard/openhaulguard/internal/sources/registry"
)

const SourceName = "dot_datahub_socrata"

type Client struct {
	BaseURL    string
	AppToken   string
	HTTPClient *http.Client
	UserAgent  string
}

type RawResponse struct {
	Dataset  registry.Dataset
	Endpoint string
	Body     []byte
	Fetch    domain.SourceFetchResult
}

func New(appToken, version string) *Client {
	return &Client{
		BaseURL:    "https://data.transportation.gov",
		AppToken:   appToken,
		HTTPClient: &http.Client{Timeout: 15 * time.Second},
		UserAgent:  "OpenHaulGuard/" + version + " (+https://github.com/openhaulguard/openhaulguard)",
	}
}

func (c *Client) CompanyCensus(ctx context.Context, query url.Values) (RawResponse, error) {
	return c.Rows(ctx, registry.MustDatasetByKey(registry.CompanyCensusKey), query)
}

func (c *Client) Rows(ctx context.Context, dataset registry.Dataset, query url.Values) (RawResponse, error) {
	endpoint := "/resource/" + url.PathEscape(dataset.ID) + ".json"
	start := time.Now()
	u, err := url.Parse(strings.TrimRight(c.BaseURL, "/") + endpoint)
	if err != nil {
		return RawResponse{}, err
	}
	if query != nil {
		q := u.Query()
		for key, values := range query {
			for _, value := range values {
				q.Add(key, value)
			}
		}
		u.RawQuery = q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return RawResponse{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.UserAgent)
	if c.AppToken != "" {
		req.Header.Set("X-App-Token", c.AppToken)
	}
	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	fetch := domain.SourceFetchResult{
		SourceName:         SourceName,
		Endpoint:           endpoint,
		RequestMethod:      http.MethodGet,
		RequestURLRedacted: u.String(),
		FetchedAt:          time.Now().UTC().Format(time.RFC3339),
		DurationMS:         time.Since(start).Milliseconds(),
	}
	if err != nil {
		fetch.ErrorCode = "OHG_SOURCE_UNAVAILABLE"
		fetch.ErrorMessage = err.Error()
		return RawResponse{Dataset: dataset, Endpoint: endpoint, Fetch: fetch}, err
	}
	defer resp.Body.Close()
	fetch.StatusCode = resp.StatusCode
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	fetch.ResponseHash = hashRaw(body)
	if readErr != nil {
		fetch.ErrorCode = "OHG_SOURCE_UNAVAILABLE"
		fetch.ErrorMessage = readErr.Error()
		return RawResponse{Dataset: dataset, Endpoint: endpoint, Body: body, Fetch: fetch}, readErr
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		fetch.ErrorCode = "OHG_SOURCE_RATE_LIMITED"
		fetch.ErrorMessage = "Socrata rate limited the request"
		return RawResponse{Dataset: dataset, Endpoint: endpoint, Body: body, Fetch: fetch}, errors.New(fetch.ErrorMessage)
	}
	if resp.StatusCode == http.StatusNotFound {
		fetch.ErrorCode = "OHG_SOURCE_NOT_FOUND"
		fetch.ErrorMessage = "dataset or row not found"
		return RawResponse{Dataset: dataset, Endpoint: endpoint, Body: body, Fetch: fetch}, errors.New(fetch.ErrorMessage)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		fetch.ErrorCode = "OHG_SOURCE_UNAVAILABLE"
		fetch.ErrorMessage = fmt.Sprintf("Socrata returned HTTP %d", resp.StatusCode)
		return RawResponse{Dataset: dataset, Endpoint: endpoint, Body: body, Fetch: fetch}, errors.New(fetch.ErrorMessage)
	}
	return RawResponse{Dataset: dataset, Endpoint: endpoint, Body: body, Fetch: fetch}, nil
}

func hashRaw(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}
