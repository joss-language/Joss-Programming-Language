package native

import (
	"fmt"
	"runtime"
	"strings"
)

// Target define la plataforma y arquitectura destino de compilación.
type Target struct {
	OS   string
	Arch string
}

func (t Target) String() string {
	if t.OS == "" && t.Arch == "" {
		return HostTarget().String()
	}
	return t.OS + "/" + t.Arch
}

func (t Target) DashString() string {
	if t.OS == "" && t.Arch == "" {
		return HostTarget().DashString()
	}
	return t.OS + "-" + t.Arch
}

// HostTarget devuelve el target de la máquina actual que ejecuta el compilador.
func HostTarget() Target {
	return Target{
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
	}
}

// SupportedTargets registra las plataformas y arquitecturas con soporte verificado en Joss.
var SupportedTargets = map[string][]string{
	"windows": {"amd64", "arm64"},
	"linux":   {"amd64", "arm64", "arm"},
	"darwin":  {"amd64", "arm64"},
	"android": {"arm64", "arm"},
}

// ParseTarget valida y normaliza un objetivo de compilación (ej. "windows-amd64", "linux-arm64", "linux", "macos").
func ParseTarget(s string) (Target, error) {
	if s == "" {
		return HostTarget(), nil
	}
	s = strings.ToLower(strings.TrimSpace(strings.ReplaceAll(s, "/", "-")))
	switch s {
	case "windows", "win":
		return Target{OS: "windows", Arch: "amd64"}, nil
	case "linux":
		return Target{OS: "linux", Arch: "amd64"}, nil
	case "darwin", "macos", "mac", "osx":
		return Target{OS: "darwin", Arch: "arm64"}, nil
	case "android", "termux":
		return Target{OS: "android", Arch: "arm64"}, nil
	}
	parts := strings.Split(s, "-")
	if len(parts) != 2 {
		return Target{}, fmt.Errorf("formato de target inválido '%s'. Debe ser <os>-<arch> (ej. windows-amd64, linux-amd64, linux-arm64, darwin-arm64)", s)
	}

	osName := strings.ToLower(parts[0])
	if osName == "macos" || osName == "mac" || osName == "osx" {
		osName = "darwin"
	} else if osName == "win" {
		osName = "windows"
	}
	archName := strings.ToLower(parts[1])
	if archName == "x86_64" || archName == "x64" {
		archName = "amd64"
	} else if archName == "aarch64" {
		archName = "arm64"
	}

	archs, ok := SupportedTargets[osName]
	if !ok {
		return Target{}, fmt.Errorf("sistema operativo '%s' no soportado para compilación nativa en Joss", osName)
	}

	for _, a := range archs {
		if a == archName {
			return Target{OS: osName, Arch: archName}, nil
		}
	}

	return Target{}, fmt.Errorf("arquitectura '%s' no soportada para %s. Opciones soportadas: %s", archName, osName, strings.Join(archs, ", "))
}
