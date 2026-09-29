// Command releasecheck compares the patch releases php.net currently lists
// against the php-* tags already published on this repository, and prints
// the missing ones as a JSON array of version strings — the shape
// check-releases.yml feeds into a workflow_dispatch matrix for build.yml.
//
// https://www.php.net/releases/index.php?json&version=<branch> only lists
// the latest patch of that branch (there is no single endpoint enumerating
// every patch across all branches), so releasecheck queries it once per
// supported branch. Confirmed live on 2026-09-29: version=8.4 returns
// {"version":"8.4.26", "source":[{"filename":"php-8.4.26.tar.gz", "sha256":"...", ...}, ...]}.
//
// Usage:
//
//	releasecheck -published 8.2.34,8.3.35,8.4.25 -branches 8.2,8.3,8.4,8.5
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const branchFeedURLTemplate = "https://www.php.net/releases/index.php?json&version=%s"

func main() {
	if err := run(os.Args[1:], os.Stdout, fetchBranchLatest); err != nil {
		fmt.Fprintln(os.Stderr, "releasecheck:", err)
		os.Exit(1)
	}
}

type fetcher func(branch string) (string, error)

func run(args []string, out io.Writer, fetch fetcher) error {
	fs := flag.NewFlagSet("releasecheck", flag.ContinueOnError)
	published := fs.String("published", "", "comma-separated versions already published here (e.g. from `gh release list`)")
	branches := fs.String("branches", "8.2,8.3,8.4,8.5", "comma-separated PHP branches to check, e.g. 8.4")
	if err := fs.Parse(args); err != nil {
		return err
	}

	have := map[string]bool{}
	for _, v := range splitNonEmpty(*published) {
		have[v] = true
	}

	var missing []string
	for _, branch := range splitNonEmpty(*branches) {
		latest, err := fetch(branch)
		if err != nil {
			return fmt.Errorf("branch %s: %w", branch, err)
		}
		if latest != "" && !have[latest] {
			missing = append(missing, latest)
		}
	}

	enc := json.NewEncoder(out)
	return enc.Encode(missing)
}

// branchFeed is the subset of https://www.php.net/releases/<branch>/feed.json
// this tool needs: the version of the newest release on that branch.
type branchFeed struct {
	Version string `json:"version"`
}

func fetchBranchLatest(branch string) (string, error) {
	url := fmt.Sprintf(branchFeedURLTemplate, branch)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET %s: status %s", url, resp.Status)
	}
	var feed branchFeed
	if err := json.NewDecoder(resp.Body).Decode(&feed); err != nil {
		return "", fmt.Errorf("decode %s: %w", url, err)
	}
	return feed.Version, nil
}

func splitNonEmpty(csv string) []string {
	var out []string
	for _, s := range strings.Split(csv, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}
