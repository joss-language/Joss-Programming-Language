//go:build !windows

package server

import "os"

func replaceSessionFile(source, destination string) error {
	return os.Rename(source, destination)
}
