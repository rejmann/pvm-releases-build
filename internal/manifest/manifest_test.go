package manifest

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"
)

func testManifest() Manifest {
	musl := "musl"
	return Manifest{
		Schema:    Schema,
		Generated: time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC),
		Releases: []Release{{
			Version:      "8.4.26",
			SourceSHA256: "32a2de53000000000000000000000000000000000000000000000000000000",
			Artifacts: []Artifact{{
				OS:               "linux",
				Arch:             "amd64",
				Libc:             &musl,
				Linkage:          "static",
				URL:              "https://github.com/rejmann/pvm-releases-build/releases/download/php-8.4.26/php-8.4.26-linux-amd64-musl.tar.gz",
				SHA256:           "9f0a00000000000000000000000000000000000000000000000000000000a",
				Size:             31737684,
				Format:           "tar.gz",
				DL:               false,
				Extensions:       []string{"bcmath", "curl"},
				SharedExtensions: []string{},
			}},
		}},
	}
}

func TestSignAndVerifyRoundTrip(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	payload, err := Marshal(testManifest())
	if err != nil {
		t.Fatal(err)
	}
	sig := Sign(priv, payload)

	if err := Verify(pub, payload, sig); err != nil {
		t.Fatalf("Verify() = %v, want nil", err)
	}
}

func TestVerifyRejectsTamperedPayload(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	payload, err := Marshal(testManifest())
	if err != nil {
		t.Fatal(err)
	}
	sig := Sign(priv, payload)

	tampered := append([]byte(nil), payload...)
	tampered[len(tampered)-2] = 'X' // mutate a trailing byte, e.g. inside a hash

	if err := Verify(pub, tampered, sig); err == nil {
		t.Fatal("Verify() = nil, want error for tampered payload")
	}
}

func TestVerifyRejectsWrongKey(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	otherPub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	payload, err := Marshal(testManifest())
	if err != nil {
		t.Fatal(err)
	}
	sig := Sign(priv, payload)

	if err := Verify(otherPub, payload, sig); err == nil {
		t.Fatal("Verify() = nil, want error for signature from a different key")
	}
}

func TestVerifyRejectsUnknownSchema(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	m := testManifest()
	m.Schema = 2
	payload, err := Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	sig := Sign(priv, payload)

	if err := Verify(pub, payload, sig); err == nil {
		t.Fatal("Verify() = nil, want error for unknown schema version")
	}
}
