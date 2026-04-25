package normalize

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"strings"
)

var identifierRE = regexp.MustCompile(`(?i)^\s*(MC|MX|FF|USDOT|DOT)?\s*#?\s*-?\s*([A-Z0-9]+)\s*$`)

func Identifier(kind, value string) (string, string, error) {
	kind = strings.ToLower(strings.TrimSpace(kind))
	value = strings.TrimSpace(value)
	if kind == "" {
		matches := identifierRE.FindStringSubmatch(value)
		if len(matches) == 3 {
			kind = strings.ToLower(matches[1])
			value = matches[2]
		}
	}
	switch kind {
	case "usdot":
		kind = "dot"
	case "":
		kind = "dot"
	}
	if kind != "mc" && kind != "mx" && kind != "ff" && kind != "dot" && kind != "name" {
		return "", "", ErrInvalidIdentifier
	}
	if kind != "name" {
		value = digitsOnly(value)
	}
	if value == "" {
		return "", "", ErrInvalidIdentifier
	}
	return kind, value, nil
}

var ErrInvalidIdentifier = invalidIdentifierError{}

type invalidIdentifierError struct{}

func (invalidIdentifierError) Error() string { return "invalid carrier identifier" }

func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func Phone(s string) string {
	d := digitsOnly(s)
	if len(d) == 10 {
		return "+1" + d
	}
	if len(d) == 11 && strings.HasPrefix(d, "1") {
		return "+" + d
	}
	return strings.TrimSpace(s)
}

func ComparableString(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	replacer := strings.NewReplacer(".", "", ",", "", "#", "", "-", " ", "  ", " ")
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return replacer.Replace(s)
}

func HashRaw(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func HashNormalized(v any) (string, error) {
	body, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}
