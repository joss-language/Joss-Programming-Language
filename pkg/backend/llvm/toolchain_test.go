package llvm

import (
	"archive/tar"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jossecurity/joss/pkg/backend/native"
	"github.com/jossecurity/joss/pkg/ir"
	"github.com/ulikunitz/xz"
)

func TestEnsureRuntimeC_ExtractsEmbeddedOutsideRepo(t *testing.T) {
	emptyDir := t.TempDir()
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(emptyDir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origWD) }()

	t.Setenv("HOME", emptyDir)
	rtPath, err := EnsureRuntimeC("")
	if err != nil {
		t.Fatalf("EnsureRuntimeC failed outside repo: %v", err)
	}
	rtDir := filepath.Dir(rtPath)
	for _, f := range []string{"joss_rt.h", "joss_raii.h", "joss_rt.c", "joss_memory_rt.inc", "joss_web_rt.c", "joss_services_rt.inc", "joss_ws_rt.inc", "joss_cron_rt.inc", "joss_builtins_rt.inc", "joss_session_rt.inc", "joss_redis_rt.inc", "joss_zip_rt.inc", "joss_concurrency_rt.c", "joss_db_rt.c", "joss_crypto_rt.c", "joss_gui_rt.c"} {
		if info, statErr := os.Stat(filepath.Join(rtDir, f)); statErr != nil || info.Size() == 0 {
			t.Fatalf("expected embedded runtime file %s in %s: %v", f, rtDir, statErr)
		}
	}
}

func TestResolveToolchain_PrefersBundledZigOverSystem(t *testing.T) {
	fakeDir := t.TempDir()
	zigName := "zig"
	if runtime.GOOS == "windows" {
		zigName = "zig.exe"
	}
	fakeZig := filepath.Join(fakeDir, zigName)
	if err := os.WriteFile(fakeZig, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("JOSS_TOOLCHAIN_DIR", fakeDir)

	tc := ResolveToolchain()
	if !tc.Available() {
		t.Fatalf("expected bundled toolchain to be available")
	}
	if tc.Kind != "bundled-zig" || !tc.UseZigCC || tc.CompilerPath != fakeZig {
		t.Fatalf("unexpected toolchain info: %+v", tc)
	}
}

func TestEndToEnd_CompileFromEmptyDirUsingEmbeddedRuntime(t *testing.T) {
	if !ResolveToolchain().Available() {
		t.Skip("no local clang/zig toolchain available")
	}
	emptyDir := t.TempDir()
	origWD, _ := os.Getwd()
	if err := os.Chdir(emptyDir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origWD) }()
	t.Setenv("HOME", emptyDir)

	prog := ir.NewProgram("embedded_rt_test")
	mainFn := ir.NewFunction("main", ir.TypeI64)
	mainFn.EntryBlock.AddInstruction(&ir.CallRuntimeInst{
		Func: "joss_print_i64",
		Args: []ir.Value{ir.NewConstInt(2026, ir.TypeI64)},
	})
	mainFn.EntryBlock.SetTerminator(&ir.ReturnTerminator{Val: ir.NewConstInt(0, ir.TypeI64)})
	prog.AddFunction(mainFn)

	outBin := filepath.Join(emptyDir, "app_standalone")
	if runtime.GOOS == "windows" {
		outBin += ".exe"
	}
	if _, err := native.BuildProgram(prog, native.BuildOptions{
		Target:     native.Target{OS: runtime.GOOS, Arch: runtime.GOARCH},
		Backend:    native.BackendLLVM,
		OutputPath: outBin,
	}); err != nil {
		t.Fatalf("BuildProgram outside repo failed: %v", err)
	}

	out, err := exec.Command(outBin).CombinedOutput()
	if err != nil {
		t.Fatalf("exec failed: %v\n%s", err, string(out))
	}
	if strings.TrimSpace(string(out)) != "2026" {
		t.Fatalf("expected 2026, got %q", strings.TrimSpace(string(out)))
	}
}

func TestExtractStripTopTarXz_ZeroExternalTools(t *testing.T) {
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "zig-linux-aarch64-0.14.0.tar.xz")
	destDir := filepath.Join(tmpDir, "extracted")

	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	xw, err := xz.NewWriter(f)
	if err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	tw := tar.NewWriter(xw)

	payload := []byte("#!/bin/sh\necho zig-ok\n")
	if err := tw.WriteHeader(&tar.Header{
		Name:     "zig-linux-aarch64-0.14.0/zig",
		Mode:     0755,
		Size:     int64(len(payload)),
		Typeflag: tar.TypeReg,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := tw.WriteHeader(&tar.Header{
		Name:     "zig-linux-aarch64-0.14.0/lib/std.zig",
		Mode:     0644,
		Size:     4,
		Typeflag: tar.TypeReg,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte("pub\n")); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := xw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	if err := extractStripTopTarXz(archivePath, destDir); err != nil {
		t.Fatalf("extractStripTopTarXz failed: %v", err)
	}
	gotZig, err := os.ReadFile(filepath.Join(destDir, "zig"))
	if err != nil {
		t.Fatalf("expected extracted zig binary: %v", err)
	}
	if string(gotZig) != string(payload) {
		t.Fatalf("unexpected zig contents: %q", string(gotZig))
	}
	gotStd, err := os.ReadFile(filepath.Join(destDir, "lib", "std.zig"))
	if err != nil || string(gotStd) != "pub\n" {
		t.Fatalf("expected extracted lib/std.zig, got %q (err=%v)", string(gotStd), err)
	}
}

func TestDownloadToolchainArchiveFromURLs_FallbackOnErrorAndStall(t *testing.T) {
	origIdle := toolchainDownloadIdleTimeout
	origHeader := toolchainResponseHeaderWait
	origAttempt := toolchainAttemptMaxDuration
	toolchainDownloadIdleTimeout = 150 * time.Millisecond
	toolchainResponseHeaderWait = 300 * time.Millisecond
	toolchainAttemptMaxDuration = 2 * time.Second
	defer func() {
		toolchainDownloadIdleTimeout = origIdle
		toolchainResponseHeaderWait = origHeader
		toolchainAttemptMaxDuration = origAttempt
	}()

	errServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "mirror unavailable", http.StatusServiceUnavailable)
	}))
	defer errServer.Close()

	stallServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1024")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("partial"))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}))
	defer stallServer.Close()

	expected := "valid-toolchain-payload-data"
	okServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(expected))
	}))
	defer okServer.Close()

	path, err := downloadToolchainArchiveFromURLs([]string{
		errServer.URL + "/zig.tar.xz",
		stallServer.URL + "/zig.tar.xz",
		okServer.URL + "/zig.tar.xz",
	}, "tar.xz")
	if err != nil {
		t.Fatalf("expected fallback mirror to succeed, got err: %v", err)
	}
	defer os.Remove(path)

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read downloaded archive: %v", err)
	}
	if string(got) != expected {
		t.Fatalf("expected %q, got %q", expected, string(got))
	}
}

func TestZigArchiveCandidateURLs_RespectsOverrideAndCommunityMirrors(t *testing.T) {
	mirrorListServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("# comment\nhttps://mirror-a.example/zig/\nhttps://mirror-b.example\n"))
	}))
	defer mirrorListServer.Close()

	origMirrorsURL := zigCommunityMirrorsURL
	zigCommunityMirrorsURL = mirrorListServer.URL
	defer func() { zigCommunityMirrorsURL = origMirrorsURL }()

	t.Setenv("JOSS_TOOLCHAIN_URL", "")
	urls, ext, err := zigArchiveCandidateURLs("linux", "arm64")
	if err != nil {
		t.Fatalf("zigArchiveCandidateURLs failed: %v", err)
	}
	if ext != "tar.xz" {
		t.Fatalf("expected tar.xz, got %q", ext)
	}
	if len(urls) < 4 {
		t.Fatalf("expected multiple mirror candidates, got %v", urls)
	}
	if urls[0] != "https://mirror-a.example/zig/zig-linux-aarch64-0.14.0.tar.xz" {
		t.Fatalf("unexpected first mirror URL: %q", urls[0])
	}
	if urls[len(urls)-1] != "https://ziglang.org/download/0.14.0/zig-linux-aarch64-0.14.0.tar.xz" {
		t.Fatalf("expected official ziglang.org as final fallback, got %q", urls[len(urls)-1])
	}

	t.Setenv("JOSS_TOOLCHAIN_URL", "https://internal.corp/zig-custom.tar.xz")
	overrideURLs, _, err := zigArchiveCandidateURLs("linux", "arm64")
	if err != nil {
		t.Fatalf("zigArchiveCandidateURLs override failed: %v", err)
	}
	if len(overrideURLs) != 1 || overrideURLs[0] != "https://internal.corp/zig-custom.tar.xz" {
		t.Fatalf("unexpected override URLs: %v", overrideURLs)
	}
}

func TestPureC11_RSASignSHA256_MatchesGoCrypto(t *testing.T) {
	if !ResolveToolchain().Available() {
		t.Skip("no local clang/zig toolchain available")
	}

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	pkcs1PEM := string(pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}))
	pkcs8DER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	pkcs8PEM := string(pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: pkcs8DER,
	}))

	signingString := "date: Tue, 24 Nov 2026 20:00:00 GMT\n(request-target): get /n/ns/b/bucket/o/1/cms-media/test.png\nhost: objectstorage.mx-queretaro-1.oraclecloud.com"
	h := sha256.Sum256([]byte(signingString))
	expectedSig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, h[:])
	if err != nil {
		t.Fatal(err)
	}
	expectedB64 := base64.StdEncoding.EncodeToString(expectedSig)

	prog := ir.NewProgram("rsa_pure_c11_test")
	mainFn := ir.NewFunction("main", ir.TypeI64)
	sig1 := mainFn.NewTemp(ir.TypeString, "sig1")
	mainFn.EntryBlock.AddInstruction(&ir.CallRuntimeInst{
		Func: "joss_rsa_sign_sha256_pure",
		Args: []ir.Value{ir.NewConstString(signingString), ir.NewConstString(pkcs1PEM)},
		Dest: sig1,
	})
	mainFn.EntryBlock.AddInstruction(&ir.CallRuntimeInst{
		Func: "joss_print_string",
		Args: []ir.Value{sig1},
	})
	sig2 := mainFn.NewTemp(ir.TypeString, "sig2")
	mainFn.EntryBlock.AddInstruction(&ir.CallRuntimeInst{
		Func: "joss_rsa_sign_sha256_pure",
		Args: []ir.Value{ir.NewConstString(signingString), ir.NewConstString(pkcs8PEM)},
		Dest: sig2,
	})
	mainFn.EntryBlock.AddInstruction(&ir.CallRuntimeInst{
		Func: "joss_print_string",
		Args: []ir.Value{sig2},
	})
	mainFn.EntryBlock.SetTerminator(&ir.ReturnTerminator{Val: ir.NewConstInt(0, ir.TypeI64)})
	prog.AddFunction(mainFn)

	outBin := filepath.Join(t.TempDir(), "rsa_pure_test")
	if runtime.GOOS == "windows" {
		outBin += ".exe"
	}
	if _, err := native.BuildProgram(prog, native.BuildOptions{
		Target:     native.Target{OS: runtime.GOOS, Arch: runtime.GOARCH},
		Backend:    native.BackendLLVM,
		OutputPath: outBin,
	}); err != nil {
		t.Fatalf("BuildProgram failed: %v", err)
	}

	out, err := exec.Command(outBin).CombinedOutput()
	if err != nil {
		t.Fatalf("exec failed: %v\n%s", err, string(out))
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 signature lines, got %d: %q", len(lines), string(out))
	}
	for idx, line := range lines {
		gotB64 := strings.TrimSpace(line)
		if gotB64 != expectedB64 {
			t.Fatalf("signature #%d mismatch:\ngot:  %s\nwant: %s", idx+1, gotB64, expectedB64)
		}
		gotRaw, decErr := base64.StdEncoding.DecodeString(gotB64)
		if decErr != nil {
			t.Fatalf("signature #%d invalid base64: %v", idx+1, decErr)
		}
		if verifyErr := rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA256, h[:], gotRaw); verifyErr != nil {
			t.Fatalf("signature #%d failed crypto/rsa verification: %v", idx+1, verifyErr)
		}
	}
}
