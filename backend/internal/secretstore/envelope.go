// Package secretstore implements the backend-only, envelope-encrypted
// credential store selected by ADR-0003 (Option A).
//
// Guarantees:
//   - The master key is provisioned outside the application and read from a
//     restrictive file; it is never accepted from a browser, an API request, a
//     command-line flag or committed configuration. A missing or unreadable key
//     fails closed.
//   - Envelopes use a reviewed authenticated-encryption primitive (AES-256-GCM
//     from the Go standard library). No custom cryptography is invented.
//   - Per-record random data keys and nonces; every envelope is bound to its
//     reference ID and carries an explicit key ID.
//   - Only the backend resolves values. No value is returned to the browser or
//     written to logs, and the ciphertext is stored in the metadata database
//     while the keys stay in a separate file (key separation).
package secretstore

import (
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
)

// envelopeVersion is the envelope format version. Bump only for a breaking
// envelope change.
const envelopeVersion = 1

// algorithmID names the authenticated cipher used for the key-wrapping layer.
const algorithmID = "AES256-GCM"

// wrapInfo binds a wrapped data key to this purpose, so a wrapped key can never
// be replayed in a different context.
const wrapInfo = "telegraf-toolkit/secretstore/v1/data-key"

// Errors are deliberately generic so a caller cannot distinguish "wrong key"
// from "tampered" — both are simply a failed authentication.
var (
	// ErrEnvelopeMalformed indicates the stored envelope could not be parsed.
	ErrEnvelopeMalformed = errors.New("secret envelope is malformed")
	// ErrEnvelopeAuthFailed indicates the envelope failed authentication, which
	// means it was tampered with, truncated, or bound to a different key.
	ErrEnvelopeAuthFailed = errors.New("secret envelope failed authentication")
	// ErrWrongKeyID indicates the envelope was wrapped by a key this store does
	// not currently hold.
	ErrWrongKeyID = errors.New("secret envelope references an unknown key ID")
	// ErrInvalidKey indicates the supplied master key is not a valid key.
	ErrInvalidKey = errors.New("invalid master key")
)

// envelope is the parsed form of a stored ciphertext envelope.
type envelope struct {
	version      int
	algorithm    string
	keyID        string
	wrappedDEK   []byte
	dekNonce     []byte
	valueNonce   []byte
	valueCT      []byte
	referenceTag string
}

// encode serializes the envelope. Fields are separated by '.'; byte fields are
// base64url without padding.
func (e envelope) encode() (string, error) {
	if e.keyID == "" {
		return "", ErrEnvelopeMalformed
	}
	parts := []string{
		strconv.Itoa(e.version),
		e.algorithm,
		e.keyID,
		b64(e.wrappedDEK),
		b64(e.dekNonce),
		b64(e.valueNonce),
		b64(e.valueCT),
		e.referenceTag,
	}
	return strings.Join(parts, "."), nil
}

// parseEnvelope parses a stored envelope. It validates structure only;
// authenticity is proven later by the GCM open.
func parseEnvelope(raw string) (envelope, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 8 {
		return envelope{}, ErrEnvelopeMalformed
	}
	version, err := strconv.Atoi(parts[0])
	if err != nil || version != envelopeVersion {
		return envelope{}, ErrEnvelopeMalformed
	}
	if parts[1] != algorithmID || parts[2] == "" {
		return envelope{}, ErrEnvelopeMalformed
	}
	var out envelope
	out.version = version
	out.algorithm = parts[1]
	out.keyID = parts[2]
	if out.wrappedDEK, err = unb64(parts[3]); err != nil {
		return envelope{}, err
	}
	if out.dekNonce, err = unb64(parts[4]); err != nil {
		return envelope{}, err
	}
	if out.valueNonce, err = unb64(parts[5]); err != nil {
		return envelope{}, err
	}
	if out.valueCT, err = unb64(parts[6]); err != nil {
		return envelope{}, err
	}
	out.referenceTag = parts[7]
	if out.referenceTag == "" {
		return envelope{}, ErrEnvelopeMalformed
	}
	return out, nil
}

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func unb64(s string) ([]byte, error) {
	if s == "" {
		return nil, ErrEnvelopeMalformed
	}
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, ErrEnvelopeMalformed
	}
	return b, nil
}
