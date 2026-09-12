package pluginpkg

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
)

func TestSignedArchiveVerificationAndTamperDetection(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	metadata := Metadata{
		Name:     "vendor/demo",
		Version:  "1.0.0",
		Bytecode: "bytecode/main.jbc",
		Symbols:  SymbolsPath,
	}
	files := map[string][]byte{
		"bytecode/main.jbc": []byte("compiled bytecode binary"),
		SymbolsPath:         []byte(`{"schema":1,"package":"vendor/demo"}`),
		"joss.yaml":         []byte("name: vendor/demo\nversion: 1.0.0\n"),
	}
	data, err := BuildSigned(metadata, files, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	archive, err := ReadVerified(data)
	if err != nil {
		t.Fatal(err)
	}
	if archive.Metadata.KeyID == "" || archive.Metadata.Signature == "" {
		t.Fatal("signed archive has incomplete signature metadata")
	}

	// 1. Tamper Bytecode
	tamperedBytecode := make(map[string][]byte)
	for k, v := range archive.Files {
		tamperedBytecode[k] = append([]byte(nil), v...)
	}
	tamperedBytecode["bytecode/main.jbc"][0] ^= 0xff
	if err := verifySignature(archive.Metadata, tamperedBytecode); err == nil {
		t.Fatal("tampered bytecode was accepted")
	}

	// 2. Tamper Symbols
	tamperedSymbols := make(map[string][]byte)
	for k, v := range archive.Files {
		tamperedSymbols[k] = append([]byte(nil), v...)
	}
	tamperedSymbols[SymbolsPath] = []byte(`{"schema":1,"package":"malicious"}`)
	if err := verifySignature(archive.Metadata, tamperedSymbols); err == nil {
		t.Fatal("tampered symbols were accepted")
	}

	// 3. Tamper Manifest
	tamperedManifest := make(map[string][]byte)
	for k, v := range archive.Files {
		tamperedManifest[k] = append([]byte(nil), v...)
	}
	tamperedManifest["joss.yaml"] = []byte("name: altered\n")
	if err := verifySignature(archive.Metadata, tamperedManifest); err == nil {
		t.Fatal("tampered manifest was accepted")
	}

	// 4. Inject unexpected file
	injectedFiles := make(map[string][]byte)
	for k, v := range archive.Files {
		injectedFiles[k] = append([]byte(nil), v...)
	}
	injectedFiles["payload.exe"] = []byte("malicious content")
	if err := verifySignature(archive.Metadata, injectedFiles); err == nil {
		t.Fatal("injected unexpected file was accepted by signature verification")
	}
}

func TestBuildSignedDeterminism(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	metadata := Metadata{
		Name:     "test/deterministic",
		Version:  "2.1.0",
		Bytecode: "bytecode/main.jbc",
		Symbols:  SymbolsPath,
		Exports:  []string{"fnA", "fnB"},
	}
	files := map[string][]byte{
		"bytecode/main.jbc": []byte("deterministic content A"),
		SymbolsPath:         []byte(`{"schema":1}`),
		"joss.yaml":         []byte("name: test/deterministic\nversion: 2.1.0\n"),
		"README.md":         []byte("# Deterministic Test"),
	}

	build1, err := BuildSigned(metadata, files, privateKey)
	if err != nil {
		t.Fatalf("build 1 fallo: %v", err)
	}

	build2, err := BuildSigned(metadata, files, privateKey)
	if err != nil {
		t.Fatalf("build 2 fallo: %v", err)
	}

	if !bytes.Equal(build1, build2) {
		hash1 := sha256.Sum256(build1)
		hash2 := sha256.Sum256(build2)
		t.Fatalf("BuildSigned no es determinista: SHA256 build1 (%x) != SHA256 build2 (%x)", hash1, hash2)
	}
}

func TestVerifiedReaderRejectsUnsignedArchive(t *testing.T) {
	data, err := Build(Metadata{Name: "vendor/demo", Version: "1.0.0", Bytecode: "main.jbc"}, map[string][]byte{"main.jbc": []byte("x")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReadVerified(data); err == nil {
		t.Fatal("unsigned archive was accepted")
	}
}

func TestLoadOrCreateSigningKey(t *testing.T) {
	// Isolate the platform home used by os.UserHomeDir. The production default
	// remains ~/.joss/keys, while tests never touch the developer's real home.
	testHome := t.TempDir()
	t.Setenv("USERPROFILE", testHome)
	t.Setenv("HOME", testHome)

	// 1. Default creation: generate key in the isolated ~/.joss/keys/
	pluginName := "test_unit_key_unique"
	key1, path1, err := LoadOrCreateSigningKey(pluginName)
	if err != nil {
		t.Fatalf("error creando llave por defecto: %v", err)
	}
	defer os.Remove(path1)

	// Segundo llamado: debe recargar exactamente la misma llave
	key2, path2, err := LoadOrCreateSigningKey(pluginName)
	if err != nil {
		t.Fatalf("error recargando llave: %v", err)
	}
	if path2 != path1 {
		t.Errorf("ruta esperada %s, obtenida %s", path1, path2)
	}
	if !bytes.Equal(key1, key2) {
		t.Fatal("las llaves generada y recargada no son identicas")
	}

	// 2. Configured key via JOSS_PLUGIN_SIGNING_KEY
	tempDir := t.TempDir()
	customKeyPath := filepath.Join(tempDir, "custom.ed25519")
	_, privKey, _ := ed25519.GenerateKey(rand.Reader)
	_ = os.WriteFile(customKeyPath, privKey, 0600)

	t.Setenv("JOSS_PLUGIN_SIGNING_KEY", customKeyPath)
	customLoaded, customPath, err := LoadOrCreateSigningKey(pluginName)
	if err != nil {
		t.Fatalf("error cargando llave configurada: %v", err)
	}
	if customPath != customKeyPath {
		t.Errorf("ruta configurada esperada %s, obtenida %s", customKeyPath, customPath)
	}
	if !bytes.Equal(customLoaded, privKey) {
		t.Fatal("la llave cargada no coincide con la llave personalizada")
	}

	// 3. Corromper el archivo de llave y verificar que se detecte el error
	_ = os.WriteFile(customKeyPath, []byte("clave_invalida_y_corta"), 0600)
	_, _, err = LoadOrCreateSigningKey(pluginName)
	if err == nil {
		t.Fatal("se esperaba error al cargar llave corrupta")
	}
}
