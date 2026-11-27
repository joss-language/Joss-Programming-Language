package llvm

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/jossecurity/joss/pkg/version"
)

const defaultRustRTBaseURL = "https://github.com/joss-language/Joss-Programming-Language/releases/download/rust-rt-latest"

func rustTripleFor(targetOS, targetArch string) string {
	switch targetOS + "/" + targetArch {
	case "windows/amd64":
		return "x86_64-pc-windows-gnu"
	case "linux/amd64":
		return "x86_64-unknown-linux-gnu"
	case "linux/arm64":
		return "aarch64-unknown-linux-gnu"
	case "darwin/amd64":
		return "x86_64-apple-darwin"
	case "darwin/arm64":
		return "aarch64-apple-darwin"
	default:
		return ""
	}
}

func rustRTCacheDir() string {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return filepath.Join(os.TempDir(), "joss-rust-rt", version.Version)
	}
	return filepath.Join(home, ".joss", "cache", "rust-rt", version.Version)
}

// EnsureRustRuntime resolves the precompiled Rust micro-kernel staticlib
// (`libjoss_rt_rs.a`) for the requested target without requiring rustc/cargo
// on the user's machine or Coolify container.
//
// Resolution order:
//  1. Explicit local dev checkout (`runtime/rust/target/<triple>/release/libjoss_rt_rs.a`)
//  2. Bundled next to runtime or `joss` executable (`<exe_dir>/runtime/rust/<os>-<arch>/libjoss_rt_rs.a`)
//  3. Local cache (`~/.joss/cache/rust-rt/<version>/libjoss_rt_rs_<os>_<arch>.a`)
//  4. Automatic download from GitHub Releases (unless JOSS_NO_AUTO_TOOLCHAIN=1)
func EnsureRustRuntime(rtDir, targetOS, targetArch string) string {
	if os.Getenv("JOSS_DISABLE_RUST_RT") == "1" {
		return ""
	}
	if targetOS == "" {
		targetOS = runtime.GOOS
	}
	if targetArch == "" {
		targetArch = runtime.GOARCH
	}
	triple := rustTripleFor(targetOS, targetArch)
	if triple == "" {
		return ""
	}
	assetName := fmt.Sprintf("libjoss_rt_rs_%s_%s.a", targetOS, targetArch)

	candidates := []string{
		filepath.Join(filepath.Dir(rtDir), "rust", "target", triple, "release", "libjoss_rt_rs.a"),
		filepath.Join(rtDir, assetName),
	}
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		candidates = append(candidates,
			filepath.Join(exeDir, "runtime", "rust", targetOS+"-"+targetArch, "libjoss_rt_rs.a"),
			filepath.Join(exeDir, "runtime", "rust", assetName),
		)
	}
	cachePath := filepath.Join(rustRTCacheDir(), assetName)
	candidates = append(candidates, cachePath)

	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() && info.Size() > 0 {
			return c
		}
	}

	if os.Getenv("JOSS_NO_AUTO_TOOLCHAIN") == "1" {
		return ""
	}

	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("JOSS_RUST_RT_BASE_URL")), "/")
	if baseURL == "" {
		baseURL = defaultRustRTBaseURL
	}
	if err := downloadRustStaticLib(baseURL+"/"+assetName, cachePath); err == nil {
		if info, statErr := os.Stat(cachePath); statErr == nil && info.Size() > 0 {
			return cachePath
		}
	}
	return ""
}

func downloadRustStaticLib(url, dstPath string) error {
	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return err
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d downloading %s", resp.StatusCode, url)
	}
	tmpPath := fmt.Sprintf("%s.tmp-%d", dstPath, os.Getpid())
	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	n, copyErr := io.Copy(f, resp.Body)
	closeErr := f.Close()
	if copyErr != nil || closeErr != nil || n == 0 {
		_ = os.Remove(tmpPath)
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		return fmt.Errorf("empty download from %s", url)
	}
	if err := os.Rename(tmpPath, dstPath); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}
