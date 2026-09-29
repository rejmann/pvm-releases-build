// Package manifest defines the manifest.json format this repository publishes
// and signs, matching manifest.schema.json at the repo root. pvm embeds the
// public half of the signing key and verifies every manifest against it
// before trusting any URL or checksum inside.
package manifest

import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Schema is the only value pvm's manifest.schema (field "schema") currently accepts.
const Schema = 1

// Manifest is the signed index of PHP builds. Sign produces the bytes that go
// into manifest.json; the accompanying manifest.json.sig holds the signature
// over exactly those bytes.
type Manifest struct {
	Schema    int       `json:"schema"`
	Generated time.Time `json:"generated"`
	Releases  []Release `json:"releases"`
}

type Release struct {
	Version      string     `json:"version"`
	SourceSHA256 string     `json:"source_sha256"`
	Artifacts    []Artifact `json:"artifacts"`
}

type Artifact struct {
	OS               string   `json:"os"`
	Arch             string   `json:"arch"`
	Libc             *string  `json:"libc"`
	Linkage          string   `json:"linkage"`
	URL              string   `json:"url"`
	SHA256           string   `json:"sha256"`
	Size             int64    `json:"size"`
	Format           string   `json:"format"`
	DL               bool     `json:"dl"`
	MinGlibc         *string  `json:"min_glibc"`
	MinMacOS         *string  `json:"min_macos"`
	Extensions       []string `json:"extensions"`
	SharedExtensions []string `json:"shared_extensions"`
}

// Marshal produces the canonical JSON bytes that get written to manifest.json
// and signed. Indented and newline-terminated so it's stable to diff in git
// and pleasant to read on GitHub.
func Marshal(m Manifest) ([]byte, error) {
	if m.Schema == 0 {
		m.Schema = Schema
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// Sign signs the exact bytes that will be published as manifest.json. Callers
// must sign the same bytes Marshal produced, not a re-encoding of the struct.
func Sign(priv ed25519.PrivateKey, manifestJSON []byte) []byte {
	return ed25519.Sign(priv, manifestJSON)
}

// Verify checks a manifest.json payload against its detached signature and
// the embedded public key. This is what pvm itself will run before trusting
// the manifest; it lives here too so the signer and the verifier can never
// drift apart.
func Verify(pub ed25519.PublicKey, manifestJSON, sig []byte) error {
	if len(pub) != ed25519.PublicKeySize {
		return fmt.Errorf("manifest: public key must be %d bytes, got %d", ed25519.PublicKeySize, len(pub))
	}
	if !ed25519.Verify(pub, manifestJSON, sig) {
		return errors.New("manifest: signature verification failed")
	}
	var m Manifest
	if err := json.Unmarshal(manifestJSON, &m); err != nil {
		return fmt.Errorf("manifest: %w", err)
	}
	if m.Schema != Schema {
		return fmt.Errorf("manifest: unsupported schema %d, this tool understands %d", m.Schema, Schema)
	}
	return nil
}
