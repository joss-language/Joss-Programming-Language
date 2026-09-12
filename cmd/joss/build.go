package main

import (
	"bufio"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	_ "embed"

	"github.com/jossecurity/joss/pkg/bytecode"
	"github.com/jossecurity/joss/pkg/crypto"
	"github.com/jossecurity/joss/pkg/i18n"
	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/pkg/pluginpkg"
)

func buildWeb() {
	fmt.Println(i18n.T("buildWebStart"))

	// 1. Validate Structure (Strict Topology)
	required := []string{
		"main.joss",
		// "env.joss", // Handled dynamically
		"app",
		"config",
		"api.joss",
		"routes.joss",
	}
	for _, f := range required {
		if _, err := os.Stat(f); os.IsNotExist(err) {
			fmt.Println(i18n.T("buildMissingRequired", map[string]any{"file": f}))
			fmt.Println(i18n.T("buildStrictStructure"))
			return
		}
	}

	// Check for environment file (env.joss OR .env)
	if _, err := os.Stat(GetEnvFile()); os.IsNotExist(err) {
		fmt.Println(i18n.T("buildMissingEnv"))
		return
	}

	// 2. Prepare Build Directory
	buildDir := "build"
	fmt.Println(i18n.T("buildCreatingDir", map[string]any{"dir": buildDir}))
	os.RemoveAll(buildDir)
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		fmt.Printf("Error creando directorio build: %v\n", err)
		return
	}

	// 3. Copy Project Files
	fmt.Println(i18n.T("buildCopyingFiles"))

	// Default ignore list
	ignoredDirs := map[string]bool{
		".git":         true,
		".vscode":      true,
		".idea":        true,
		"build":        true,
		"vendor":       true,
		"node_modules": true, // Handled separately
		".gemini":      true, // Agent artifacts
		"storage":      true, // Usually link, handled separately? Or copy structure? User said "anexe todas".
	}

	// Check for node_modules inclusion
	includeNodeModules := false
	if _, err := os.Stat("node_modules"); err == nil {
		fmt.Print(i18n.T("buildPromptNodeModules"))
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.ToLower(strings.TrimSpace(response))

		if response == "y" || response == "s" || response == "si" || response == "yes" {
			includeNodeModules = true
			fmt.Println(i18n.T("buildIncludeNodeModules"))
		} else {
			fmt.Println(i18n.T("buildOmitNodeModules"))
		}
	}

	// Dynamic copy of root directories
	files, err := os.ReadDir(".")
	if err == nil {
		for _, f := range files {
			name := f.Name()
			if f.IsDir() {
				if ignoredDirs[name] {
					continue
				}
				// Copy Directory
				if _, err := os.Stat(name); err == nil {
					copyDir(name, filepath.Join(buildDir, name))
				}
			} else {
				// Copy Files (only specific extensions or all?)
				// The previous code copied specific root files.
				// User said "anexe todas las carpetas". He didn't specify files, but implied "everything".
				// Let's copy all root files except specific ignores
				if name == "joss.exe" || strings.HasSuffix(name, ".log") || strings.HasSuffix(name, ".enc") {
					continue
				}
				copyFile(name, filepath.Join(buildDir, name))
			}
		}
	}

	// Handle node_modules if requested
	if includeNodeModules {
		copyDir("node_modules", filepath.Join(buildDir, "node_modules"))
	}

	// 4. Copy Database and WAL files
	if _, err := os.Stat("database.sqlite"); err == nil {
		copyFile("database.sqlite", filepath.Join(buildDir, "database.sqlite"))
		fmt.Println(i18n.T("buildDbCopied"))

		// Copy WAL files if they exist
		if _, err := os.Stat("database.sqlite-shm"); err == nil {
			copyFile("database.sqlite-shm", filepath.Join(buildDir, "database.sqlite-shm"))
		}
		if _, err := os.Stat("database.sqlite-wal"); err == nil {
			copyFile("database.sqlite-wal", filepath.Join(buildDir, "database.sqlite-wal"))
		}
	}

	// 4. Create nginx_port.conf
	envFile := GetEnvFile()
	port := getEnvPort(envFile)
	if port == "" {
		port = "8000"
	}

	nginxContent := fmt.Sprintf("set $joss_port %s;", port)
	if err := os.WriteFile(filepath.Join(buildDir, "nginx_port.conf"), []byte(nginxContent), 0644); err != nil {
		fmt.Printf("Error creando nginx_port.conf: %v\n", err)
	} else {
		fmt.Println(i18n.T("buildNginxPortCreated", map[string]any{"port": port}))
	}

	// 5. Encrypt env.joss to build/env.enc
	fmt.Println(i18n.T("buildEncryptingEnv"))
	encryptEnvTo(filepath.Join(buildDir, "env.enc"))

	fmt.Println(i18n.T("buildWebSuccess"))
	fmt.Println(i18n.T("buildDeployTip"))
	fmt.Println(i18n.T("buildRunTip"))
}

func buildProgram() {
	fmt.Println(i18n.T("buildProgramStart"))

	fmt.Println(i18n.T("buildSelectOS"))
	fmt.Println(i18n.T("buildOSWindows"))
	fmt.Println(i18n.T("buildOSLinux"))
	fmt.Println(i18n.T("buildOSMacArm"))
	fmt.Println(i18n.T("buildOSMacIntel"))
	fmt.Println(i18n.T("buildOSCurrent", map[string]any{"os": runtime.GOOS, "arch": runtime.GOARCH}))
	fmt.Print(i18n.T("buildPromptOption"))

	reader := bufio.NewReader(os.Stdin)
	option, _ := reader.ReadString('\n')
	option = strings.TrimSpace(option)

	switch option {
	case "1", "windows":
		buildNative("windows", "amd64", false)
	case "2", "linux":
		buildNative("linux", "amd64", false)
	case "3", "mac", "macos", "darwin-arm64":
		buildNative("darwin", "arm64", false)
	case "4", "darwin-amd64":
		buildNative("darwin", "amd64", false)
	case "5", "current", "":
		buildNative(runtime.GOOS, runtime.GOARCH, false)
	default:
		fmt.Println(i18n.T("buildInvalidOption"))
	}
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func copyDir(src string, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, _ := filepath.Rel(src, path)
		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}
		return copyFile(path, dstPath)
	})
}

func encryptEnvTo(destPath string) {
	envPath := GetEnvFile()
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		fmt.Printf("Error: No se encontró archivo de entorno ('env.joss' o '.env') para encriptar.\n")
		return
	}

	data, err := os.ReadFile(envPath)
	if err != nil {
		fmt.Printf("Error leyendo %s: %v\n", envPath, err)
		return
	}

	// Generate a random salt
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		fmt.Printf("Error generando salt: %v\n", err)
		return
	}

	// Derive key (In a real scenario, this key needs to be shared securely or embedded in a way the runtime can recover it,
	// but for this "Gran Biblia" spec, the runtime generates a master key in RAM.
	// However, to decrypt, the runtime needs the SAME key used here.
	// The spec says: "Runtime: Al ejecutar main.joss, el motor genera una llave maestra efímera en RAM para desencriptar el entorno".
	// This implies the key is either derived from something constant or embedded.
	// For simplicity and to match the "embedded in build" concept, we will generate a key, encrypt the env,
	// and then we need to decide how the runtime gets it.
	// The spec says: "Encriptador de Entorno: Toma env.joss ... genera una sal ... y lo cifra ... El resultado se incrusta en el build."
	// And "Runtime ... genera una llave maestra efímera ... para desencriptar".
	// This is slightly contradictory if the key is ephemeral and random.
	// Let's assume the "llave maestra" is derived from a hardcoded secret in the engine + the salt,
	// or the key is stored in the build but obfuscated.
	// Let's use a fixed internal secret for now to allow the runtime to decrypt it,
	// as the runtime needs to know how to decrypt it without user input.

	masterSecret := []byte("JOSSECURITY_MASTER_SECRET_2025") // Internal Engine Secret
	key := crypto.DeriveKey(masterSecret, salt)

	encrypted, err := crypto.EncryptAES(data, key)
	if err != nil {
		fmt.Printf("Error encriptando env: %v\n", err)
		return
	}

	// Format: [Salt 16] [Encrypted Data]
	finalData := append(salt, encrypted...)

	err = os.WriteFile(destPath, finalData, 0644)
	if err != nil {
		fmt.Printf("Error escribiendo %s: %v\n", destPath, err)
		return
	}
	fmt.Println(i18n.T("buildEnvEncryptedSaved", map[string]any{"path": destPath}))
}

func getEnvPort(envPath string) string {
	content, err := os.ReadFile(envPath)
	if err != nil {
		return ""
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Handle "PORT=..." and "PORT = ..."
		if strings.HasPrefix(line, "#") {
			continue
		}

		upper := strings.ToUpper(line)
		if strings.HasPrefix(upper, "PORT") || strings.HasPrefix(upper, "JOSS_PORT") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				// Verify key is exactly PORT or JOSS_PORT (ignoring case/space)
				key := strings.TrimSpace(strings.ToUpper(parts[0]))
				if key == "PORT" || key == "JOSS_PORT" {
					val := strings.TrimSpace(parts[1])
					val = strings.Trim(val, "\"")
					val = strings.Trim(val, "'")
					return val
				}
			}
		}
	}
	return ""
}

func buildPackage(pkgPath string) {
	fmt.Println(i18n.T("pkgBuildStarting", map[string]any{"path": pkgPath}))

	// Validate path exists
	info, err := os.Stat(pkgPath)
	if err != nil || !info.IsDir() {
		fmt.Println(i18n.T("pkgBuildInvalidDir", map[string]any{"path": pkgPath}))
		return
	}

	// Read joss.yaml manifest first to check package validity
	manifestPath := filepath.Join(pkgPath, "joss.yaml")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		fmt.Println(i18n.T("pkgBuildMissingYaml", map[string]any{"path": pkgPath}))
		return
	}
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		fmt.Printf("Error: No se pudo leer joss.yaml: %v\n", err)
		return
	}
	pluginType := packageManifestValue(string(manifestData), "type", "")
	if strings.EqualFold(pluginType, "go_extension") {
		fmt.Println("Error: type=go_extension no produce un plugin dinámico multiplataforma.")
		fmt.Println("Use type: joss y declare entry.main (por defecto src/plugin.joss).")
		return
	}
	entry := packageManifestValue(string(manifestData), "entry", "main")
	if entry == "" {
		entry = "src/plugin.joss"
	}
	cleanEntry := filepath.Clean(filepath.FromSlash(entry))
	if cleanEntry == ".." || filepath.IsAbs(cleanEntry) || strings.HasPrefix(cleanEntry, ".."+string(filepath.Separator)) {
		fmt.Printf("Error: entry.main sale del paquete: %s\n", entry)
		return
	}
	program, err := compilePluginProgram(pkgPath, filepath.Join(pkgPath, cleanEntry))
	if err != nil {
		fmt.Printf("Error compilando entry.main '%s': %v\n", entry, err)
		return
	}
	compiled, err := bytecode.Encode(program)
	if err != nil {
		fmt.Printf("Error generando bytecode: %v\n", err)
		return
	}

	name := packageManifestValue(string(manifestData), "name", "")
	versionValue := packageManifestValue(string(manifestData), "version", "")
	symbolData, err := json.MarshalIndent(pluginpkg.BuildSymbolIndex(program, name, versionValue), "", "  ")
	if err != nil {
		fmt.Printf("Error generando indice de simbolos: %v\n", err)
		return
	}

	files := map[string][]byte{
		"joss.yaml":           manifestData,
		"bytecode/main.jbc":   compiled,
		pluginpkg.SymbolsPath: symbolData,
	}
	nativeConfig := packageManifestSection(string(manifestData), "native")
	abiConfig := packageManifestSection(string(manifestData), "abi")
	protocol := nativeConfig["protocol"]
	delete(nativeConfig, "protocol")
	if len(nativeConfig) > 0 && protocol == "" {
		protocol = "joss-rpc-v1"
	}

	// Include ONLY bytecode, symbols, joss.yaml, and small non-binary assets.
	// JP v2 packages contain deterministically compiled pure bytecode (main.jbc).
	err = filepath.Walk(pkgPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// Skip .git folders to avoid packing credentials
		if strings.Contains(path, "/.git/") || strings.Contains(path, "\\.git\\") || strings.HasSuffix(path, ".git") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip native/ folder if present — packages execute bytecode internally
		relCheck, _ := filepath.Rel(pkgPath, path)
		relCheckSlash := filepath.ToSlash(relCheck)
		if relCheckSlash == "native" || strings.HasPrefix(relCheckSlash, "native/") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".jp" || ext == ".go" || isPluginSourceExtension(ext) || info.Name() == "env.joss" || info.Name() == "env.enc" {
			return nil
		}
		// Incluir exclusivamente manifiesto, licencias, README, bytecode y metadatos del plugin
		if ext == ".exe" || ext == ".dll" || ext == ".so" || ext == ".dylib" {
			return nil
		}

		if info.IsDir() {
			return nil
		}

		// Skip fuente original .joss o archivos de desarrollo (.github, src, etc.)
		relPath, err := filepath.Rel(pkgPath, path)
		if err != nil {
			return nil
		}
		relSlash := filepath.ToSlash(relPath)
		if strings.HasPrefix(relSlash, ".github/") || strings.HasPrefix(relSlash, "src/") || ext == ".joss" || ext == ".go" {
			return nil
		}

		// joss.yaml fue procesado arriba.
		if filepath.Clean(path) == filepath.Clean(manifestPath) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err == nil {
			files[relSlash] = data
		}
		return nil
	})

	if err != nil {
		fmt.Printf("Error leyendo archivos del paquete: %v\n", err)
		return
	}

	metadata := pluginpkg.Metadata{
		Name:         name,
		Version:      versionValue,
		Bytecode:     "bytecode/main.jbc",
		Dependencies: parseManifestDependencies(string(manifestData)),
		Native:       nativeConfig,
		ABI:          abiConfig,
		Protocol:     protocol,
		Symbols:      pluginpkg.SymbolsPath,
	}
	signingKey, signingKeyPath, err := pluginpkg.LoadOrCreateSigningKey(name)
	if err != nil {
		fmt.Printf("Error preparando firma del plugin: %v\n", err)
		return
	}
	archive, err := pluginpkg.BuildSigned(metadata, files, signingKey)
	if err != nil {
		fmt.Printf("Error creando JP v2: %v\n", err)
		return
	}

	pkgName := name
	if pkgName == "" {
		pkgName = filepath.Base(pkgPath)
	}
	outPath := filepath.Join(pkgPath, pkgName+".jp")

	if err := os.WriteFile(outPath, archive, 0644); err != nil {
		fmt.Printf("Error al escribir el archivo compilado del paquete: %v\n", err)
		return
	}

	fmt.Println(i18n.T("pkgBuildSuccess", map[string]any{"path": outPath}))
	fmt.Printf("[Package Build] Llave de autor: %s (no se incluye en el JP)\n", signingKeyPath)
}

func isPluginSourceExtension(ext string) bool {
	switch ext {
	case ".joss", ".go", ".c", ".cc", ".cpp", ".cxx", ".h", ".hpp", ".py", ".pyw", ".php", ".phtml", ".m", ".mm", ".java", ".kt", ".kts", ".dart", ".cs", ".rs", ".swift":
		return true
	default:
		return false
	}
}

func packageManifestValue(content, section, key string) string {
	activeSection := ""
	for _, raw := range strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n") {
		line := strings.TrimRight(raw, " \t")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) != 2 {
			continue
		}
		lineKey := strings.TrimSpace(parts[0])
		value := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
		if indent == 0 {
			activeSection = lineKey
			if key == "" && lineKey == section {
				return value
			}
			continue
		}
		if activeSection == section && lineKey == key {
			return value
		}
	}
	return ""
}

func inspectPackage(filename string) {
	data, err := os.ReadFile(filename)
	if err != nil {
		fmt.Printf("Error leyendo JP: %v\n", err)
		return
	}
	if !pluginpkg.IsV2(data) {
		fmt.Println(i18n.T("pkgInspectJpV1Deprecated"))
		return
	}
	archive, err := pluginpkg.Read(data)
	if err != nil {
		fmt.Printf("JP v2 inválido: %v\n", err)
		return
	}
	fmt.Printf("JP v2 %s %s\n", archive.Metadata.Name, archive.Metadata.Version)
	if archive.Metadata.Signature != "" {
		fmt.Printf("Firma: %s (%s) verificada\n", archive.Metadata.SignatureAlgorithm, archive.Metadata.KeyID)
	} else {
		fmt.Println("Firma: ausente; el runtime rechazará este paquete")
	}
	fmt.Printf("Bytecode: %s (%d bytes)\n", archive.Metadata.Bytecode, len(archive.Files[archive.Metadata.Bytecode]))
	if archive.Metadata.Symbols != "" {
		var symbols pluginpkg.SymbolIndex
		if err := json.Unmarshal(archive.Files[archive.Metadata.Symbols], &symbols); err == nil {
			methodCount := 0
			for _, class := range symbols.Classes {
				methodCount += len(class.Methods)
			}
			fmt.Printf("IntelliSense: %s (%d clases, %d metodos, %d funciones)\n", archive.Metadata.Symbols, len(symbols.Classes), methodCount, len(symbols.Functions))
		}
	}
	if len(archive.Metadata.Native) == 0 && len(archive.Metadata.ABI) == 0 {
		fmt.Println("Payloads nativos: ninguno")
	} else if len(archive.Metadata.Native) > 0 {
		fmt.Printf("Protocolo: %s\n", archive.Metadata.Protocol)
		fmt.Println("Payloads nativos:")
		for _, target := range sortedManifestKeys(archive.Metadata.Native) {
			asset := archive.Metadata.Native[target]
			fmt.Printf("  %s -> %s (%d bytes)\n", target, asset, len(archive.Files[asset]))
		}
	}
	if len(archive.Metadata.ABI) > 0 {
		fmt.Println("Bibliotecas ABI C v1:")
		for _, target := range sortedManifestKeys(archive.Metadata.ABI) {
			asset := archive.Metadata.ABI[target]
			fmt.Printf("  %s -> %s (%d bytes)\n", target, asset, len(archive.Files[asset]))
		}
	}
	fmt.Printf("Archivos internos: %d\n", len(archive.Files))
}

func sortedManifestKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func packageManifestSection(content, section string) map[string]string {
	values := make(map[string]string)
	active := false
	for _, raw := range strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n") {
		line := strings.TrimRight(raw, " \t")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		if indent == 0 {
			active = trimmed == section+":"
			continue
		}
		if !active {
			continue
		}
		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) == 2 {
			values[strings.TrimSpace(parts[0])] = strings.Trim(strings.TrimSpace(parts[1]), "\"'")
		}
	}
	return values
}

func compilePluginProgram(root, filename string) (*parser.Program, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	absFile, err := filepath.Abs(filename)
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(absRoot, absFile)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("archivo de entrada fuera del paquete: %s", filename)
	}
	data, err := os.ReadFile(absFile)
	if err != nil {
		return nil, err
	}
	p := parser.NewParser(parser.NewLexer(string(data)))
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		return nil, fmt.Errorf("%s: %s", filepath.ToSlash(rel), strings.Join(errs, "; "))
	}
	return program, nil
}
