package secretstore

import (
	"strings"
	"testing"
)

func TestEnvelopeEncodeParseRoundTrip(t *testing.T) {
	orig := envelope{
		version:      envelopeVersion,
		algorithm:    algorithmID,
		keyID:        "k1",
		wrappedDEK:   []byte{1, 2, 3, 4},
		dekNonce:     []byte{5, 6, 7},
		valueNonce:   []byte{8, 9, 10},
		valueCT:      []byte{11, 12, 13, 14, 15},
		referenceTag: "cred_abc",
	}
	encoded, err := orig.encode()
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	parsed, err := parseEnvelope(encoded)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed.keyID != "k1" || parsed.referenceTag != "cred_abc" || parsed.version != envelopeVersion {
		t.Fatalf("round-trip mismatch: %+v", parsed)
	}
	if string(parsed.valueCT) != string(orig.valueCT) {
		t.Fatalf("ciphertext mismatch: %v vs %v", parsed.valueCT, orig.valueCT)
	}
}

func TestParseEnvelopeRejectsMalformed(t *testing.T) {
	cases := map[string]string{
		"empty":           "",
		"too few fields":  "1.AES256-GCM.k1",
		"too many fields": "1.AES256-GCM.k1.a.b.c.d.e.f",
		"bad version":     "9.AES256-GCM.k1.a.b.c.d.tag",
		"bad algorithm":   "1.ROT13.k1.a.b.c.d.tag",
		"empty key id":    "1.AES256-GCM..a.b.c.d.tag",
		"bad base64":      "1.AES256-GCM.k1.!!!.b.c.d.tag",
		"empty reference": "1.AES256-GCM.k1.a.b.c.d.",
	}
	for name, raw := range cases {
		if _, err := parseEnvelope(raw); err == nil {
			t.Errorf("%s: expected a parse error for %q", name, raw)
		}
	}
}

func TestEnvelopeEncodeRejectsEmptyKeyID(t *testing.T) {
	if _, err := (envelope{version: envelopeVersion, algorithm: algorithmID}).encode(); err == nil {
		t.Fatal("encoding an envelope without a key ID must fail")
	}
}

func TestFingerprintIsStableAndNonReversible(t *testing.T) {
	fp := fingerprint([]byte("some-value"))
	if fp == "" || !strings.HasPrefix(fp, "sha256:") {
		t.Fatalf("fingerprint = %q, want a sha256 prefix", fp)
	}
	if strings.Contains(fp, "some-value") {
		t.Fatal("fingerprint must not contain the value")
	}
	if fp != fingerprint([]byte("some-value")) {
		t.Fatal("fingerprint must be deterministic")
	}
	if fp == fingerprint([]byte("some-other-value")) {
		t.Fatal("different values must have different fingerprints")
	}
}
