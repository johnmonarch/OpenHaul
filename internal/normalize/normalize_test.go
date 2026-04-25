package normalize

import "testing"

func TestIdentifierNormalization(t *testing.T) {
	tests := []struct {
		inType    string
		inValue   string
		wantType  string
		wantValue string
	}{
		{"", "MC123456", "mc", "123456"},
		{"", "MC-123456", "mc", "123456"},
		{"", "mx 123456", "mx", "123456"},
		{"", "DOT 1234567", "dot", "1234567"},
		{"", "USDOT#1234567", "dot", "1234567"},
		{"mc", "MC-123456", "mc", "123456"},
	}
	for _, tt := range tests {
		gotType, gotValue, err := Identifier(tt.inType, tt.inValue)
		if err != nil {
			t.Fatalf("Identifier(%q, %q) returned error: %v", tt.inType, tt.inValue, err)
		}
		if gotType != tt.wantType || gotValue != tt.wantValue {
			t.Fatalf("Identifier(%q, %q) = %s %s, want %s %s", tt.inType, tt.inValue, gotType, gotValue, tt.wantType, tt.wantValue)
		}
	}
}

func TestPhoneNormalization(t *testing.T) {
	tests := map[string]string{
		"(555) 555-5555":  "+15555555555",
		"555-555-5555":    "+15555555555",
		"+1 555 555 5555": "+15555555555",
	}
	for input, want := range tests {
		if got := Phone(input); got != want {
			t.Fatalf("Phone(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestHashNormalizedIgnoresMapOrder(t *testing.T) {
	left := map[string]any{"b": 2, "a": 1}
	right := map[string]any{"a": 1, "b": 2}
	leftHash, err := HashNormalized(left)
	if err != nil {
		t.Fatal(err)
	}
	rightHash, err := HashNormalized(right)
	if err != nil {
		t.Fatal(err)
	}
	if leftHash != rightHash {
		t.Fatalf("hashes differ for same map content: %s vs %s", leftHash, rightHash)
	}
}
