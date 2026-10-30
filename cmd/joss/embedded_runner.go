package main

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/jossecurity/joss/pkg/core"
	"github.com/jossecurity/joss/pkg/crypto"
	"github.com/jossecurity/joss/pkg/server"
	"github.com/jossecurity/joss/pkg/vfs"
)

const runnerTrailerMagic = "JOSS_RUNNER_DATA"

// runEmbeddedRunnerIfSealed comprueba si el ejecutable actual tiene datos VFS sellados
// al final (trailer JOSS_RUNNER_DATA). Si los tiene, monta el VFS en memoria y arranca
// directamente el servidor HTTP sin interpretar argumentos CLI.
func runEmbeddedRunnerIfSealed() bool {
	exePath, err := os.Executable()
	if err != nil {
		return false
	}

	f, err := os.Open(exePath)
	if err != nil {
		return false
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return false
	}
	fileSize := stat.Size()
	// Mínimo: magic (16) + len (8) + key (32) = 56 bytes
	if fileSize < 56 {
		return false
	}

	marker := make([]byte, 16)
	if _, err := f.ReadAt(marker, fileSize-16); err != nil {
		return false
	}
	if string(marker) != runnerTrailerMagic {
		return false
	}

	lenBuf := make([]byte, 8)
	if _, err := f.ReadAt(lenBuf, fileSize-16-8); err != nil {
		return false
	}
	var assetsLen int64
	if err := binary.Read(bytes.NewReader(lenBuf), binary.LittleEndian, &assetsLen); err != nil {
		return false
	}
	if assetsLen <= 0 || fileSize < 16+8+32+assetsLen {
		return false
	}

	key := make([]byte, 32)
	if _, err := f.ReadAt(key, fileSize-16-8-32); err != nil {
		return false
	}

	encryptedAssets := make([]byte, assetsLen)
	if _, err := f.ReadAt(encryptedAssets, fileSize-16-8-32-assetsLen); err != nil {
		return false
	}

	decryptedData, err := crypto.DecryptAES(encryptedAssets, key)
	if err != nil {
		fmt.Printf("[Sealed Runner] Error desencriptando assets sellados: %v\n", err)
		return false
	}

	var files map[string][]byte
	var decReader io.Reader = bytes.NewReader(decryptedData)
	if len(decryptedData) >= 2 && decryptedData[0] == 0x1f && decryptedData[1] == 0x8b {
		zr, gzErr := gzip.NewReader(bytes.NewReader(decryptedData))
		if gzErr != nil {
			fmt.Printf("[Sealed Runner] Error descomprimiendo assets: %v\n", gzErr)
			return false
		}
		defer zr.Close()
		decReader = zr
	}
	if err := gob.NewDecoder(decReader).Decode(&files); err != nil {
		fmt.Printf("[Sealed Runner] Error decodificando VFS embebido: %v\n", err)
		return false
	}

	exeDir := filepath.Dir(exePath)
	_ = os.Chdir(exeDir)

	mem := vfs.NewMemFS()
	mem.Files = files
	core.SetFileSystem(mem)
	server.GlobalFileSystem = mem

	fmt.Println("[Joss Standalone Runner] Iniciando aplicación web sellada...")
	server.Start(mem)
	os.Exit(0)
	return true
}
