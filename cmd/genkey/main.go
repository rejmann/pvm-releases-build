// Command genkey generates the ed25519 keypair that signs manifest.json.
// It is meant to be run once, locally, by whoever holds the repo's admin
// access — not by CI. It never writes the private key to disk: the private
// key is only ever printed to stdout, base64-encoded, so the operator can
// pipe it straight into `gh secret set` (see SETUP.md) without it touching
// the filesystem. The public key is written to keys/manifest-signing.pub,
// which is meant to be committed — it's what gets embedded in pvm later.
//
// Usage:
//
//	go run ./cmd/genkey | gh secret set MANIFEST_SIGNING_KEY --repo rejmann/pvm-releases-build
//
// That pipes the private key directly to gh; the public key still lands at
// keys/manifest-signing.pub for you to `git add` and commit. Re-running this
// overwrites keys/manifest-signing.pub — if a key already exists, rotating
// it means every previously published manifest.json stops verifying against
// the new pvm build until re-signed, so don't run this casually.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
)

func main() {
	pubPath := flag.String("pub", "keys/manifest-signing.pub", "path to write the public key to")
	flag.Parse()

	if _, err := os.Stat(*pubPath); err == nil {
		fmt.Fprintf(os.Stderr, "genkey: %s already exists — refusing to overwrite it silently.\n", *pubPath)
		fmt.Fprintf(os.Stderr, "        Remove it first if you really mean to rotate the signing key\n")
		fmt.Fprintf(os.Stderr, "        (see the rotation note in this command's doc comment first).\n")
		os.Exit(1)
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "genkey:", err)
		os.Exit(1)
	}

	pubB64 := base64.StdEncoding.EncodeToString(pub)
	if err := os.WriteFile(*pubPath, []byte(pubB64+"\n"), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "genkey:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "genkey: wrote public key to %s — git add and commit it.\n", *pubPath)
	fmt.Fprintf(os.Stderr, "genkey: printing the PRIVATE key to stdout now. Pipe it directly into\n")
	fmt.Fprintf(os.Stderr, "genkey: `gh secret set MANIFEST_SIGNING_KEY`; do not save it to a file.\n\n")

	// Only the private key goes to stdout, so a `| gh secret set ...` pipe
	// never picks up the status lines above (those are on stderr).
	fmt.Println(base64.StdEncoding.EncodeToString(priv))
}
