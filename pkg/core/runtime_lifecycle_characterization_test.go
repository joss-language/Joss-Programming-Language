package core

import (
	"bufio"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/pkg/pluginruntime"
)

func TestNewRuntimeProvidesCanonicalHostState(t *testing.T) {
	runtime := NewRuntime()
	defer runtime.Free()
	for _, name := range []string{"cout", "cin", "cerr", "endl", "JOSS_VERSION", "View", "Request", "Response"} {
		if _, exists := runtime.Variables[name]; !exists {
			t.Errorf("new runtime is missing canonical host binding %q", name)
		}
		if !runtime.HostGlobals[name] {
			t.Errorf("canonical host binding %q is not marked as host-global", name)
		}
	}
	if runtime.MaxCallDepth != DefaultMaxCallDepth {
		t.Fatalf("MaxCallDepth = %d, want %d", runtime.MaxCallDepth, DefaultMaxCallDepth)
	}
}

func TestForkCopiesConfigurationAndIsolatesMutableExecutionMaps(t *testing.T) {
	parent := NewRuntime()
	defer parent.Free()
	parent.Env["APP_MODE"] = "test"
	parent.Variables["requestValue"] = map[string]interface{}{"id": int64(1)}
	parent.VarTypes["requestValue"] = "map"
	parent.Routes["GET"] = map[string]interface{}{"/one": "handler"}
	parent.CustomMiddlewares["auth"] = "middleware"
	parent.Enums["Status"] = &EnumDefinition{Name: "Status", Cases: map[string]*EnumValue{"Ready": {EnumName: "Status", Name: "Ready", Value: "ready"}}}
	parent.PluginRegistry = pluginruntime.NewPluginRegistry(parent)

	fork := parent.Fork()
	defer fork.Free()
	if fork.PluginRegistry != parent.PluginRegistry {
		t.Fatal("fork must share the configured plugin registry")
	}
	if fork.Env["APP_MODE"] != "test" || fork.VarTypes["requestValue"] != "map" {
		t.Fatal("fork did not copy runtime configuration")
	}

	fork.Env["APP_MODE"] = "fork"
	fork.Variables["requestValue"].(map[string]interface{})["id"] = int64(2)
	fork.Routes["GET"]["/two"] = "fork-handler"
	fork.CustomMiddlewares["fork"] = "middleware"
	delete(fork.Enums, "Status")
	if parent.Env["APP_MODE"] != "test" {
		t.Fatal("fork environment mutation leaked into parent")
	}
	if parent.Variables["requestValue"].(map[string]interface{})["id"] != int64(1) {
		t.Fatal("fork variable map mutation leaked into parent")
	}
	if _, exists := parent.Routes["GET"]["/two"]; exists {
		t.Fatal("fork route mutation leaked into parent")
	}
	if _, exists := parent.CustomMiddlewares["fork"]; exists {
		t.Fatal("fork middleware mutation leaked into parent")
	}
	if parent.Enums["Status"] == nil {
		t.Fatal("fork enum catalog mutation leaked into parent")
	}
}

func TestForkPreservesEnumCatalogAndStartsWithCleanRequestState(t *testing.T) {
	parent := NewRuntime()
	defer parent.Free()
	parent.Enums["Status"] = &EnumDefinition{Name: "Status", Cases: map[string]*EnumValue{"Ready": {EnumName: "Status", Name: "Ready", Value: "ready"}}}
	parent.CurrentSource = "bootstrap"
	parent.CurrentFile = "routes.joss"
	parent.SEO = &SEOData{Title: "bootstrap"}
	parent.classMetadataCache["Parent"] = &classMetadata{}

	child := parent.Fork()
	defer child.Free()
	if child.Enums["Status"] == nil {
		t.Fatal("fork lost the parent enum catalog")
	}
	if child.CurrentSource != "" || child.CurrentFile != "" || child.SEO != nil || len(child.classMetadataCache) != 0 {
		t.Fatal("fork inherited request/execution state")
	}
	child.Enums["ChildOnly"] = &EnumDefinition{Name: "ChildOnly"}
	if parent.Enums["ChildOnly"] != nil {
		t.Fatal("child enum catalog mutation leaked into parent")
	}
}

func TestRuntimePoolConcurrentAcquireAndFree(t *testing.T) {
	const workers = 8
	const iterations = 12
	var wg sync.WaitGroup
	errors := make(chan error, workers)
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for iteration := 0; iteration < iterations; iteration++ {
				runtime := NewRuntime()
				if _, contaminated := runtime.Variables["poolProbe"]; contaminated || runtime.Env["POOL_PROBE"] != "" {
					errors <- fmt.Errorf("worker %d acquired contaminated runtime", worker)
					runtime.Free()
					return
				}
				runtime.Variables["poolProbe"] = worker
				runtime.Env["POOL_PROBE"] = fmt.Sprint(worker)
				runtime.Free()
			}
		}(worker)
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		t.Fatal(err)
	}
}

func TestFreeRemovesPerExecutionAndPerRequestStateBeforeReuse(t *testing.T) {
	runtime := NewRuntime()
	runtime.Env["REQUEST_SECRET"] = "must-not-survive"
	runtime.Variables["requestValue"] = int64(42)
	runtime.VarTypes["requestValue"] = "int"
	runtime.Constants["requestValue"] = true
	runtime.HostGlobals["requestValue"] = true
	runtime.ProjectRoot = "request-root"
	runtime.CurrentSource = "request-source"
	runtime.CurrentFile = "request.joss"
	runtime.SEO = &SEOData{Title: "request-title"}
	runtime.captureEnvironment = &ClosureEnvironment{Variables: map[string]interface{}{"captured": true}}
	runtime.cinReader = bufio.NewReader(strings.NewReader("secret"))
	runtime.cinTokens = []string{"secret"}
	runtime.currentGenerator = newGenerator()
	runtime.generatorIndex = 7
	runtime.topDefers = []*parser.DeferStatement{{}}
	runtime.classMetadataCache["RequestClass"] = &classMetadata{}
	runtime.Free()

	reused := NewRuntime()
	defer reused.Free()
	if reused.Env["REQUEST_SECRET"] != "" {
		t.Fatal("pooled runtime retained request environment")
	}
	if _, exists := reused.Variables["requestValue"]; exists {
		t.Fatal("pooled runtime retained request variable")
	}
	if reused.ProjectRoot != "" || reused.CurrentSource != "" || reused.CurrentFile != "" {
		t.Fatalf("pooled runtime retained source context: root=%q source=%q file=%q", reused.ProjectRoot, reused.CurrentSource, reused.CurrentFile)
	}
	if reused.SEO != nil || reused.captureEnvironment != nil || reused.cinReader != nil || len(reused.cinTokens) != 0 {
		t.Fatal("pooled runtime retained request/session execution helpers")
	}
	if reused.currentGenerator != nil || reused.generatorIndex != 0 || len(reused.topDefers) != 0 {
		t.Fatal("pooled runtime retained generator/defer execution state")
	}
	if len(reused.classMetadataCache) != 0 {
		t.Fatal("pooled runtime retained class metadata from the previous execution")
	}
}

func BenchmarkRuntimeLifecycle(b *testing.B) {
	b.Run("construct", func(b *testing.B) {
		b.ReportAllocs()
		for iteration := 0; iteration < b.N; iteration++ {
			_ = newRuntimeState().(*Runtime)
		}
	})
	b.Run("pool_acquire_free", func(b *testing.B) {
		b.ReportAllocs()
		for iteration := 0; iteration < b.N; iteration++ {
			runtime := NewRuntime()
			runtime.Free()
		}
	})
	parent := NewRuntime()
	b.Cleanup(parent.Free)
	b.Run("fork_free", func(b *testing.B) {
		b.ReportAllocs()
		for iteration := 0; iteration < b.N; iteration++ {
			runtime := parent.Fork()
			runtime.Free()
		}
	})
}
