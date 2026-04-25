package socrata

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/openhaulguard/openhaulguard/internal/sources/registry"
)

func TestRowsSendsOptionalAppToken(t *testing.T) {
	transport := &recordingTransport{body: `[{"dot_number":"1234567"}]`}

	client := New("token-123", "test")
	client.BaseURL = "https://example.test"
	client.HTTPClient = &http.Client{Transport: transport}
	got, err := client.Rows(context.Background(), registry.MustDatasetByKey(registry.CompanyCensusKey), url.Values{"$limit": []string{"1"}})
	if err != nil {
		t.Fatal(err)
	}
	if transport.path != "/resource/az4n-8mr2.json" {
		t.Fatalf("path = %q", transport.path)
	}
	if transport.token != "token-123" {
		t.Fatalf("X-App-Token = %q", transport.token)
	}
	if transport.limit != "1" {
		t.Fatalf("$limit = %q", transport.limit)
	}
	if got.Fetch.SourceName != SourceName {
		t.Fatalf("source = %q", got.Fetch.SourceName)
	}
	if got.Fetch.ResponseHash == "" {
		t.Fatal("expected response hash")
	}
}

func TestRowsOmitsEmptyAppToken(t *testing.T) {
	transport := &recordingTransport{body: `[]`}

	client := New("", "test")
	client.BaseURL = "https://example.test"
	client.HTTPClient = &http.Client{Transport: transport}
	if _, err := client.CompanyCensus(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if transport.token != "" {
		t.Fatalf("X-App-Token = %q", transport.token)
	}
}

type recordingTransport struct {
	path  string
	token string
	limit string
	body  string
}

func (t *recordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.path = req.URL.Path
	t.token = req.Header.Get("X-App-Token")
	t.limit = req.URL.Query().Get("$limit")
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewBufferString(t.body)),
		Request:    req,
	}, nil
}
