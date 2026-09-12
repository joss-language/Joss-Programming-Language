package pluginruntime

import (
	"fmt"
	"sync"
	"testing"

	"github.com/jossecurity/joss/pkg/plugincompiler/ir"
)

type recordingHost struct {
	name string
	mu   sync.Mutex
	seen int
}

func (h *recordingHost) CallHostFunction(string, []interface{}) (interface{}, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.seen++
	return h.name, nil
}

func TestRegistryViewsShareCatalogButBindCallsToTheirOwnHost(t *testing.T) {
	parentHost := &recordingHost{name: "parent"}
	forkHost := &recordingHost{name: "fork"}
	parent := NewPluginRegistry(parentHost)
	fork := parent.WithHost(forkHost)
	plugin := &Plugin{
		Name:   "host_probe",
		Format: FormatJPBC,
		jpbcModule: &JPBCModule{
			ConstantPool: []interface{}{"probe"},
			Functions: map[string]*JPBCFunction{
				"run": {Name: "run", Instructions: []JPBCInstruction{{Op: ir.OpCallStatic, ConstIdx: 0}, {Op: ir.OpReturn}}},
			},
		},
	}
	if err := parent.Register(plugin); err != nil {
		t.Fatal(err)
	}
	if fork.Get(plugin.Name) != plugin {
		t.Fatal("fork-bound view does not observe the shared plugin catalog")
	}

	const calls = 16
	var wg sync.WaitGroup
	for i := 0; i < calls; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			value, err := parent.CallFunction("host_probe", "run", nil)
			if err != nil || value != "parent" {
				t.Errorf("parent call = %v, %v", value, err)
			}
		}()
		go func() {
			defer wg.Done()
			value, err := fork.CallFunction("host_probe", "run", nil)
			if err != nil || value != "fork" {
				t.Errorf("fork call = %v, %v", value, err)
			}
		}()
	}
	wg.Wait()
	if parentHost.seen != calls || forkHost.seen != calls {
		t.Fatalf("host calls = parent:%d fork:%d, want %d each", parentHost.seen, forkHost.seen, calls)
	}
}

func TestRegistryCatalogSupportsConcurrentRegisterAndLookup(t *testing.T) {
	registry := NewPluginRegistry(nil)
	view := registry.WithHost(nil)
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		i := i
		wg.Add(2)
		go func() {
			defer wg.Done()
			if err := registry.Register(&Plugin{Name: fmt.Sprintf("plugin-%d", i)}); err != nil {
				t.Errorf("register: %v", err)
			}
		}()
		go func() {
			defer wg.Done()
			_ = view.List()
		}()
	}
	wg.Wait()
	if got := len(view.List()); got != 32 {
		t.Fatalf("catalog size = %d, want 32", got)
	}
}
