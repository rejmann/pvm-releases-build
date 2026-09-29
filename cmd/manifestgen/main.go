// Command manifestgen assembles manifest.json from per-artifact JSON
// descriptors emitted by the build matrix, merges it with the previous
// manifest (so releases already published stay in the index), signs the
// result with the ed25519 key in MANIFEST_SIGNING_KEY, and writes both the
// manifest and its detached signature to disk.
//
// Usage:
//
//	manifestgen \
//	  -artifacts build/manifest-inputs/*.json \
//	  -previous manifest.json \
//	  -out manifest.json \
//	  -sig manifest.json.sig
//
// The signing key is read from the MANIFEST_SIGNING_KEY environment
// variable: a base64-encoded 64-byte ed25519 private key (the format
// scripts/gen-signing-key.sh produces). It is never written to disk here.
package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rejmann/pvm-releases-build/internal/manifest"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "manifestgen:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("manifestgen", flag.ContinueOnError)
	artifactsGlob := fs.String("artifacts", "", "glob matching per-artifact JSON descriptors to merge in")
	previousPath := fs.String("previous", "", "path to the previously published manifest.json (optional; omit for the first ever release)")
	outPath := fs.String("out", "manifest.json", "path to write the signed manifest to")
	sigPath := fs.String("sig", "manifest.json.sig", "path to write the detached signature to")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *artifactsGlob == "" {
		return fmt.Errorf("-artifacts is required")
	}

	priv, err := loadSigningKey()
	if err != nil {
		return err
	}

	m, err := loadPrevious(*previousPath)
	if err != nil {
		return err
	}

	inputs, err := loadArtifactInputs(*artifactsGlob)
	if err != nil {
		return err
	}
	for _, in := range inputs {
		m = mergeArtifact(m, in)
	}
	sortReleases(m.Releases)

	m.Generated = time.Now().UTC()
	payload, err := manifest.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	sig := manifest.Sign(priv, payload)

	if err := os.WriteFile(*outPath, payload, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", *outPath, err)
	}
	if err := os.WriteFile(*sigPath, []byte(base64.StdEncoding.EncodeToString(sig)+"\n"), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", *sigPath, err)
	}
	fmt.Printf("wrote %s (%d releases) and %s\n", *outPath, len(m.Releases), *sigPath)
	return nil
}

// loadSigningKey reads MANIFEST_SIGNING_KEY: base64 of a 64-byte ed25519
// private key, as produced by scripts/gen-signing-key.sh. The workflow only
// ever has this in the environment for the duration of this one step.
func loadSigningKey() (ed25519.PrivateKey, error) {
	encoded := strings.TrimSpace(os.Getenv("MANIFEST_SIGNING_KEY"))
	if encoded == "" {
		return nil, fmt.Errorf("MANIFEST_SIGNING_KEY is not set (see SETUP.md for how to generate and configure it)")
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("MANIFEST_SIGNING_KEY: not valid base64: %w", err)
	}
	if len(raw) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("MANIFEST_SIGNING_KEY: expected %d raw bytes, got %d", ed25519.PrivateKeySize, len(raw))
	}
	return ed25519.PrivateKey(raw), nil
}

func loadPrevious(path string) (manifest.Manifest, error) {
	if path == "" {
		return manifest.Manifest{Schema: manifest.Schema}, nil
	}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return manifest.Manifest{Schema: manifest.Schema}, nil
	}
	if err != nil {
		return manifest.Manifest{}, fmt.Errorf("read previous manifest: %w", err)
	}
	var m manifest.Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return manifest.Manifest{}, fmt.Errorf("parse previous manifest: %w", err)
	}
	return m, nil
}

// artifactInput is one build target's descriptor, written by the build job
// after it uploads the tarball to the GitHub Release.
type artifactInput struct {
	Version      string            `json:"version"`
	SourceSHA256 string            `json:"source_sha256"`
	Artifact     manifest.Artifact `json:"artifact"`
}

func loadArtifactInputs(glob string) ([]artifactInput, error) {
	matches, err := filepath.Glob(glob)
	if err != nil {
		return nil, fmt.Errorf("glob %s: %w", glob, err)
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no artifact descriptors matched %s", glob)
	}
	sort.Strings(matches)

	inputs := make([]artifactInput, 0, len(matches))
	for _, path := range matches {
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		var in artifactInput
		if err := json.Unmarshal(b, &in); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		if in.Version == "" {
			return nil, fmt.Errorf("%s: missing version", path)
		}
		inputs = append(inputs, in)
	}
	return inputs, nil
}

// mergeArtifact adds in's artifact to the release for in.Version, replacing
// any existing artifact for the same os/arch/libc so a re-run overwrites
// rather than duplicates.
func mergeArtifact(m manifest.Manifest, in artifactInput) manifest.Manifest {
	for i := range m.Releases {
		if m.Releases[i].Version != in.Version {
			continue
		}
		r := &m.Releases[i]
		if r.SourceSHA256 == "" {
			r.SourceSHA256 = in.SourceSHA256
		}
		for j := range r.Artifacts {
			if sameTarget(r.Artifacts[j], in.Artifact) {
				r.Artifacts[j] = in.Artifact
				return m
			}
		}
		r.Artifacts = append(r.Artifacts, in.Artifact)
		return m
	}
	m.Releases = append(m.Releases, manifest.Release{
		Version:      in.Version,
		SourceSHA256: in.SourceSHA256,
		Artifacts:    []manifest.Artifact{in.Artifact},
	})
	return m
}

func sameTarget(a, b manifest.Artifact) bool {
	libcEqual := (a.Libc == nil) == (b.Libc == nil) && (a.Libc == nil || *a.Libc == *b.Libc)
	return a.OS == b.OS && a.Arch == b.Arch && libcEqual
}

func sortReleases(releases []manifest.Release) {
	sort.Slice(releases, func(i, j int) bool {
		return versionLess(releases[i].Version, releases[j].Version)
	})
}

// versionLess compares two "X.Y.Z" PHP versions numerically per component,
// since a plain string sort would put "8.4.9" after "8.4.26".
func versionLess(a, b string) bool {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(pa) && i < len(pb); i++ {
		na, _ := strconv.Atoi(pa[i])
		nb, _ := strconv.Atoi(pb[i])
		if na != nb {
			return na < nb
		}
	}
	return len(pa) < len(pb)
}
