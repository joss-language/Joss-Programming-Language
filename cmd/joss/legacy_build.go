package main

import (
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jossecurity/joss/pkg/backend/native"
	"github.com/jossecurity/joss/pkg/crypto"
)

const legacyRunnerTrailer = "JOSS_RUNNER_DATA"

func buildLegacyProgram(filename, outExe, targetStr string, enableGUI bool) (string, error) {
	if _, err := os.Stat(filename); err != nil {
		return "", fmt.Errorf("archivo '%s' no encontrado: %w", filename, err)
	}
	target, err := native.ParseTarget(targetStr)
	if err != nil {
		return "", err
	}
	if outExe == "" {
		base := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
		if base == "" {
			base = "app"
		}
		outExe = filepath.Join("build", base)
		if target.OS == "windows" {
			outExe += ".exe"
		}
	} else if target.OS == "windows" && !strings.HasSuffix(strings.ToLower(outExe), ".exe") {
		outExe += ".exe"
	}

	files, detectedCaps, err := collectProjectFiles(enableGUI, filename)
	if err != nil {
		return "", fmt.Errorf("empaquetar proyecto: %w", err)
	}
	detectedCaps["gui"] = enableGUI
	pruningTags := computePruningTags(detectedCaps)
	runner, cleanup, err := resolveLegacyRunner(target, pruningTags, enableGUI)
	if err != nil {
		return "", err
	}
	defer cleanup()
	if err := os.MkdirAll(filepath.Dir(outExe), 0755); err != nil && filepath.Dir(outExe) != "." {
		return "", err
	}
	if err := copyFile(runner, outExe); err != nil {
		return "", fmt.Errorf("copiar runner anterior: %w", err)
	}
	if target.OS != "windows" {
		_ = os.Chmod(outExe, 0755)
	}

	if filepath.ToSlash(filepath.Clean(filename)) != "main.joss" {
		source, readErr := os.ReadFile(filename)
		if readErr != nil {
			_ = os.Remove(outExe)
			return "", fmt.Errorf("leer entrypoint %q: %w", filename, readErr)
		}
		if encoded, ok := tryCompileJossBytecode(source); ok {
			source = encoded
		}
		files["main.joss"] = source
	}
	assets, key, err := sealLegacyProject(files)
	if err != nil {
		_ = os.Remove(outExe)
		return "", fmt.Errorf("sellar proyecto: %w", err)
	}
	file, err := os.OpenFile(outExe, os.O_WRONLY|os.O_APPEND, 0755)
	if err != nil {
		_ = os.Remove(outExe)
		return "", err
	}
	writeErr := func() error {
		if _, err := file.Write(assets); err != nil {
			return err
		}
		if _, err := file.Write(key); err != nil {
			return err
		}
		var length [8]byte
		binary.LittleEndian.PutUint64(length[:], uint64(len(assets)))
		if _, err := file.Write(length[:]); err != nil {
			return err
		}
		_, err := io.WriteString(file, legacyRunnerTrailer)
		return err
	}()
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(outExe)
		if writeErr != nil {
			return "", fmt.Errorf("sellar ejecutable anterior: %w", writeErr)
		}
		return "", closeErr
	}
	info, err := os.Stat(outExe)
	if err != nil {
		return "", err
	}
	fmt.Printf("✓ Ejecutable autónomo generado: %s (%.1f MB) [compatibilidad Go, perfil: %s, target: %s]\n", outExe, float64(info.Size())/(1024*1024), runnerProfileName(pruningTags), target.DashString())
	return outExe, nil
}

func sealLegacyProject(files map[string][]byte) ([]byte, []byte, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, nil, err
	}
	var compressed bytes.Buffer
	writer, err := gzip.NewWriterLevel(&compressed, gzip.BestCompression)
	if err != nil {
		return nil, nil, err
	}
	if err := gob.NewEncoder(writer).Encode(files); err != nil {
		_ = writer.Close()
		return nil, nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, nil, err
	}
	sealed, err := crypto.EncryptAES(compressed.Bytes(), key)
	if err != nil {
		return nil, nil, err
	}
	return sealed, key, nil
}

func resolveLegacyRunner(target native.Target, pruningTags []string, enableGUI bool) (string, func(), error) {
	name := "joss-runner"
	if target.OS == "windows" {
		name += ".exe"
	}
	profile := runnerProfileName(pruningTags)
	var profileCandidates []string
	var genericCandidates []string
	if dir := os.Getenv("JOSS_RUNNER_DIR"); dir != "" {
		profileCandidates = append(profileCandidates, filepath.Join(dir, target.DashString(), "profiles", profile, name))
		genericCandidates = append(genericCandidates, filepath.Join(dir, target.DashString(), name))
	}
	if executable, err := os.Executable(); err == nil {
		profileCandidates = append(profileCandidates, filepath.Join(filepath.Dir(executable), "runners", target.DashString(), "profiles", profile, name))
		genericCandidates = append(genericCandidates, filepath.Join(filepath.Dir(executable), "runners", target.DashString(), name))
	}
	if root := findJossRepoRoot(); root != "" {
		profileCandidates = append(profileCandidates, filepath.Join(root, "runners", target.DashString(), "profiles", profile, name))
		genericCandidates = append(genericCandidates, filepath.Join(root, "runners", target.DashString(), name))
	}
	for _, candidate := range profileCandidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() && info.Size() > 0 {
			return candidate, func() {}, nil
		}
	}

	root := findJossRepoRoot()
	if root == "" {
		for _, candidate := range genericCandidates {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() && info.Size() > 0 {
				return candidate, func() {}, nil
			}
		}
		return "", func() {}, fmt.Errorf("runner anterior para %s no está instalado; se esperaba %s junto a Joss", target.DashString(), filepath.Join("runners", target.DashString(), name))
	}
	goPath, err := exec.LookPath("go")
	if err != nil {
		for _, candidate := range genericCandidates {
			if info, statErr := os.Stat(candidate); statErr == nil && !info.IsDir() && info.Size() > 0 {
				return candidate, func() {}, nil
			}
		}
		return "", func() {}, fmt.Errorf("runner anterior para %s (perfil %s) no está instalado y no hay toolchain Go en el checkout para generarlo", target.DashString(), profile)
	}
	if len(pruningTags) == 0 {
		for _, candidate := range genericCandidates {
			if info, statErr := os.Stat(candidate); statErr == nil && !info.IsDir() && info.Size() > 0 {
				return candidate, func() {}, nil
			}
		}
	}
	temp, err := os.CreateTemp("", "joss-legacy-runner-*")
	if err != nil {
		return "", func() {}, err
	}
	_ = temp.Close()
	_ = os.Remove(temp.Name())
	ldflags := "-s -w"
	if target.OS == "windows" && enableGUI {
		ldflags = "-s -w -H=windowsgui"
	}
	args := []string{"build", "-trimpath", "-buildvcs=false", "-ldflags=" + ldflags, "-o", temp.Name()}
	if len(pruningTags) > 0 {
		args = append(args, "-tags="+strings.Join(pruningTags, ","))
	}
	args = append(args, "./cmd/runner")
	cmd := exec.Command(goPath, args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS="+target.OS, "GOARCH="+target.Arch)
	if output, err := cmd.CombinedOutput(); err != nil {
		_ = os.Remove(temp.Name())
		return "", func() {}, fmt.Errorf("compilar runner anterior para %s: %w\n%s", target.DashString(), err, strings.TrimSpace(string(output)))
	}
	cleanup := func() { _ = os.Remove(temp.Name()) }
	return temp.Name(), cleanup, nil
}

func runnerProfileName(tags []string) string {
	if len(tags) == 0 {
		return "full"
	}
	ordered := append([]string(nil), tags...)
	sort.Strings(ordered)
	return strings.Join(ordered, "_")
}

func findJossRepoRoot() string {
	starts := []string{"."}
	if exe, err := os.Executable(); err == nil {
		starts = append(starts, filepath.Dir(exe))
	}
	for _, start := range starts {
		current, err := filepath.Abs(start)
		if err != nil {
			continue
		}
		for {
			if _, err := os.Stat(filepath.Join(current, "go.mod")); err == nil {
				if _, err := os.Stat(filepath.Join(current, "cmd", "runner", "main.go")); err == nil {
					return current
				}
			}
			parent := filepath.Dir(current)
			if parent == current {
				break
			}
			current = parent
		}
	}
	return ""
}
