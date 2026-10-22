package main

import "testing"

func TestNormalizeVersion(t *testing.T) {
	cases := map[string]string{
		"v0.0.1":  "0.0.1",
		"0.0.1":   "0.0.1",
		"(devel)": "dev",
		"":        "dev",
	}
	for in, want := range cases {
		if got := normalizeVersion(in); got != want {
			t.Errorf("normalizeVersion(%q) = %q, want %q", in, got, want)
		}
	}
}
