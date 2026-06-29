package main

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestHoistGlobalConfig(t *testing.T) {
	cases := []struct {
		name     string
		in       []string
		wantSub  string
		wantArgs []string
		wantOK   bool
	}{
		{"leading --config space", []string{"--config", "c.json", "triage", "f.txt"}, "triage", []string{"--config", "c.json", "f.txt"}, true},
		{"leading --config equals", []string{"--config=c.json", "classify", "x"}, "classify", []string{"--config", "c.json", "x"}, true},
		{"leading -config single dash", []string{"-config", "c.json", "models"}, "models", []string{"--config", "c.json"}, true},
		{"trailing --config untouched", []string{"triage", "f.txt", "--config", "c.json"}, "triage", []string{"f.txt", "--config", "c.json"}, true},
		{"no global config", []string{"summarize", "f.txt", "--json"}, "summarize", []string{"f.txt", "--json"}, true},
		{"config but no subcommand", []string{"--config", "c.json"}, "", nil, false},
		{"empty", []string{}, "", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sub, args, ok := hoistGlobalConfig(tc.in)
			if ok != tc.wantOK || sub != tc.wantSub || !reflect.DeepEqual(args, tc.wantArgs) {
				t.Fatalf("hoistGlobalConfig(%v) = (%q, %v, %v); want (%q, %v, %v)",
					tc.in, sub, args, ok, tc.wantSub, tc.wantArgs, tc.wantOK)
			}
		})
	}
}

// TestResolveCfgPath pins the config-path precedence: explicit --config flag >
// $LOCAL_OFFLOAD_CONFIG env > the conventional ~/.local-offload/config.json when
// it exists > "" (built-in defaults). The last rule is the fix for a bare
// `local-offload mcp` (no flag, no env) silently running on defaults — which left
// shadow capture off and the flywheel starved.
func TestResolveCfgPath(t *testing.T) {
	def := filepath.Join("/home/u", ".local-offload", "config.json")
	always := func(string) bool { return true }
	never := func(string) bool { return false }
	onlyDefault := func(p string) bool { return p == def }
	cases := []struct {
		name, flagPath, envPath, home string
		exists                        func(string) bool
		want                          string
	}{
		{"flag wins over env and default", "c.json", "e.json", "/home/u", always, "c.json"},
		{"env when no flag", "", "e.json", "/home/u", always, "e.json"},
		{"default when flag+env empty and it exists", "", "", "/home/u", onlyDefault, def},
		{"empty when default missing", "", "", "/home/u", never, ""},
		{"empty when home unknown", "", "", "", always, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveCfgPath(tc.flagPath, tc.envPath, tc.home, tc.exists)
			if got != tc.want {
				t.Fatalf("resolveCfgPath(%q,%q,%q) = %q; want %q",
					tc.flagPath, tc.envPath, tc.home, got, tc.want)
			}
		})
	}
}
