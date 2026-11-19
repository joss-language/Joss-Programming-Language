package native_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jossecurity/joss/pkg/analyzer"
	_ "github.com/jossecurity/joss/pkg/backend/llvm"
	backend "github.com/jossecurity/joss/pkg/backend/native"
	core "github.com/jossecurity/joss/pkg/core"
	"github.com/jossecurity/joss/pkg/ir"
	"github.com/jossecurity/joss/pkg/parser"
)

func readProcessRSSKB(pid int) int64 {
	if runtime.GOOS != "linux" {
		return -1
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return -1
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "VmRSS:") {
			var kb int64
			_, _ = fmt.Sscanf(strings.TrimSpace(strings.TrimPrefix(line, "VmRSS:")), "%d", &kb)
			return kb
		}
	}
	return -1
}

func TestBenchmark_NativeVsGo_ComputeAndWeb(t *testing.T) {
	nc := backend.NewNativeCompiler()
	if !nc.HasNativeToolchain() {
		t.Skip("clang not available on PATH")
	}

	t.Run("Compute_Fib28", func(t *testing.T) {
		source := `
public func fib(int $n): int {
	($n <= 1) ? {
		return $n;
	}
	return fib($n - 1) + fib($n - 2);
}

print(fib(28));
`
		// 1. Go Interpreter
		p := parser.NewParser(parser.NewLexer(source))
		prog := p.ParseProgram()
		units := []analyzer.SourceUnit{{Path: "fib28.joss", Program: prog}}
		prep := analyzer.PrepareProgram(units, core.BuildAnalysisEnvironment())
		if prep.HasErrors() {
			t.Fatalf("analysis errors: %v", prep.Diagnostics)
		}

		tGo0 := time.Now()
		var goOut bytes.Buffer
		rt := core.NewRuntime()
		rt.Out = &goOut
		rt.ExecutePrepared(prep)
		rt.Free()
		goDur := time.Since(tGo0)

		// 2. Lower to IR
		irProg, err := ir.NewLowerer(prep, nil).LowerProgram("fib28")
		if err != nil {
			t.Fatalf("lowering failed: %v", err)
		}
		if err := ir.NewVerifier().Verify(irProg); err != nil {
			t.Fatalf("verify failed: %v", err)
		}

		tmpDir := t.TempDir()
		nativeExe := filepath.Join(tmpDir, "fib28_native")
		llvmExe := filepath.Join(tmpDir, "fib28_llvm")

		resNative, err := backend.BuildProgram(irProg, backend.BuildOptions{
			Target:     backend.Target{OS: runtime.GOOS, Arch: runtime.GOARCH},
			Backend:    backend.BackendNative,
			OutputPath: nativeExe,
		})
		if err != nil {
			t.Fatalf("BackendNative build failed: %v", err)
		}

		resLLVM, err := backend.BuildProgram(irProg, backend.BuildOptions{
			Target:     backend.Target{OS: runtime.GOOS, Arch: runtime.GOARCH},
			Backend:    backend.BackendLLVM,
			OutputPath: llvmExe,
		})
		if err != nil {
			t.Fatalf("BackendLLVM build failed: %v", err)
		}

		tNat0 := time.Now()
		outNat, err := exec.Command(nativeExe).CombinedOutput()
		natDur := time.Since(tNat0)
		if err != nil {
			t.Fatalf("native exec failed: %v", err)
		}

		tLLVM0 := time.Now()
		outLLVM, err := exec.Command(llvmExe).CombinedOutput()
		llvmDur := time.Since(tLLVM0)
		if err != nil {
			t.Fatalf("llvm exec failed: %v", err)
		}

		want := "317811"
		if strings.TrimSpace(goOut.String()) != want || strings.TrimSpace(string(outNat)) != want || strings.TrimSpace(string(outLLVM)) != want {
			t.Fatalf("mismatch: go=%q native=%q llvm=%q", goOut.String(), string(outNat), string(outLLVM))
		}

		t.Logf("[Compute fib(28)] Go Interpreter: exec=%v", goDur)
		t.Logf("[Compute fib(28)] Native Integrated: exec=%v (%.1fx faster), size=%d bytes", natDur, float64(goDur)/float64(natDur), resNative.ExecutableSize)
		t.Logf("[Compute fib(28)] Native LLVM+LLD:   exec=%v (%.1fx faster), size=%d bytes", llvmDur, float64(goDur)/float64(llvmDur), resLLVM.ExecutableSize)
	})

	t.Run("WebApp_TutorialWeb_NativeVsGoServer", func(t *testing.T) {
		origWd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		repoRoot := filepath.Clean(filepath.Join(origWd, "..", ".."))
		webDir := filepath.Join(repoRoot, "ejemplos", "tutorial-web")
		tmpDir := t.TempDir()

		// Build Go Server benchmark binary (same pkg/server + core.Runtime used by joss server / runner)
		goSrvSrcDir := filepath.Join(tmpDir, "gosrv")
		if err := os.MkdirAll(goSrvSrcDir, 0755); err != nil {
			t.Fatal(err)
		}
		goSrvMain := `package main
import "github.com/jossecurity/joss/pkg/server"
func main() {
	server.Start(nil)
}
`
		if err := os.WriteFile(filepath.Join(goSrvSrcDir, "main.go"), []byte(goSrvMain), 0644); err != nil {
			t.Fatal(err)
		}
		goSrvBin := filepath.Join(tmpDir, "tutorial_web_go")
		buildGoCmd := exec.Command("go", "build", "-tags", "nodb noredis nowebview2 noembedlocales", "-o", goSrvBin, filepath.Join(goSrvSrcDir, "main.go"))
		buildGoCmd.Dir = repoRoot
		if out, err := buildGoCmd.CombinedOutput(); err != nil {
			t.Fatalf("building Go server binary failed: %v\n%s", err, string(out))
		}
		goBinStat, _ := os.Stat(goSrvBin)

		// Build Native AOT Web App binary (BackendLLVM, zero Go runtime)
		if err := os.Chdir(webDir); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(origWd)

		units, parseDiags := analyzer.LoadProject("main.joss", "app")
		if len(parseDiags) > 0 {
			t.Fatalf("parse diags: %v", parseDiags)
		}
		report := core.AnalyzeSourceUnits(units)
		if report.HasErrors() {
			t.Fatalf("analysis errors: %v", report.Diagnostics)
		}
		reach := analyzer.BuildReachabilityGraph(report.Prepared, analyzer.ReachabilityOptions{
			Entrypoint: "main.joss",
			RouteFiles: []string{"routes.joss", "api.joss"},
		})
		irProg, err := ir.LowerAndVerify(report.Prepared, "tutorial_web", reach)
		if err != nil {
			t.Fatalf("LowerAndVerify failed: %v", err)
		}

		nativeWebBin := filepath.Join(tmpDir, "tutorial_web_native")
		resNat, err := backend.BuildProgram(irProg, backend.BuildOptions{
			Target:     backend.Target{OS: runtime.GOOS, Arch: runtime.GOARCH},
			Backend:    backend.BackendLLVM,
			OutputPath: nativeWebBin,
		})
		if err != nil {
			t.Fatalf("BuildProgram(BackendLLVM) failed: %v", err)
		}

		const numReqs = 50
		benchServer := func(label, binPath, port string) (coldStart time.Duration, reqTotal time.Duration, rssKB int64) {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, binPath)
			cmd.Dir = webDir
			cmd.Env = append(os.Environ(), "PORT="+port, "DISABLE_WATCHDOG=true", "APP_ENV=production")
			tStart := time.Now()
			if err := cmd.Start(); err != nil {
				t.Fatalf("%s start failed: %v", label, err)
			}
			defer func() {
				if cmd.Process != nil {
					_ = cmd.Process.Kill()
				}
			}()

			addr := "127.0.0.1:" + port
			ready := false
			for i := 0; i < 200; i++ {
				if resp, rerr := httpGetLocal(addr, "/"); rerr == nil && strings.Contains(resp, "200 OK") {
					coldStart = time.Since(tStart)
					ready = true
					break
				}
				time.Sleep(5 * time.Millisecond)
			}
			if !ready {
				t.Fatalf("%s failed to become ready on %s", label, addr)
			}

			tReq := time.Now()
			for i := 0; i < numReqs; i++ {
				path := "/"
				if i%2 == 1 {
					path = "/saludo/Joss"
				}
				resp, rerr := httpGetLocal(addr, path)
				if rerr != nil || !strings.Contains(resp, "200 OK") {
					t.Fatalf("%s req %d (%s) failed: err=%v", label, i, path, rerr)
				}
			}
			reqTotal = time.Since(tReq)
			rssKB = readProcessRSSKB(cmd.Process.Pid)
			return coldStart, reqTotal, rssKB
		}

		goCold, goTotal, goRSS := benchServer("Go-Server", goSrvBin, "18996")
		natCold, natTotal, natRSS := benchServer("Native-AOT", nativeWebBin, "18997")

		goRPS := float64(numReqs) / goTotal.Seconds()
		natRPS := float64(numReqs) / natTotal.Seconds()

		t.Logf("[Web tutorial-web] Go Server:  binary=%d bytes (%.2f MB), cold_start=%v, %d reqs=%v (%.0f req/s, avg=%v), RSS=%d KB",
			goBinStat.Size(), float64(goBinStat.Size())/(1024*1024), goCold, numReqs, goTotal, goRPS, goTotal/numReqs, goRSS)
		t.Logf("[Web tutorial-web] Native AOT: binary=%d bytes (%.2f MB), cold_start=%v, %d reqs=%v (%.0f req/s, avg=%v), RSS=%d KB",
			resNat.ExecutableSize, float64(resNat.ExecutableSize)/(1024*1024), natCold, numReqs, natTotal, natRPS, natTotal/numReqs, natRSS)
		t.Logf("[Web tutorial-web] Ratios: binary=%.1fx smaller, cold_start=%.1fx faster, throughput=%.2fx, memory=%.1fx less RSS",
			float64(goBinStat.Size())/float64(resNat.ExecutableSize),
			float64(goCold)/float64(natCold),
			natRPS/goRPS,
			float64(goRSS)/float64(natRSS))
	})
}
