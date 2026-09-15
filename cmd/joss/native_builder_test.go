package main

import "testing"

func TestValidateBuildTarget(t *testing.T) {
	tests := []struct {
		os       string
		arch     string
		expected bool
	}{
		{"windows", "amd64", true},
		{"windows", "arm64", true},
		{"linux", "amd64", true},
		{"linux", "arm64", true},
		{"darwin", "arm64", true},
		{"darwin", "amd64", true},
		{"android", "arm64", true},
		{"android", "arm", true},
		{"android", "386", true},
		{"android", "amd64", true},
		{"android", "mips", false},
		{"unknownos", "amd64", false},
	}

	for _, tc := range tests {
		_, _, valid := validateBuildTarget(tc.os, tc.arch)
		if valid != tc.expected {
			t.Errorf("validateBuildTarget(%q, %q) = %v, want %v", tc.os, tc.arch, valid, tc.expected)
		}
	}
}
