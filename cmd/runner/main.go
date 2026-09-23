package main

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	semanticanalyzer "github.com/jossecurity/joss/pkg/analyzer"
	"github.com/jossecurity/joss/pkg/bytecode"
	"github.com/jossecurity/joss/pkg/core"
	"github.com/jossecurity/joss/pkg/crypto"
	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/pkg/server"
	"github.com/jossecurity/joss/pkg/vfs"
)

const MagicMarker = "JOSS_RUNNER_DATA"

func main() {
	setupLogging()

	// 1. Read Assets from Tail
	exePath, err := os.Executable()
	if err != nil {
		log.Fatalf("Error getting executable path: %v", err)
	}

	// Fix: Change CWD to executable directory to ensure relative paths (like Storage/database.sqlite) work
	exeDir := filepath.Dir(exePath)
	if err := os.Chdir(exeDir); err != nil {
		log.Printf("Warning: Could not change CWD to %s: %v", exeDir, err)
	}

	f, err := os.Open(exePath)
	if err != nil {
		log.Fatalf("Error opening executable: %v", err)
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		log.Fatalf("Error getting file stat: %v", err)
	}
	fileSize := stat.Size()

	// Read Magic Marker (16 bytes)
	if fileSize < 16 {
		log.Fatal("Invalid executable size")
	}
	marker := make([]byte, 16)
	f.ReadAt(marker, fileSize-16)

	if string(marker) != MagicMarker {
		log.Fatal("Error: Corrupted or invalid runner binary (Magic Marker not found)")
	}

	// Read Assets Length (8 bytes)
	// Layout: [Data] [Key 32] [Len 8] [Magic 16]
	lenBuf := make([]byte, 8)
	f.ReadAt(lenBuf, fileSize-16-8)
	var assetsLen int64
	binary.Read(bytes.NewReader(lenBuf), binary.LittleEndian, &assetsLen)

	// Read Key (32 bytes)
	key := make([]byte, 32)
	f.ReadAt(key, fileSize-16-8-32)

	// Read Encrypted Assets
	encryptedAssets := make([]byte, assetsLen)
	f.ReadAt(encryptedAssets, fileSize-16-8-32-assetsLen)

	// 2. Decrypt Assets
	decryptedData, err := crypto.DecryptAES(encryptedAssets, key)
	if err != nil {
		log.Fatalf("Error decrypting assets: %v", err)
	}

	// 3. Hydrate VFS
	var files map[string][]byte
	decDecoder := gob.NewDecoder(bytes.NewReader(decryptedData))
	if err := decDecoder.Decode(&files); err != nil {
		log.Fatalf("Error decoding assets: %v", err)
	}

	memFS := vfs.NewMemFS()
	memFS.Files = files

	// 4. Handle Arguments (Support for self-spawned "server start")
	if len(os.Args) >= 3 && os.Args[1] == "server" && os.Args[2] == "start" {
		server.Start(memFS)
		return
	}

	// 5. Normal Startup: Execute main.joss and Open WebView
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Recovered from panic in runtime: %v", r)
			}
		}()

		// Set Global FileSystem for Server::start() native calls
		server.GlobalFileSystem = memFS
		core.SetFileSystem(memFS)

		units, prepareErr := packagedSourceUnits(files)
		if prepareErr != nil {
			log.Printf("Program preparation failed: %v", prepareErr)
			return
		}
		report := core.AnalyzeSourceUnits(units)
		if report.HasIssues() {
			report.PrintReport()
		}
		if report.HasErrors() {
			log.Printf("Program preparation rejected the packaged application")
			return
		}

		r := core.NewRuntime()
		defer r.Free()
		r.LoadEnv(memFS)
		for _, unit := range report.Prepared.Units[1:] {
			if strings.HasPrefix(filepath.ToSlash(unit.Path), "app/") {
				r.Execute(unit.Program)
			}
		}
		if program := report.Prepared.Entrypoint(); program != nil {
			r.Execute(program)
		} else {
			// Fallback: Start server directly
			server.Start(memFS)
		}
	}()

	// Determine port from env (loaded from VFS or defaults)
	port := os.Getenv("PORT")
	if port == "" {
		port = os.Getenv("JOSS_PORT")
	}

	var envData []byte
	if data, ok := files["env.joss"]; ok {
		envData = data
	} else if data, ok := files[".env"]; ok {
		envData = data
	} else if envEnc, ok := files["env.enc"]; ok {
		if len(envEnc) > 16 {
			salt := envEnc[:16]
			ciphertext := envEnc[16:]
			masterSecret := []byte("JOSSECURITY_MASTER_SECRET_2025")
			key := crypto.DeriveKey(masterSecret, salt)
			decrypted, err := crypto.DecryptAES(ciphertext, key)
			if err == nil {
				envData = decrypted
			}
		}
	}

	if port == "" && len(envData) > 0 {
		lines := bytes.Split(envData, []byte("\n"))
		for _, line := range lines {
			s := bytes.TrimSpace(line)
			if bytes.HasPrefix(s, []byte("#")) {
				continue
			}
			parts := bytes.SplitN(s, []byte("="), 2)
			if len(parts) == 2 {
				key := string(bytes.ToUpper(bytes.TrimSpace(parts[0])))
				if key == "PORT" || key == "JOSS_PORT" || key == "APP_PORT" || key == "SERVER_PORT" {
					val := bytes.TrimSpace(parts[1])
					val = bytes.Trim(val, "\"")
					val = bytes.Trim(val, "'")
					port = string(val)
					break
				}
			}
		}
	}

	if port == "" {
		port = "8000"
	}

	// Wait for resolved port, or fallback to default
	finalPort := waitForPortOrPort(port, "8000")
	runGUIOrWait(finalPort)
}

func packagedSourceUnits(files map[string][]byte) ([]semanticanalyzer.SourceUnit, error) {
	normalizedFiles := make(map[string][]byte, len(files))
	for name, data := range files {
		normalizedFiles[canonicalPackagedPath(name)] = data
	}
	mainData, exists := normalizedFiles["main.joss"]
	if !exists {
		return nil, fmt.Errorf("main.joss is missing")
	}
	paths := make([]string, 0, len(files))
	for name := range normalizedFiles {
		normalized := canonicalPackagedPath(name)
		if normalized == "main.joss" || !parser.IsJossSourceFile(normalized) {
			continue
		}
		if strings.HasPrefix(normalized, "app/") || normalized == "routes.joss" || normalized == "api.joss" || normalized == "config/cron.joss" {
			paths = append(paths, normalized)
		}
	}
	sort.Strings(paths)
	ordered := append([]string{"main.joss"}, paths...)
	units := make([]semanticanalyzer.SourceUnit, 0, len(ordered))
	for _, name := range ordered {
		data := normalizedFiles[name]
		if name == "main.joss" {
			data = mainData
		}
		var program *parser.Program
		if bytecode.IsBytecode(data) {
			decoded, err := bytecode.Decode(data)
			if err != nil {
				return nil, fmt.Errorf("decode %s: %w", name, err)
			}
			program = decoded
		} else {
			p := parser.NewParser(parser.NewLexer(string(data)))
			program = p.ParseProgram()
			if len(p.Diagnostics()) > 0 {
				return nil, fmt.Errorf("parse %s: %s", name, p.Diagnostics()[0].Message)
			}
		}
		units = append(units, semanticanalyzer.SourceUnit{Path: name, Program: program})
	}
	return units, nil
}

func canonicalPackagedPath(name string) string {
	return path.Clean(strings.ReplaceAll(name, `\`, "/"))
}

func waitForSignal() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan
	log.Println("[Joss Runner] Finalizando proceso...")
}

func setupLogging() {
	// Keep standard output unless running in hidden GUI mode
}

func waitForPortOrPort(p1, p2 string) string {
	target := ""
	for i := 0; i < 60; i++ { // 12 seconds
		if checkPort(p1) {
			target = p1
			break
		}
		if checkPort(p2) {
			target = p2
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if target == "" {
		return p1 // Default to first
	}
	return target
}

func checkPort(port string) bool {
	conn, err := net.DialTimeout("tcp", "localhost:"+port, 100*time.Millisecond)
	if err == nil {
		conn.Close()
		return true
	}
	return false
}
