package llvm

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/jossecurity/joss/pkg/version"
	runtimeassets "github.com/jossecurity/joss/runtime"
	"github.com/ulikunitz/xz"
)

const managedZigVersion = "0.14.0"

// ToolchainInfo describe el compilador C/LLVM resuelto para compilación nativa standalone.
type ToolchainInfo struct {
	Kind         string // "bundled-zig", "bundled-clang", "managed-zig", "system-clang", "system-zig"
	CompilerPath string
	UseZigCC     bool
	LLDPath      string
	RootDir      string
}

// Available indica si se encontró un compilador listo para usar.
func (t ToolchainInfo) Available() bool {
	return t.CompilerPath != ""
}

// ResolveToolchain busca un toolchain nativo sin requerir instalación manual del desarrollador:
// 1. JOSS_TOOLCHAIN_DIR
// 2. <exe_dir>/toolchain/ (incluido en el paquete oficial de Joss)
// 3. ~/.joss/toolchain/<os>-<arch>/ (gestionado automáticamente por Joss)
// 4. clang o zig en PATH del sistema
func ResolveToolchain() ToolchainInfo {
	if envDir := strings.TrimSpace(os.Getenv("JOSS_TOOLCHAIN_DIR")); envDir != "" {
		if tc := probeToolchainDir(envDir, "bundled"); tc.Available() {
			return tc
		}
	}

	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		for _, rel := range []string{
			filepath.Join(exeDir, "toolchain", runtime.GOOS+"-"+runtime.GOARCH),
			filepath.Join(exeDir, "toolchain"),
		} {
			if tc := probeToolchainDir(rel, "bundled"); tc.Available() {
				return tc
			}
		}
	}

	if managedDir := ManagedToolchainDir(); managedDir != "" {
		if tc := probeToolchainDir(managedDir, "managed"); tc.Available() {
			return tc
		}
	}

	if clangPath, err := exec.LookPath("clang"); err == nil && clangPath != "" {
		lldPath, _ := exec.LookPath("ld.lld")
		if lldPath == "" {
			lldPath, _ = exec.LookPath("lld")
		}
		return ToolchainInfo{
			Kind:         "system-clang",
			CompilerPath: clangPath,
			LLDPath:      lldPath,
		}
	}

	if zigPath, err := exec.LookPath("zig"); err == nil && zigPath != "" {
		return ToolchainInfo{
			Kind:         "system-zig",
			CompilerPath: zigPath,
			UseZigCC:     true,
		}
	}

	return ToolchainInfo{}
}

func probeToolchainDir(dir, prefix string) ToolchainInfo {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return ToolchainInfo{}
	}
	exeSuffix := ""
	if runtime.GOOS == "windows" {
		exeSuffix = ".exe"
	}

	zigCandidates := []string{
		filepath.Join(dir, "zig"+exeSuffix),
		filepath.Join(dir, "bin", "zig"+exeSuffix),
	}
	for _, cand := range zigCandidates {
		if fi, statErr := os.Stat(cand); statErr == nil && !fi.IsDir() {
			return ToolchainInfo{
				Kind:         prefix + "-zig",
				CompilerPath: cand,
				UseZigCC:     true,
				RootDir:      dir,
			}
		}
	}

	clangCandidates := []string{
		filepath.Join(dir, "bin", "clang"+exeSuffix),
		filepath.Join(dir, "clang"+exeSuffix),
	}
	for _, cand := range clangCandidates {
		if fi, statErr := os.Stat(cand); statErr == nil && !fi.IsDir() {
			lldPath := ""
			for _, lldName := range []string{"ld.lld" + exeSuffix, "lld" + exeSuffix} {
				lldCand := filepath.Join(filepath.Dir(cand), lldName)
				if lfi, lerr := os.Stat(lldCand); lerr == nil && !lfi.IsDir() {
					lldPath = lldCand
					break
				}
			}
			return ToolchainInfo{
				Kind:         prefix + "-clang",
				CompilerPath: cand,
				LLDPath:      lldPath,
				RootDir:      dir,
			}
		}
	}
	return ToolchainInfo{}
}

// ManagedToolchainDir devuelve ~/.joss/toolchain/<os>-<arch>
func ManagedToolchainDir() string {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return ""
	}
	return filepath.Join(home, ".joss", "toolchain", runtime.GOOS+"-"+runtime.GOARCH)
}

// EnsureRuntimeC devuelve la ruta a joss_rt.c:
// 1. Hint explícito (opts.RuntimeSource)
// 2. Checkout local (runtime/native/joss_rt.c)
// 3. Junto al ejecutable (<exe_dir>/runtime/native/joss_rt.c)
// 4. Extracción automática del runtime C11 embebido dentro del binario de Joss (~/.joss/cache/runtime/<version>/)
func EnsureRuntimeC(hint string) (string, error) {
	if hint != "" {
		if _, err := os.Stat(hint); err == nil {
			return hint, nil
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		dir := cwd
		for i := 0; i < 6; i++ {
			candidate := filepath.Join(dir, "runtime", "native", "joss_rt.c")
			if _, statErr := os.Stat(candidate); statErr == nil {
				return candidate, nil
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	if exePath, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exePath), "runtime", "native", "joss_rt.c")
		if _, statErr := os.Stat(candidate); statErr == nil {
			return candidate, nil
		}
	}

	cacheBase := os.TempDir()
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		cacheBase = filepath.Join(home, ".joss", "cache")
	}
	targetDir := filepath.Join(cacheBase, "runtime", version.Version)
	return runtimeassets.ExtractNativeRuntime(targetDir)
}

// EnsureToolchain garantiza que exista un toolchain nativo listo; si no hay ninguno
// en el paquete ni en el sistema y no está desactivado con JOSS_NO_AUTO_TOOLCHAIN=1,
// descarga automáticamente el toolchain portable autocontenido en ~/.joss/toolchain/.
func EnsureToolchain() (ToolchainInfo, error) {
	if tc := ResolveToolchain(); tc.Available() {
		return tc, nil
	}
	if os.Getenv("JOSS_NO_AUTO_TOOLCHAIN") == "1" {
		return ToolchainInfo{}, fmt.Errorf("no se encontró toolchain nativo (bundled/clang/zig) y JOSS_NO_AUTO_TOOLCHAIN=1 está activo")
	}
	return InstallManagedToolchain()
}

func zigArchiveURL(goos, goarch string) (string, string, error) {
	var zigOS, zigArch, ext string
	switch goos {
	case "windows":
		zigOS = "windows"
		ext = "zip"
	case "linux":
		zigOS = "linux"
		ext = "tar.xz"
	case "darwin":
		zigOS = "macos"
		ext = "tar.xz"
	default:
		return "", "", fmt.Errorf("plataforma no soportada para descarga automática de toolchain: %s/%s", goos, goarch)
	}
	switch goarch {
	case "amd64":
		zigArch = "x86_64"
	case "arm64":
		zigArch = "aarch64"
	default:
		return "", "", fmt.Errorf("arquitectura no soportada para descarga automática de toolchain: %s/%s", goos, goarch)
	}
	name := fmt.Sprintf("zig-%s-%s-%s.%s", zigOS, zigArch, managedZigVersion, ext)
	url := fmt.Sprintf("https://ziglang.org/download/%s/%s", managedZigVersion, name)
	return url, ext, nil
}

// InstallManagedToolchain descarga e instala el toolchain portable autocontenido (Zig/Clang/LLD)
// dentro de ~/.joss/toolchain/<os>-<arch>/ para que el desarrollador solo necesite Joss.
func InstallManagedToolchain() (ToolchainInfo, error) {
	destDir := ManagedToolchainDir()
	if destDir == "" {
		return ToolchainInfo{}, fmt.Errorf("no se pudo determinar el directorio home del usuario para instalar el toolchain")
	}
	if tc := probeToolchainDir(destDir, "managed"); tc.Available() {
		return tc, nil
	}

	url, ext, err := zigArchiveURL(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return ToolchainInfo{}, err
	}

	fmt.Printf("📦 Instalando toolchain nativo autocontenido de Joss (%s)...\n", managedZigVersion)
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return ToolchainInfo{}, fmt.Errorf("descargar toolchain nativo (%s): %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ToolchainInfo{}, fmt.Errorf("descargar toolchain nativo (%s): HTTP %d", url, resp.StatusCode)
	}

	tmpFile, err := os.CreateTemp("", "joss-toolchain-*."+ext)
	if err != nil {
		return ToolchainInfo{}, err
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		_ = tmpFile.Close()
		return ToolchainInfo{}, fmt.Errorf("guardar toolchain descargado: %w", err)
	}
	_ = tmpFile.Close()

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return ToolchainInfo{}, err
	}

	switch ext {
	case "zip":
		if err := extractStripTopZip(tmpPath, destDir); err != nil {
			return ToolchainInfo{}, err
		}
	case "tar.gz":
		if err := extractStripTopTarGz(tmpPath, destDir); err != nil {
			return ToolchainInfo{}, err
		}
	case "tar.xz":
		if err := extractStripTopTarXz(tmpPath, destDir); err != nil {
			return ToolchainInfo{}, fmt.Errorf("extraer toolchain tar.xz: %w", err)
		}
	}

	tc := probeToolchainDir(destDir, "managed")
	if !tc.Available() {
		return ToolchainInfo{}, fmt.Errorf("toolchain extraído en %s pero no se encontró el binario ejecutable", destDir)
	}
	fmt.Printf("✓ Toolchain nativo listo en %s\n", destDir)
	return tc, nil
}

func extractStripTopZip(zipPath, destDir string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zr.Close()
	cleanDest := filepath.Clean(destDir)
	for _, f := range zr.File {
		parts := strings.SplitN(filepath.ToSlash(f.Name), "/", 2)
		if len(parts) < 2 || parts[1] == "" {
			continue
		}
		target := filepath.Join(cleanDest, filepath.FromSlash(parts[1]))
		if !strings.HasPrefix(filepath.Clean(target), cleanDest+string(os.PathSeparator)) {
			continue
		}
		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(target, 0755)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
		if err != nil {
			_ = rc.Close()
			return err
		}
		_, copyErr := io.Copy(out, rc)
		_ = rc.Close()
		_ = out.Close()
		if copyErr != nil {
			return copyErr
		}
	}
	return nil
}

func extractStripTopTarGz(tarPath, destDir string) error {
	f, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	return extractStripTopTar(gz, destDir)
}

func extractStripTopTarXz(tarPath, destDir string) error {
	f, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer f.Close()
	xr, err := xz.NewReader(f)
	if err != nil {
		return err
	}
	return extractStripTopTar(xr, destDir)
}

func extractStripTopTar(r io.Reader, destDir string) error {
	tr := tar.NewReader(r)
	cleanDest := filepath.Clean(destDir)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		parts := strings.SplitN(filepath.ToSlash(hdr.Name), "/", 2)
		if len(parts) < 2 || parts[1] == "" {
			continue
		}
		target := filepath.Join(cleanDest, filepath.FromSlash(parts[1]))
		if !strings.HasPrefix(filepath.Clean(target), cleanDest+string(os.PathSeparator)) {
			continue
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			mode := os.FileMode(hdr.Mode) & 0777
			if mode == 0 {
				mode = 0644
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(out, tr)
			closeErr := out.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			_ = os.Remove(target)
			if err := os.Symlink(hdr.Linkname, target); err != nil {
				return err
			}
		case tar.TypeLink:
			linkParts := strings.SplitN(filepath.ToSlash(hdr.Linkname), "/", 2)
			if len(linkParts) < 2 || linkParts[1] == "" {
				continue
			}
			linkTarget := filepath.Join(cleanDest, filepath.FromSlash(linkParts[1]))
			if !strings.HasPrefix(filepath.Clean(linkTarget), cleanDest+string(os.PathSeparator)) {
				continue
			}
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			_ = os.Remove(target)
			if err := os.Link(linkTarget, target); err != nil {
				return err
			}
		}
	}
	return nil
}
