package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestViewDefaultsFlashVariablesAndRendersSafely(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origWd)

	viewsDir := filepath.Join("app", "views", "test")
	if err := os.MkdirAll(viewsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	viewContent := `<div>
		{{ ($success) ? {
			<span class="ok">{{ $success }}</span>
		} : {
			<span class="no-success">Sin exito</span>
		} }}
		{{ ($error) ? {
			<span class="err">{{ $error }}</span>
		} : {
			<span class="no-error">Sin error</span>
		} }}
		<h1>{{ $title }}</h1>
	</div>`

	if err := os.WriteFile(filepath.Join(viewsDir, "profile.joss.html"), []byte(viewContent), 0o644); err != nil {
		t.Fatal(err)
	}

	r := NewRuntime()
	out := r.executeViewMethod(nil, "render", []interface{}{
		"test.profile",
		map[string]interface{}{
			"title": "Perfil de Prueba",
		},
	})

	html, ok := out.(string)
	if !ok {
		t.Fatalf("expected string output, got %T: %v", out, out)
	}

	if !strings.Contains(html, "Sin exito") {
		t.Fatalf("expected 'Sin exito' when $success is default empty, got: %s", html)
	}
	if !strings.Contains(html, "Sin error") {
		t.Fatalf("expected 'Sin error' when $error is default empty, got: %s", html)
	}
	if !strings.Contains(html, "Perfil de Prueba") {
		t.Fatalf("expected 'Perfil de Prueba' in output, got: %s", html)
	}
}

func TestViewErrorContextIncludesViewName(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origWd)

	viewsDir := filepath.Join("app", "views", "test")
	if err := os.MkdirAll(viewsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	viewContent := `<div>{{ $variableInexistenteQueDebeFallar }}</div>`

	if err := os.WriteFile(filepath.Join(viewsDir, "broken.joss.html"), []byte(viewContent), 0o644); err != nil {
		t.Fatal(err)
	}

	r := NewRuntime()

	defer func() {
		rec := recover()
		if rec == nil {
			t.Fatalf("expected view rendering to panic on undefined variable")
		}
		jErr, ok := rec.(*JossError)
		if !ok {
			t.Fatalf("expected *JossError, got %T: %v", rec, rec)
		}
		if !strings.Contains(jErr.Message, "Error en vista 'test.broken'") {
			t.Fatalf("expected error message to mention view name, got: %s", jErr.Message)
		}
	}()

	r.executeViewMethod(nil, "render", []interface{}{
		"test.broken",
		map[string]interface{}{},
	})
}

func TestViewPreservesJavaScriptTemplateLiterals(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origWd)

	viewsDir := filepath.Join("app", "views", "test")
	if err := os.MkdirAll(viewsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	viewContent := `<script>
		const totalCount = 5;
		const msg = ` + "`" + `${totalCount} items` + "`" + `;
	</script>`

	if err := os.WriteFile(filepath.Join(viewsDir, "script.joss.html"), []byte(viewContent), 0o644); err != nil {
		t.Fatal(err)
	}

	r := NewRuntime()
	out := r.executeViewMethod(nil, "render", []interface{}{
		"test.script",
		map[string]interface{}{},
	})

	html, ok := out.(string)
	if !ok {
		t.Fatalf("expected string output, got %T: %v", out, out)
	}

	if !strings.Contains(html, "${totalCount}") {
		t.Fatalf("expected '${totalCount}' to be preserved in output, got: %s", html)
	}
}

func TestViewSingleBranchTernary(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origWd)

	viewsDir := filepath.Join("app", "views", "test")
	if err := os.MkdirAll(viewsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	viewContent := `<div>
		{{ ($showAdmin) ? {
			<span class="badge">Admin</span>
		} }}
		{{ ($showGuest) ? {
			<span class="badge">Guest</span>
		} }}
	</div>`

	if err := os.WriteFile(filepath.Join(viewsDir, "single.joss.html"), []byte(viewContent), 0o644); err != nil {
		t.Fatal(err)
	}

	r := NewRuntime()
	out := r.executeViewMethod(nil, "render", []interface{}{
		"test.single",
		map[string]interface{}{
			"showAdmin": true,
			"showGuest": false,
		},
	})

	html, ok := out.(string)
	if !ok {
		t.Fatalf("expected string output, got %T: %v", out, out)
	}

	if !strings.Contains(html, "Admin") {
		t.Fatalf("expected 'Admin' when $showAdmin is true, got: %s", html)
	}
	if strings.Contains(html, "Guest") {
		t.Fatalf("did not expect 'Guest' when $showGuest is false, got: %s", html)
	}
}
