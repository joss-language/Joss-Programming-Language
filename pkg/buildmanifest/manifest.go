package buildmanifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/jossecurity/joss/pkg/analyzer"
	"github.com/jossecurity/joss/pkg/version"
)

// TargetInfo describes the compilation platform and profile.
type TargetInfo struct {
	OS      string `json:"os"`
	Arch    string `json:"arch"`
	Profile string `json:"profile"` // "cli", "gui", "server"
}

// ReachabilitySummary details which files and symbols are kept or pruned.
type ReachabilitySummary struct {
	ScannedFiles   int      `json:"scanned_files"`
	IncludedFiles  int      `json:"included_files"`
	PrunedFiles    int      `json:"pruned_files"`
	LiveClasses    []string `json:"live_classes"`
	PrunedClasses  []string `json:"pruned_classes,omitempty"`
	RetainedFiles  []string `json:"retained_files"`
	DiscardedFiles []string `json:"discarded_files,omitempty"`
}

// VFSMetrics tracks code and asset payload footprints.
type VFSMetrics struct {
	SourceFilesCount   int    `json:"source_files_count"`
	BytecodeFilesCount int    `json:"bytecode_files_count"`
	TotalPayloadBytes  int64  `json:"total_payload_bytes"`
	CompressionRatio   string `json:"compression_ratio,omitempty"`
}

// BuildManifest stores the complete internal metadata for a joss build execution.
type BuildManifest struct {
	CompilerVersion     string              `json:"compiler_version"`
	LanguageVersion     string              `json:"language_version"`
	BuildTimestamp      string              `json:"build_timestamp"`
	Mode                string              `json:"mode"` // "release", "debug"
	Entrypoint          string              `json:"entrypoint"`
	Target              TargetInfo          `json:"target"`
	Reachability        ReachabilitySummary `json:"reachability"`
	RuntimeCapabilities map[string]bool     `json:"runtime_capabilities"`
	VFS                 VFSMetrics          `json:"vfs"`
	OutputBinary        string              `json:"output_binary"`
	BuildHash           string              `json:"build_hash"`
}

// GenerateManifest constructs a canonical build manifest from graph results and compilation settings.
func GenerateManifest(
	entrypoint, mode, targetOS, targetArch, profile, outBinary string,
	allFiles []string,
	graph *analyzer.ReachabilityGraph,
	payloadBytes int64,
) *BuildManifest {
	caps := make(map[string]bool)
	if graph != nil {
		for cap, active := range graph.RuntimeCapabilities {
			caps[string(cap)] = active
		}
	}

	retained := make([]string, 0)
	discarded := make([]string, 0)
	liveClasses := make([]string, 0)

	if graph != nil {
		for _, f := range allFiles {
			if graph.IsFileReachable(f) {
				retained = append(retained, f)
			} else {
				discarded = append(discarded, f)
			}
		}
		for c := range graph.LiveClasses {
			liveClasses = append(liveClasses, c)
		}
	} else {
		retained = append(retained, allFiles...)
	}

	sort.Strings(retained)
	sort.Strings(discarded)
	sort.Strings(liveClasses)

	hasher := sha256.New()
	hasher.Write([]byte(entrypoint + mode + targetOS + targetArch + profile))
	for _, f := range retained {
		hasher.Write([]byte(f))
	}
	buildHash := hex.EncodeToString(hasher.Sum(nil))

	return &BuildManifest{
		CompilerVersion: version.Version,
		LanguageVersion: version.Version,
		BuildTimestamp:  time.Now().UTC().Format(time.RFC3339),
		Mode:            mode,
		Entrypoint:      entrypoint,
		Target: TargetInfo{
			OS:      targetOS,
			Arch:    targetArch,
			Profile: profile,
		},
		Reachability: ReachabilitySummary{
			ScannedFiles:   len(allFiles),
			IncludedFiles:  len(retained),
			PrunedFiles:    len(discarded),
			LiveClasses:    liveClasses,
			RetainedFiles:  retained,
			DiscardedFiles: discarded,
		},
		RuntimeCapabilities: caps,
		VFS: VFSMetrics{
			SourceFilesCount:   len(retained),
			BytecodeFilesCount: len(retained),
			TotalPayloadBytes:  payloadBytes,
		},
		OutputBinary: outBinary,
		BuildHash:    buildHash,
	}
}

// SaveToFile persists the manifest to a destination path (e.g. .joss/cache/build-manifest.json).
func (m *BuildManifest) SaveToFile(destPath string) error {
	dir := filepath.Dir(destPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(destPath, data, 0644)
}
