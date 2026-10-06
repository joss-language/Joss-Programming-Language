package vfs

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"time"
)

// MemFS implements http.FileSystem backed by an in-memory map.
type MemFS struct {
	Files map[string][]byte
}

func NewMemFS() *MemFS {
	return &MemFS{
		Files: make(map[string][]byte),
	}
}

// GetFiles returns the raw underlying files map.
func (fs *MemFS) GetFiles() map[string][]byte {
	if fs == nil {
		return nil
	}
	return fs.Files
}

func (fs *MemFS) Open(name string) (http.File, error) {
	// Clean path and remove leading slash
	name = strings.TrimPrefix(path.Clean(name), "/")
	if name == "" || name == "." {
		name = "" // Root
	}

	// Check if it's a file
	if content, ok := fs.Files[name]; ok {
		return &memFile{
			name:    path.Base(name),
			content: bytes.NewReader(content),
			size:    int64(len(content)),
			isDir:   false,
		}, nil
	}

	// Check if it's a directory (by prefix)
	// This is O(N), but acceptable for small asset bundles.
	// For larger bundles, a tree structure would be better.
	var children []os.FileInfo
	isDir := false
	dirPrefix := name
	if dirPrefix != "" {
		dirPrefix += "/"
	}

	for fName, _ := range fs.Files {
		if name == "" || strings.HasPrefix(fName, dirPrefix) {
			// It's inside this directory
			// Check if it's a direct child
			rel := strings.TrimPrefix(fName, dirPrefix)
			if !strings.Contains(rel, "/") && rel != "" {
				isDir = true
				children = append(children, &memFileInfo{
					name:  rel,
					size:  0,     // Directory size irrelevant here
					isDir: false, // We don't track subdirs explicitly in this simple loop, but files are enough for http.FileServer
				})
			} else if strings.Contains(rel, "/") {
				// It is a subdirectory
				subDirName := strings.Split(rel, "/")[0]
				// Check if we already added this subdir
				found := false
				for _, child := range children {
					if child.Name() == subDirName {
						found = true
						break
					}
				}
				if !found {
					isDir = true
					children = append(children, &memFileInfo{
						name:  subDirName,
						size:  0,
						isDir: true,
					})
				}
			}
		}
	}

	if isDir || name == "" {
		return &memFile{
			name:     path.Base(name),
			children: children,
			isDir:    true,
		}, nil
	}

	return nil, os.ErrNotExist
}

// memFile implements http.File
type memFile struct {
	name     string
	content  *bytes.Reader
	size     int64
	isDir    bool
	children []os.FileInfo
	dirIdx   int
}

func (f *memFile) Close() error { return nil }

func (f *memFile) Read(p []byte) (n int, err error) {
	if f.isDir {
		return 0, errors.New("is a directory")
	}
	return f.content.Read(p)
}

func (f *memFile) Seek(offset int64, whence int) (int64, error) {
	if f.isDir {
		return 0, errors.New("is a directory")
	}
	return f.content.Seek(offset, whence)
}

func (f *memFile) Readdir(count int) ([]os.FileInfo, error) {
	if !f.isDir {
		return nil, errors.New("not a directory")
	}
	if f.dirIdx >= len(f.children) {
		if count > 0 {
			return nil, io.EOF
		}
		return nil, nil
	}

	if count <= 0 {
		return f.children, nil
	}

	end := f.dirIdx + count
	if end > len(f.children) {
		end = len(f.children)
	}
	res := f.children[f.dirIdx:end]
	f.dirIdx = end
	return res, nil
}

func (f *memFile) Stat() (os.FileInfo, error) {
	return &memFileInfo{
		name:  f.name,
		size:  f.size,
		isDir: f.isDir,
	}, nil
}

// memFileInfo implements os.FileInfo
type memFileInfo struct {
	name  string
	size  int64
	isDir bool
}

func (i *memFileInfo) Name() string { return i.name }
func (i *memFileInfo) Size() int64  { return i.size }
func (i *memFileInfo) Mode() os.FileMode {
	if i.isDir {
		return os.ModeDir | 0555
	}
	return 0444
}
func (i *memFileInfo) ModTime() time.Time { return time.Now() } // Dummy time
func (i *memFileInfo) IsDir() bool        { return i.isDir }
func (i *memFileInfo) Sys() interface{}   { return nil }
