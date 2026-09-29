package main

import "testing"

func TestVersionLess(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"8.4.9", "8.4.26", true},
		{"8.4.26", "8.4.9", false},
		{"8.3.35", "8.4.1", true},
		{"8.4.26", "8.4.26", false},
	}
	for _, c := range cases {
		if got := versionLess(c.a, c.b); got != c.want {
			t.Errorf("versionLess(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestLoadSigningKeyMissing(t *testing.T) {
	t.Setenv("MANIFEST_SIGNING_KEY", "")
	if _, err := loadSigningKey(); err == nil {
		t.Fatal("loadSigningKey() = nil error, want error when unset")
	}
}

func TestLoadSigningKeyWrongLength(t *testing.T) {
	t.Setenv("MANIFEST_SIGNING_KEY", "AAAA")
	if _, err := loadSigningKey(); err == nil {
		t.Fatal("loadSigningKey() = nil error, want error for undersized key")
	}
}
