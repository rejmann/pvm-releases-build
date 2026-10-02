// Command releasecheck discovers every PHP minor branch at or above
// -min-version, compares each branch's current latest patch (from php.net)
// against the php-* tags already published on this repository, and prints
// the missing ones as a JSON array of version strings — the shape
// check-releases.yml feeds into repository_dispatch events for build.yml.
//
// Branches are discovered dynamically from php-src's own maintenance
// branches (repos/php/php-src/branches on GitHub), not hardcoded here: a
// new branch (an eventual 8.6, or lowering -min-version to backfill 7.0-7.3
// or older) needs no code change, just evidence that build.yml's extension
// set (build/extensions.txt) actually compiles there — see the -min-version
// flag doc below for why the default is conservative.
//
// https://www.php.net/releases/index.php?json&version=<branch> only lists
// the latest patch of that branch (there is no single endpoint enumerating
// every patch across all branches), so releasecheck queries it once per
// discovered branch. Confirmed live on 2026-10-01 for both a current branch
// (8.4 -> 8.4.26) and a 15-year-old one (5.6 -> 5.6.40): same shape.
//
// Usage:
//
//	releasecheck -published 8.2.34,8.3.35,8.4.25
//	releasecheck -published "" -min-version 8.0
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	branchFeedURLTemplate = "https://www.php.net/releases/index.php?json&version=%s"
	phpSrcBranchesAPI     = "https://api.github.com/repos/php/php-src/branches?per_page=100"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "releasecheck:", err)
		os.Exit(1)
	}
}

func run(args []string, out io.Writer, warn io.Writer) error {
	fs := flag.NewFlagSet("releasecheck", flag.ContinueOnError)
	published := fs.String("published", "", "comma-separated versions already published here (e.g. from `gh release list`)")
	minVersion := fs.String("min-version", "7.4", "lowest PHP branch (major.minor) to consider\n"+
		"Deliberately conservative: only PHP 7.4+ has been verified to compile with this repo's\n"+
		"extension set (see build/extensions.txt). Lowering this backfills older branches\n"+
		"automatically on the next check-releases.yml run — but an unbuildable branch is retried\n"+
		"forever (check-releases.yml only stops once a version is actually published), so don't\n"+
		"lower it without first confirming the branch builds.")
	if err := fs.Parse(args); err != nil {
		return err
	}

	have := map[string]bool{}
	for _, v := range splitNonEmpty(*published) {
		have[v] = true
	}

	branches, err := discoverBranches(httpGet, *minVersion)
	if err != nil {
		return fmt.Errorf("discover branches: %w", err)
	}

	var missing []string
	for _, branch := range branches {
		latest, err := fetchBranchLatest(httpGet, branch)
		if err != nil {
			// A branch with no release yet (e.g. a future major still in
			// development) or a transient fetch error shouldn't block
			// checking every other branch.
			fmt.Fprintf(warn, "releasecheck: skipping branch %s: %v\n", branch, err)
			continue
		}
		if latest != "" && !have[latest] {
			missing = append(missing, latest)
		}
	}

	enc := json.NewEncoder(out)
	return enc.Encode(missing)
}

// httpGetter abstracts the one thing discoverBranches/fetchBranchLatest need
// from net/http, so both can share retry/auth handling in one place.
type httpGetter func(url string) (*http.Response, error)

func httpGet(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if token := firstNonEmpty(os.Getenv("GITHUB_TOKEN"), os.Getenv("GH_TOKEN")); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := &http.Client{Timeout: 15 * time.Second}
	return client.Do(req)
}

// minorBranchPattern matches php-src's maintenance branches for a released
// minor version, e.g. "PHP-8.4" or "PHP-7.4" — not individual patch tags
// like "PHP-8.4.1" and not pre-release branches like "PHP-7.1.0RC1".
var minorBranchPattern = regexp.MustCompile(`^PHP-(\d+)\.(\d+)$`)

// discoverBranches lists php-src's branches on GitHub, keeps only clean
// "PHP-X.Y" maintenance branches, and returns the "X.Y" versions at or
// above minVersion, sorted ascending.
func discoverBranches(get httpGetter, minVersion string) ([]string, error) {
	minMajor, minMinor, err := parseMajorMinor(minVersion)
	if err != nil {
		return nil, fmt.Errorf("-min-version: %w", err)
	}

	var branches []string
	url := phpSrcBranchesAPI
	for url != "" {
		resp, err := get(url)
		if err != nil {
			return nil, err
		}
		body, next, err := readBranchPage(resp)
		if err != nil {
			return nil, err
		}
		for _, name := range body {
			m := minorBranchPattern.FindStringSubmatch(name)
			if m == nil {
				continue
			}
			major, _ := strconv.Atoi(m[1])
			minor, _ := strconv.Atoi(m[2])
			if major < minMajor || (major == minMajor && minor < minMinor) {
				continue
			}
			branches = append(branches, fmt.Sprintf("%d.%d", major, minor))
		}
		url = next
	}

	sortVersions(branches)
	return branches, nil
}

type ghBranch struct {
	Name string `json:"name"`
}

// readBranchPage decodes one page of the branches API response and returns
// the next page's URL (from the Link header), or "" if this was the last page.
func readBranchPage(resp *http.Response) (names []string, next string, err error) {
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("GET branches page: status %s", resp.Status)
	}
	var page []ghBranch
	if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
		return nil, "", fmt.Errorf("decode branches page: %w", err)
	}
	for _, b := range page {
		names = append(names, b.Name)
	}
	return names, nextPageURL(resp.Header.Get("Link")), nil
}

// nextPageURL parses a GitHub "Link" header for the rel="next" URL, per
// https://docs.github.com/en/rest/using-the-rest-api/using-pagination-in-the-rest-api.
func nextPageURL(link string) string {
	for _, part := range strings.Split(link, ",") {
		segments := strings.Split(part, ";")
		if len(segments) < 2 {
			continue
		}
		if !strings.Contains(segments[1], `rel="next"`) {
			continue
		}
		url := strings.TrimSpace(segments[0])
		url = strings.TrimPrefix(url, "<")
		url = strings.TrimSuffix(url, ">")
		return url
	}
	return ""
}

// branchFeed is the subset of
// https://www.php.net/releases/index.php?json&version=<branch> this tool
// needs: the version of the newest release on that branch.
type branchFeed struct {
	Version string `json:"version"`
}

func fetchBranchLatest(get httpGetter, branch string) (string, error) {
	url := fmt.Sprintf(branchFeedURLTemplate, branch)
	resp, err := get(url)
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
	if feed.Version == "" {
		return "", fmt.Errorf("no release found for branch %s", branch)
	}
	return feed.Version, nil
}

func parseMajorMinor(v string) (major, minor int, err error) {
	parts := strings.SplitN(v, ".", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("expected major.minor, got %q", v)
	}
	major, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid major in %q: %w", v, err)
	}
	minor, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid minor in %q: %w", v, err)
	}
	return major, minor, nil
}

func sortVersions(versions []string) {
	less := func(i, j int) bool {
		ai, bi, _ := parseMajorMinor(versions[i])
		aj, bj, _ := parseMajorMinor(versions[j])
		if ai != aj {
			return ai < aj
		}
		return bi != bj && bi < bj
	}
	// Small lists (a few dozen branches at most); insertion sort keeps this
	// dependency-free and the comparator above pure and easy to unit test.
	for i := 1; i < len(versions); i++ {
		for j := i; j > 0 && less(j, j-1); j-- {
			versions[j], versions[j-1] = versions[j-1], versions[j]
		}
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
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
