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
		{"android-arm", true},
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
