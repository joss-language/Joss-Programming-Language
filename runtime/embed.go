package runtimeassets

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed native/joss_rt.h native/joss_rt.c native/joss_web_rt.c native/joss_concurrency_rt.c native/joss_db_rt.c native/joss_db_wire.inc native/joss_crypto_rt.c native/joss_crypto_primitives.inc native/joss_gui_rt.c
var NativeFS embed.FS

var embeddedNativeFiles = []string{
	"joss_rt.h",
	"joss_rt.c",
	"joss_web_rt.c",
	"joss_concurrency_rt.c",
	"joss_db_rt.c",
	"joss_db_wire.inc",
	"joss_crypto_rt.c",
	"joss_crypto_primitives.inc",
	"joss_gui_rt.c",
}

// ExtractNativeRuntime materializa los archivos del runtime C11 embebidos en targetDir
// y devuelve la ruta absoluta a joss_rt.c.
func ExtractNativeRuntime(targetDir string) (string, error) {
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", fmt.Errorf("runtimeassets: crear directorio %s: %w", targetDir, err)
	}
	for _, name := range embeddedNativeFiles {
		data, err := NativeFS.ReadFile("native/" + name)
		if err != nil {
			return "", fmt.Errorf("runtimeassets: leer native/%s embebido: %w", name, err)
		}
		dst := filepath.Join(targetDir, name)
		if existing, err := os.ReadFile(dst); err == nil && len(existing) == len(data) && string(existing) == string(data) {
			continue
		}
		if err := os.WriteFile(dst, data, 0644); err != nil {
			return "", fmt.Errorf("runtimeassets: escribir %s: %w", dst, err)
		}
	}
	return filepath.Join(targetDir, "joss_rt.c"), nil
}
