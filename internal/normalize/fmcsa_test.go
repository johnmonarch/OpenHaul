package normalize

import (
	"os"
	"testing"
	"time"

	"github.com/openhaulguard/openhaulguard/internal/sources/fmcsa"
)

func TestFMCSAFixtureToCarrier(t *testing.T) {
	body, err := os.ReadFile("../../examples/fixtures/fmcsa_qcmobile/fmcsa_qcmobile_carrier_valid.json")
	if err != nil {
		t.Fatal(err)
	}
	observedAt := time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
	carrier, err := FMCSAResponsesToCarrier("mc", "123456", []fmcsa.RawResponse{{Endpoint: "fixture", Body: body}}, observedAt)
	if err != nil {
		t.Fatal(err)
	}
	if carrier.USDOTNumber != "1234567" {
		t.Fatalf("USDOT = %q", carrier.USDOTNumber)
	}
	if carrier.Contact.Phone != "+15555555555" {
		t.Fatalf("phone = %q", carrier.Contact.Phone)
	}
	if len(carrier.Authority) == 0 {
		t.Fatalf("expected authority record")
	}
}
