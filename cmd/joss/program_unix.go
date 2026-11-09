//go:build !windows

package main

import (
	_ "embed"
)

//go:embed default_logo.png
var defaultLogo []byte

func startProgram() {
	handleNativeServerStart([]string{"--gui"})
}
