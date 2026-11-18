package main

import (
	"testing"

	"github.com/jossecurity/joss/pkg/backend/native"
)

func TestParseBuildTarget(t *testing.T) {
	tests := []struct {
		target   string
		expected bool
	}{
		{"windows-amd64", true},
		{"windows-arm64", true},
		{"linux-amd64", true},
		{"linux-arm64", true},
		{"darwin-arm64", true},
		{"darwin-amd64", true},
		{"android-arm64", true},
		{"android-arm", false},
		{"windows", true},
		{"linux", true},
		{"macos", true},
		{"android", true},
		{"android-mips", false},
		{"unknownos-amd64", false},
		{"invalid-target-format-extra", false},
	}

	for _, tc := range tests {
		_, err := native.ParseTarget(tc.target)
		valid := err == nil
		if valid != tc.expected {
			t.Errorf("native.ParseTarget(%q) valid = %v, want %v (err: %v)", tc.target, valid, tc.expected, err)
		}
	}
}

func TestComputePruningTagsSelectOnlyUnusedCapabilities(t *testing.T) {
	tags := computePruningTags(map[string]bool{
		"database":     true,
		"mysql":        true,
		"sqlite":       false,
		"postgres":     false,
		"mssql":        false,
		"redis":        false,
		"gui":          false,
		"embedlocales": false,
	})
	for _, expected := range []string{"nosqlite", "nopostgres", "nomssql", "noredis", "nogui", "noembedlocales"} {
		if !containsString(tags, expected) {
			t.Errorf("pruning tags %v do not include %q", tags, expected)
		}
	}
	if containsString(tags, "nomysql") {
		t.Fatalf("mysql profile must retain the mysql driver: %v", tags)
	}
	if containsString(tags, "nodb") {
		t.Fatalf("mysql profile must not disable all database drivers: %v", tags)
	}
	if got, want := runnerProfileName(tags), "noembedlocales_nogui_nomssql_nopostgres_noredis_nosqlite"; got != want {
		t.Fatalf("mysql runner profile = %q, want release profile %q", got, want)
	}
}

func TestComputePruningTagsDefinesPureRunnerProfile(t *testing.T) {
	tags := computePruningTags(map[string]bool{})
	if got, want := runnerProfileName(tags), "nodb_noembedlocales_nogui_noredis"; got != want {
		t.Fatalf("pure runner profile = %q, want release profile %q", got, want)
	}
}

func TestDatabaseDriverDetectionHonorsConnectionAliasBeforeDefault(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  map[string]string
		want string
	}{
		{name: "DB_CONNECTION fallback", env: map[string]string{"DB_CONNECTION": "postgresql"}, want: "postgresql"},
		{name: "DB takes precedence", env: map[string]string{"DB": "mysql", "DB_CONNECTION": "sqlite"}, want: "mysql"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := configuredDatabaseDriver(tc.env); got != tc.want {
				t.Fatalf("configured database driver = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRunnerProfileNameIsStable(t *testing.T) {
	first := runnerProfileName([]string{"noredis", "nodb", "nogui"})
	second := runnerProfileName([]string{"nogui", "noredis", "nodb"})
	if first != second {
		t.Fatalf("runner profile names differ for same tags: %q != %q", first, second)
	}
	if got := runnerProfileName(nil); got != "full" {
		t.Fatalf("empty profile = %q, want full", got)
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
