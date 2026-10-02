package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

func jsonResponse(status int, body string, headers http.Header) *http.Response {
	if headers == nil {
		headers = http.Header{}
	}
	return &http.Response{
		StatusCode: status,
		Status:     fmt.Sprintf("%d", status),
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     headers,
	}
}

func TestDiscoverBranchesFiltersAndPaginates(t *testing.T) {
	page1 := `[
		{"name":"PHP-8.4"},
		{"name":"PHP-8.4.1"},
		{"name":"PHP-7.1.0RC1"},
		{"name":"PHP-5"}
	]`
	page2 := `[{"name":"PHP-7.4"},{"name":"PHP-8.5"}]`

	calls := 0
	get := func(url string) (*http.Response, error) {
		calls++
		switch calls {
		case 1:
			h := http.Header{"Link": {`<https://api.github.com/x?page=2>; rel="next"`}}
			return jsonResponse(200, page1, h), nil
		case 2:
			return jsonResponse(200, page2, nil), nil
		default:
			t.Fatalf("unexpected call #%d to %s", calls, url)
			return nil, nil
		}
	}

	branches, err := discoverBranches(get, "7.4")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"7.4", "8.4", "8.5"}
	if !stringsEqual(branches, want) {
		t.Errorf("discoverBranches() = %v, want %v", branches, want)
	}
}

func TestDiscoverBranchesRespectsMinVersion(t *testing.T) {
	get := func(url string) (*http.Response, error) {
		return jsonResponse(200, `[{"name":"PHP-7.4"},{"name":"PHP-8.0"},{"name":"PHP-8.1"}]`, nil), nil
	}
	branches, err := discoverBranches(get, "8.1")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"8.1"}
	if !stringsEqual(branches, want) {
		t.Errorf("discoverBranches() = %v, want %v", branches, want)
	}
}

func TestFetchBranchLatest(t *testing.T) {
	get := func(url string) (*http.Response, error) {
		return jsonResponse(200, `{"version":"8.4.26"}`, nil), nil
	}
	got, err := fetchBranchLatest(get, "8.4")
	if err != nil {
		t.Fatal(err)
	}
	if got != "8.4.26" {
		t.Errorf("fetchBranchLatest() = %q, want %q", got, "8.4.26")
	}
}

func TestFetchBranchLatestNoRelease(t *testing.T) {
	get := func(url string) (*http.Response, error) {
		return jsonResponse(200, `{}`, nil), nil
	}
	if _, err := fetchBranchLatest(get, "8.6"); err == nil {
		t.Fatal("fetchBranchLatest() = nil error, want error for a branch with no release")
	}
}

func TestRunSkipsBrokenBranchWithoutAbortingOthers(t *testing.T) {
	branchCalls := 0
	get := func(url string) (*http.Response, error) {
		switch {
		case strings.Contains(url, "php-src/branches"):
			return jsonResponse(200, `[{"name":"PHP-7.4"},{"name":"PHP-8.4"}]`, nil), nil
		case strings.Contains(url, "version=7.4"):
			branchCalls++
			return jsonResponse(500, `boom`, nil), nil
		case strings.Contains(url, "version=8.4"):
			branchCalls++
			return jsonResponse(200, `{"version":"8.4.26"}`, nil), nil
		default:
			t.Fatalf("unexpected url %s", url)
			return nil, nil
		}
	}

	// run() hardcodes httpGet; exercise discoverBranches+fetchBranchLatest
	// directly instead, the same way run() composes them, so the test
	// doesn't depend on real network access.
	branches, err := discoverBranches(get, "7.4")
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	var warn bytes.Buffer
	var missing []string
	have := map[string]bool{}
	for _, branch := range branches {
		latest, err := fetchBranchLatest(get, branch)
		if err != nil {
			fmt.Fprintf(&warn, "skip %s: %v\n", branch, err)
			continue
		}
		if !have[latest] {
			missing = append(missing, latest)
		}
	}
	if err := json.NewEncoder(&out).Encode(missing); err != nil {
		t.Fatal(err)
	}

	if branchCalls != 2 {
		t.Errorf("expected both branches to be queried, got %d calls", branchCalls)
	}
	if !strings.Contains(warn.String(), "skip 7.4") {
		t.Errorf("expected a warning about the broken 7.4 branch, got: %s", warn.String())
	}
	if !strings.Contains(out.String(), "8.4.26") {
		t.Errorf("expected 8.4.26 to still be found despite 7.4 failing, got: %s", out.String())
	}
}

func TestParseMajorMinor(t *testing.T) {
	cases := []struct {
		in         string
		major, min int
		wantErr    bool
	}{
		{"7.4", 7, 4, false},
		{"8.10", 8, 10, false},
		{"8", 0, 0, true},
		{"x.y", 0, 0, true},
	}
	for _, c := range cases {
		major, minor, err := parseMajorMinor(c.in)
		if (err != nil) != c.wantErr {
			t.Errorf("parseMajorMinor(%q) error = %v, wantErr %v", c.in, err, c.wantErr)
			continue
		}
		if err == nil && (major != c.major || minor != c.min) {
			t.Errorf("parseMajorMinor(%q) = (%d, %d), want (%d, %d)", c.in, major, minor, c.major, c.min)
		}
	}
}

func TestSortVersions(t *testing.T) {
	in := []string{"8.10", "7.4", "8.2", "8.9", "7.0"}
	sortVersions(in)
	want := []string{"7.0", "7.4", "8.2", "8.9", "8.10"}
	if !stringsEqual(in, want) {
		t.Errorf("sortVersions() = %v, want %v", in, want)
	}
}

func TestNextPageURL(t *testing.T) {
	link := `<https://api.github.com/x?page=2>; rel="next", <https://api.github.com/x?page=5>; rel="last"`
	if got := nextPageURL(link); got != "https://api.github.com/x?page=2" {
		t.Errorf("nextPageURL() = %q, want the rel=next URL", got)
	}
	if got := nextPageURL(`<https://api.github.com/x?page=5>; rel="last"`); got != "" {
		t.Errorf("nextPageURL() = %q, want empty when there is no rel=next", got)
	}
}

func stringsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
