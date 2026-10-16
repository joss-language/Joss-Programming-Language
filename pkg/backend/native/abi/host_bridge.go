package main

import "C"

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/jossecurity/joss/pkg/core"
	"github.com/jossecurity/joss/pkg/crypto"
	"github.com/jossecurity/joss/pkg/server"
	"github.com/jossecurity/joss/pkg/vfs"
)

const trailerMagic = "JOSS_RUNNER_DATA"

var (
	vfsInitOnce sync.Once
	embeddedVFS *vfs.MemFS
)

func loadEmbeddedVFS() *vfs.MemFS {
	vfsInitOnce.Do(func() {
		exePath, err := os.Executable()
		if err != nil {
			return
		}
		exeDir := filepath.Dir(exePath)
		_ = os.Chdir(exeDir)

		f, err := os.Open(exePath)
		if err != nil {
			return
		}
		defer f.Close()

		stat, err := f.Stat()
		if err != nil {
			return
		}
		fileSize := stat.Size()
		if fileSize < 16+8+32 {
			return
		}

		marker := make([]byte, 16)
		if _, err := f.ReadAt(marker, fileSize-16); err != nil {
			return
		}
		if string(marker) != trailerMagic {
			return
		}

		lenBuf := make([]byte, 8)
		if _, err := f.ReadAt(lenBuf, fileSize-16-8); err != nil {
			return
		}
		var assetsLen int64
		if err := binary.Read(bytes.NewReader(lenBuf), binary.LittleEndian, &assetsLen); err != nil {
			return
		}
		if assetsLen <= 0 || fileSize < 16+8+32+assetsLen {
			return
		}

		key := make([]byte, 32)
		if _, err := f.ReadAt(key, fileSize-16-8-32); err != nil {
			return
		}

		encryptedAssets := make([]byte, assetsLen)
		if _, err := f.ReadAt(encryptedAssets, fileSize-16-8-32-assetsLen); err != nil {
			return
		}

		decryptedData, err := crypto.DecryptAES(encryptedAssets, key)
		if err != nil {
			return
		}

		var files map[string][]byte
		var decReader io.Reader = bytes.NewReader(decryptedData)
		if len(decryptedData) >= 2 && decryptedData[0] == 0x1f && decryptedData[1] == 0x8b {
			zr, gzErr := gzip.NewReader(bytes.NewReader(decryptedData))
			if gzErr != nil {
				return
			}
			defer zr.Close()
			decReader = zr
		}
		if err := gob.NewDecoder(decReader).Decode(&files); err != nil {
			return
		}

		mem := vfs.NewMemFS()
		mem.Files = files
		embeddedVFS = mem
		core.SetFileSystem(mem)
		server.GlobalFileSystem = mem
	})
	return embeddedVFS
}

//export _joss_host_bridge
func _joss_host_bridge(methodNamePtr *C.char) {
	if methodNamePtr == nil {
		return
	}
	methodName := C.GoString(methodNamePtr)
	fmt.Printf("[HostBridge ABI] Llamada nativa recibida para: %s\n", methodName)

	memFS := loadEmbeddedVFS()

	if methodName == "Server::start" || strings.HasSuffix(methodName, "Server_start") {
		if memFS != nil {
			server.Start(memFS)
		} else {
			server.Start(nil)
		}
	} else if methodName == "Plugin::call" || strings.HasSuffix(methodName, "Plugin_call") {
		fmt.Printf("[HostBridge ABI] Plugin call: %s\n", methodName)
	}
}

func main() {}
