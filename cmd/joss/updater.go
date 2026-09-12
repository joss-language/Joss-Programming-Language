package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/jossecurity/joss/pkg/i18n"
	"github.com/jossecurity/joss/pkg/version"
)

const (
	githubReleasesURL = "https://api.github.com/repos/josprox/Joss-language/releases"
	defaultChannel    = "stable"
)

type UpdateConfig struct {
	Channel          string `json:"channel"`
	LastCheck        int64  `json:"last_check"`
	LatestVersion    string `json:"latest_version"`
	LatestAssetURL   string `json:"latest_asset_url"`
	NotificationDone bool   `json:"notification_done"`
}

type GitHubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type GitHubRelease struct {
	TagName    string        `json:"tag_name"`
	Name       string        `json:"name"`
	Draft      bool          `json:"draft"`
	Prerelease bool          `json:"prerelease"`
	Assets     []GitHubAsset `json:"assets"`
}

func getUpdateConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	dir := filepath.Join(home, ".joss")
	os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "update_config.json")
}

func loadUpdateConfig() UpdateConfig {
	cfg := UpdateConfig{Channel: defaultChannel}
	path := getUpdateConfigPath()
	data, err := os.ReadFile(path)
	if err == nil {
		json.Unmarshal(data, &cfg)
	}
	if cfg.Channel == "" {
		cfg.Channel = defaultChannel
	}
	return cfg
}

func saveUpdateConfig(cfg UpdateConfig) {
	path := getUpdateConfigPath()
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err == nil {
		os.WriteFile(path, data, 0644)
	}
}

// checkUpdateBackground performs a fast, non-blocking check for new versions on startup.
func checkUpdateBackground() {
	cfg := loadUpdateConfig()

	// Check at most once per 30 minutes
	now := time.Now().Unix()
	if now-cfg.LastCheck < 1800 && cfg.LatestVersion != "" {
		if isVersionNewer(cfg.LatestVersion, version.Version) {
			printUpdateNotification(cfg.LatestVersion, cfg.Channel)
		}
		return
	}

	// Non-blocking quick check
	go func() {
		defer func() {
			if r := recover(); r != nil {
				// Silent recover
			}
		}()

		client := &http.Client{Timeout: 2 * time.Second}
		req, err := http.NewRequest("GET", githubReleasesURL, nil)
		if err != nil {
			return
		}
		req.Header.Set("User-Agent", "Joss-CLI-Updater/"+version.Version)

		resp, err := client.Do(req)
		if err != nil || resp.StatusCode != 200 {
			return
		}
		defer resp.Body.Close()

		var releases []GitHubRelease
		if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil || len(releases) == 0 {
			return
		}

		targetRelease := selectReleaseByChannel(releases, cfg.Channel)
		if targetRelease == nil {
			return
		}

		remoteVer := cleanVersionTag(targetRelease.TagName)
		assetURL := findMatchingAsset(targetRelease.Assets, runtime.GOOS, runtime.GOARCH)

		cfg.LastCheck = time.Now().Unix()
		cfg.LatestVersion = remoteVer
		cfg.LatestAssetURL = assetURL
		saveUpdateConfig(cfg)

		if isVersionNewer(remoteVer, version.Version) {
			printUpdateNotification(remoteVer, cfg.Channel)
		}
	}()
}

func printUpdateNotification(remoteVer, channel string) {
	channelTag := strings.ToUpper(channel)
	fmt.Printf("\n💡 \033[1;33m[JOSS UPDATE]\033[0m %s \033[1;36mv%s\033[0m -> \033[1;32mv%s\033[0m (%s)\n", i18n.Tr("updaterNewVersionTitle"), version.Version, remoteVer, channelTag)
	fmt.Printf("   %s\n\n", i18n.Tr("updaterRunHint", map[string]interface{}{"channel": strings.ToLower(channel)}))
}

// handleUpdateCommand executes the 'joss update' CLI command.
func handleUpdateCommand(args []string) {
	cfg := loadUpdateConfig()
	force := false
	// Parse flags for channel override (--canary / --stable) and force flag (-f / --force)
	for _, arg := range args {
		argLower := strings.ToLower(arg)
		if argLower == "-f" || argLower == "--force" || argLower == "-force" {
			force = true
		} else if strings.Contains(argLower, "canary") {
			cfg.Channel = "canary"
		} else if strings.Contains(argLower, "stable") {
			cfg.Channel = "stable"
		}
	}

	saveUpdateConfig(cfg)

	fmt.Printf("\n=======================================================\n")
	fmt.Printf("%s (Joss Auto-Updater)\n", i18n.Tr("updaterTitle"))
	fmt.Printf(" %s : v%s\n", i18n.Tr("updaterCurrentVersion"), version.Version)
	fmt.Printf(" %s  : %s\n", i18n.Tr("updaterSelectedChannel"), strings.ToUpper(cfg.Channel))
	if force {
		fmt.Printf(" %s   : ACTIVO (-f)\n", i18n.Tr("updaterForcedMode"))
	}
	fmt.Printf("=======================================================\n\n")

	fmt.Println(i18n.Tr("updaterCheckingGitHub"))

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", githubReleasesURL, nil)
	if err != nil {
		fmt.Println(i18n.Tr("updaterPrepareRequestError", i18n.M{"error": err.Error()}))
		return
	}
	req.Header.Set("User-Agent", "Joss-CLI-Updater/"+version.Version)

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(i18n.Tr("updaterGitHubConnectError", i18n.M{"error": err.Error()}))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		fmt.Println(i18n.Tr("updaterGitHubStatusError", i18n.M{"status": resp.StatusCode}))
		return
	}

	var releases []GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil || len(releases) == 0 {
		fmt.Println(i18n.Tr("updaterNoReleasesFound"))
		return
	}

	targetRelease := selectReleaseByChannel(releases, cfg.Channel)
	if targetRelease == nil {
		fmt.Println(i18n.Tr("updaterNoReleaseForChannel", map[string]interface{}{"channel": cfg.Channel}))
		return
	}

	remoteVer := cleanVersionTag(targetRelease.TagName)
	fmt.Printf("📦 %s\n", i18n.Tr("updaterSelectedRelease", i18n.M{"name": targetRelease.Name, "tag": targetRelease.TagName}))

	if !force && compareVersions(remoteVer, version.Version) == 0 {
		fmt.Println(i18n.Tr("updaterAlreadyUpdated", map[string]interface{}{"version": version.Version, "channel": strings.ToUpper(cfg.Channel)}))
		fmt.Println(i18n.Tr("updaterForceTip"))
		fmt.Println()
		return
	}

	if force {
		fmt.Println(i18n.Tr("updaterForcedRedownload"))
	}

	assetURL := findMatchingAsset(targetRelease.Assets, runtime.GOOS, runtime.GOARCH)
	if assetURL == "" {
		fmt.Printf("⚠️ %s\n", i18n.Tr("updaterNoSpecificBinary", i18n.M{"os": runtime.GOOS, "arch": runtime.GOARCH, "tag": targetRelease.TagName}))
		fmt.Println(i18n.Tr("updaterDownloadingGeneral"))
		if len(targetRelease.Assets) > 0 {
			assetURL = targetRelease.Assets[0].BrowserDownloadURL
		}
	}

	if assetURL == "" {
		fmt.Println(i18n.Tr("updaterNoBinariesFound"))
		return
	}

	fmt.Println(i18n.Tr("updaterDownloading"))

	tempDir, err := os.MkdirTemp("", "joss-update-*")
	if err != nil {
		fmt.Println(i18n.Tr("updaterTempDirError", i18n.M{"error": err.Error()}))
		return
	}
	defer os.RemoveAll(tempDir)

	downloadPath := filepath.Join(tempDir, "update_download.bin")
	if err := downloadFile(downloadPath, assetURL); err != nil {
		fmt.Println(i18n.Tr("updaterDownloadError", i18n.M{"error": err.Error()}))
		return
	}

	binaryToApply := downloadPath

	// Extract binary if download is a ZIP archive
	if data, err := os.ReadFile(downloadPath); err == nil && len(data) >= 4 && data[0] == 'P' && data[1] == 'K' {
		zipReader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err == nil {
			extractedBin := filepath.Join(tempDir, "extracted_joss_binary")
			found := false
			expectedName := fmt.Sprintf("joss-%s-%s", runtime.GOOS, runtime.GOARCH)
			if runtime.GOOS == "windows" {
				expectedName += ".exe"
			}
			for _, file := range zipReader.File {
				name := strings.ToLower(filepath.Base(file.Name))
				if name == "joss.exe" || name == "joss" || name == expectedName {
					rc, err := file.Open()
					if err == nil {
						content, _ := io.ReadAll(rc)
						rc.Close()
						if len(content) > 0 {
							os.WriteFile(extractedBin, content, 0755)
							binaryToApply = extractedBin
							found = true
							break
						}
					}
				}
			}
			if !found {
				for _, file := range zipReader.File {
					if !file.FileInfo().IsDir() && strings.HasPrefix(strings.ToLower(filepath.Base(file.Name)), "joss") {
						rc, err := file.Open()
						if err == nil {
							content, _ := io.ReadAll(rc)
							rc.Close()
							if len(content) > 0 {
								os.WriteFile(extractedBin, content, 0755)
								binaryToApply = extractedBin
								break
							}
						}
					}
				}
			}
		}
	}

	fmt.Println(i18n.Tr("updaterApplying"))

	currentExe, err := os.Executable()
	if err != nil {
		fmt.Println(i18n.Tr("updaterCurrentExeError", i18n.M{"error": err.Error()}))
		return
	}

	// Apply self-update binary replacement safely
	if err := replaceExecutable(currentExe, binaryToApply); err != nil {
		fmt.Println(i18n.Tr("updaterApplyError", i18n.M{"error": err.Error()}))
		return
	}

	cfg.LastCheck = time.Now().Unix()
	cfg.LatestVersion = remoteVer
	cfg.LatestAssetURL = assetURL
	saveUpdateConfig(cfg)

	fmt.Printf("\n%s\n", i18n.Tr("updaterSuccessTitle"))
	fmt.Printf(" %s : v%s (%s)\n", i18n.Tr("updaterNewVersion"), remoteVer, strings.ToUpper(cfg.Channel))
	fmt.Printf(" %s     : %s\n\n", i18n.Tr("updaterExecutable"), currentExe)
}

func selectReleaseByChannel(releases []GitHubRelease, channel string) *GitHubRelease {
	if strings.ToLower(channel) == "canary" {
		// Prefer prerelease if available, otherwise latest release
		for i := range releases {
			if releases[i].Prerelease && !releases[i].Draft {
				return &releases[i]
			}
		}
		// Fallback to latest
		if len(releases) > 0 {
			return &releases[0]
		}
		return nil
	}

	// Stable channel: non-draft and non-prerelease
	for i := range releases {
		if !releases[i].Draft && !releases[i].Prerelease {
			return &releases[i]
		}
	}
	// Fallback if none found
	if len(releases) > 0 {
		return &releases[0]
	}
	return nil
}

func findMatchingAsset(assets []GitHubAsset, targetOS, targetArch string) string {
	targetOS = strings.ToLower(targetOS)
	targetArch = strings.ToLower(targetArch)

	for _, a := range assets {
		name := strings.ToLower(a.Name)
		if strings.Contains(name, targetOS) && strings.Contains(name, targetArch) {
			return a.BrowserDownloadURL
		}
	}
	for _, a := range assets {
		name := strings.ToLower(a.Name)
		if strings.Contains(name, targetOS) {
			return a.BrowserDownloadURL
		}
	}
	return ""
}

func downloadFile(filepath string, url string) error {
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("servidor devolvió status %s", resp.Status)
	}

	_, err = io.Copy(out, resp.Body)
	return err
}

func replaceExecutable(currentExe, newExe string) error {
	// Resolve symlinks to target the real executable file
	if resolved, err := filepath.EvalSymlinks(currentExe); err == nil && resolved != "" {
		currentExe = resolved
	}

	input, err := os.ReadFile(newExe)
	if err != nil {
		return fmt.Errorf("no se pudo leer el archivo binario descargado: %w", err)
	}

	if runtime.GOOS == "windows" {
		oldExe := currentExe + ".old"
		_ = os.Remove(oldExe)

		// 1. Rename running executable to .old (Windows allows renaming running binaries, but not overwriting them directly)
		if err := os.Rename(currentExe, oldExe); err != nil {
			if os.IsPermission(err) || strings.Contains(strings.ToLower(err.Error()), "access is denied") {
				return fmt.Errorf("permiso denegado al modificar '%s'.\n👉 Ejecuta tu terminal como Administrador (Run as Administrator) para actualizar Joss en este directorio", currentExe)
			}
			return fmt.Errorf("no se pudo renombrar el ejecutable actual (%s): %w", currentExe, err)
		}

		// 2. Write new binary in the original location
		if err := os.WriteFile(currentExe, input, 0755); err != nil {
			// Rollback if writing fails
			_ = os.Rename(oldExe, currentExe)
			if os.IsPermission(err) || strings.Contains(strings.ToLower(err.Error()), "access is denied") {
				return fmt.Errorf("permiso denegado al escribir en '%s'.\n👉 Ejecuta tu terminal como Administrador (Run as Administrator) para completar la actualización", currentExe)
			}
			return fmt.Errorf("no se pudo escribir el nuevo ejecutable (%s): %w", currentExe, err)
		}

		// Try removing .old (if locked, it's ok, it will be overwritten on next update)
		_ = os.Remove(oldExe)
		return nil
	}

	// Unix-like OS (Linux / macOS / BSD):
	// Direct os.WriteFile() on a running executable yields ETXTBSY ("text file busy").
	// Standard Unix safe update pattern:
	// 1. Write the new binary to a temporary file in the same directory.
	// 2. Set executable permissions (0755).
	// 3. Atomically rename the temp file over the running target path (POSIX rename unlinks the old inode atomically).
	targetDir := filepath.Dir(currentExe)
	tmpFile := filepath.Join(targetDir, fmt.Sprintf(".joss-update-%d.tmp", time.Now().UnixNano()))

	if err := os.WriteFile(tmpFile, input, 0755); err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("permiso denegado al escribir en '%s'.\n👉 Ejecuta 'sudo joss update' para actualizar Joss en este directorio del sistema", targetDir)
		}
		return fmt.Errorf("no se pudo crear archivo temporal de actualización en '%s': %w", targetDir, err)
	}
	defer os.Remove(tmpFile)

	if err := os.Chmod(tmpFile, 0755); err != nil {
		// Non-fatal if filesystem doesn't support chmod
	}

	if err := os.Rename(tmpFile, currentExe); err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("permiso denegado al reemplazar '%s'.\n👉 Ejecuta 'sudo joss update' para completar la actualización", currentExe)
		}
		// Fallback: attempt unlinking target first
		_ = os.Remove(currentExe)
		if retryErr := os.Rename(tmpFile, currentExe); retryErr != nil {
			return fmt.Errorf("no se pudo reemplazar el binario ejecutable (%s): %w", currentExe, retryErr)
		}
	}

	return nil
}

func cleanVersionTag(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	return v
}

func parseVersionParts(v string) []int {
	v = cleanVersionTag(v)
	if idx := strings.IndexAny(v, "-+"); idx != -1 {
		v = v[:idx]
	}
	parts := strings.Split(v, ".")
	nums := make([]int, 0, len(parts))
	for _, p := range parts {
		n := 0
		fmt.Sscanf(p, "%d", &n)
		nums = append(nums, n)
	}
	return nums
}

func compareVersions(v1, v2 string) int {
	nums1 := parseVersionParts(v1)
	nums2 := parseVersionParts(v2)

	maxLen := len(nums1)
	if len(nums2) > maxLen {
		maxLen = len(nums2)
	}

	for i := 0; i < maxLen; i++ {
		n1 := 0
		if i < len(nums1) {
			n1 = nums1[i]
		}
		n2 := 0
		if i < len(nums2) {
			n2 = nums2[i]
		}
		if n1 > n2 {
			return 1
		}
		if n1 < n2 {
			return -1
		}
	}
	return 0
}

func isVersionNewer(remote, current string) bool {
	return compareVersions(remote, current) > 0
}
