package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jossecurity/joss/pkg/i18n"
	"gopkg.in/yaml.v3"
)

type PluginCommandInfo struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description" json:"description"`
	Usage       string `yaml:"usage" json:"usage"`
	Protected   bool   `yaml:"protected" json:"protected"`
	Handler     string `yaml:"handler" json:"handler"`
}

type PluginManifestInfo struct {
	Name        string                       `yaml:"name" json:"name"`
	Version     string                       `yaml:"version" json:"version"`
	Description string                       `yaml:"description" json:"description"`
	Repository  string                       `yaml:"repository" json:"repository"`
	Dir         string                       `yaml:"-" json:"-"`
	Commands    map[string]PluginCommandInfo `yaml:"commands" json:"commands"`
}

// discoverPlugins scans local folders, examples, and joss.yaml to find all available plugins
func discoverPlugins() map[string]PluginManifestInfo {
	plugins := make(map[string]PluginManifestInfo)

	// 1. Search in local plugins/ directory and example plugin directories
	searchPluginDirs := []string{
		"plugins",
		filepath.Join("..", "plugins"),
		filepath.Join("..", "..", "plugins"),
		filepath.Join("ejemplos", "plugins"),
		filepath.Join("..", "ejemplos", "plugins"),
		filepath.Join("..", "..", "ejemplos", "plugins"),
	}


	for _, baseDir := range searchPluginDirs {
		entries, err := os.ReadDir(baseDir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				// Could be a .jp file
				if strings.HasSuffix(strings.ToLower(entry.Name()), ".jp") {
					pName := strings.TrimSuffix(entry.Name(), ".jp")
					if _, ok := plugins[pName]; !ok {
						manifest := PluginManifestInfo{
							Name:        pName,
							Version:     "latest",
							Description: fmt.Sprintf("Paquete compilado .jp de %s", pName),
							Dir:         baseDir,
							Commands:    make(map[string]PluginCommandInfo),
						}
						plugins[pName] = manifest
					}
				}
				continue
			}

			pluginDir := filepath.Join(baseDir, entry.Name())
			yamlPath := filepath.Join(pluginDir, "joss.yaml")
			if _, err := os.Stat(yamlPath); err == nil {
				data, err := os.ReadFile(yamlPath)
				if err == nil {
					var manifest PluginManifestInfo
					if err := yaml.Unmarshal(data, &manifest); err == nil && manifest.Name != "" {
						manifest.Dir = pluginDir
						if manifest.Commands == nil {
							manifest.Commands = make(map[string]PluginCommandInfo)
						}
						plugins[manifest.Name] = manifest
					}
				}
			}
		}
	}

	return plugins
}

// handlePluginHelp prints command details for all plugins or a specific plugin
func handlePluginHelp(pluginName string) {
	allPlugins := discoverPlugins()

	if pluginName != "" {
		pName := strings.ToLower(strings.TrimSpace(pluginName))
		manifest, found := allPlugins[pName]
		if !found {
			// Try finding with joss_ prefix
			if !strings.HasPrefix(pName, "joss_") {
				manifest, found = allPlugins["joss_"+pName]
			}
		}

		if !found {
			fmt.Println(i18n.Tr("pluginNotFound", i18n.M{"name": pluginName}))
			fmt.Println("Usa 'joss help plugins' para ver la lista de plugins disponibles.")
			return
		}

		fmt.Printf("Plugin: %s (v%s)\n", manifest.Name, manifest.Version)
		if manifest.Description != "" {
			fmt.Printf("Descripción: %s\n", manifest.Description)
		}
		if manifest.Repository != "" {
			fmt.Printf("Repositorio: %s\n", manifest.Repository)
		}
		fmt.Println()

		if len(manifest.Commands) == 0 {
			fmt.Println(i18n.Tr("pluginNoProtectedCommands"))
			return
		}

		fmt.Println(i18n.Tr("pluginProvidedCommands"))
		// Sort commands
		cmdNames := make([]string, 0, len(manifest.Commands))
		for cName := range manifest.Commands {
			cmdNames = append(cmdNames, cName)
		}
		sort.Strings(cmdNames)

		for _, cName := range cmdNames {
			cmd := manifest.Commands[cName]
			protTag := ""
			if cmd.Protected {
				protTag = " [protegido]"
			}
			fmt.Printf("  joss %s%s\n", cName, protTag)
			if cmd.Description != "" {
				fmt.Printf("    Descripción: %s\n", cmd.Description)
			}
			if cmd.Usage != "" {
				fmt.Printf("    Uso:         %s\n", cmd.Usage)
			}
			fmt.Println()
		}
		return
	}

	// Print all plugins
	fmt.Println(i18n.Tr("pluginAvailableTitle"))
	fmt.Println()

	pluginNames := make([]string, 0, len(allPlugins))
	for pName := range allPlugins {
		pluginNames = append(pluginNames, pName)
	}
	sort.Strings(pluginNames)

	for _, pName := range pluginNames {
		manifest := allPlugins[pName]
		desc := manifest.Description
		if desc == "" {
			desc = "Plugin de Joss"
		}
		fmt.Printf("[%s] v%s — %s\n", manifest.Name, manifest.Version, desc)

		if len(manifest.Commands) == 0 {
			fmt.Println("  " + i18n.Tr("pluginNoDirectCli"))
		} else {
			cmdNames := make([]string, 0, len(manifest.Commands))
			for cName := range manifest.Commands {
				cmdNames = append(cmdNames, cName)
			}
			sort.Strings(cmdNames)

			for _, cName := range cmdNames {
				cmd := manifest.Commands[cName]
				protTag := ""
				if cmd.Protected {
					protTag = " [protegido]"
				}
				fmt.Printf("  %-28s - %s\n", cName+protTag, cmd.Description)
			}
		}
		fmt.Println()
	}

	fmt.Println("Usa 'joss help plugins <nombre_plugin>' para ver detalles y opciones de un plugin específico.")
}

// tryDispatchPluginCommand executes recognized plugin commands
func tryDispatchPluginCommand(command string, args []string) bool {
	plugins := discoverPlugins()
	for _, p := range plugins {
		cmd, ok := p.Commands[command]
		if !ok {
			continue
		}

		if cmd.Handler != "" && p.Dir != "" {
			handlerPath := filepath.Join(p.Dir, cmd.Handler)
			if strings.HasSuffix(handlerPath, ".joss") && fileExists(handlerPath) {
				executeScript(handlerPath)
				return true
			}
			if strings.HasSuffix(handlerPath, ".go") && fileExists(handlerPath) {
				cmdArgs := append([]string{"run", handlerPath}, args...)
				c := exec.Command("go", cmdArgs...)
				c.Stdout = os.Stdout
				c.Stderr = os.Stderr
				c.Stdin = os.Stdin
				if err := c.Run(); err != nil {
					fmt.Printf("Error al ejecutar comando '%s': %v\n", command, err)
				}
				return true
			}
			if fileExists(handlerPath) {
				c := exec.Command(handlerPath, args...)
				c.Stdout = os.Stdout
				c.Stderr = os.Stderr
				c.Stdin = os.Stdin
				if err := c.Run(); err != nil {
					fmt.Printf("Error al ejecutar comando '%s': %v\n", command, err)
				}
				return true
			}
		}

		fmt.Printf("Comando '%s' provisto por el plugin '%s'.\n", command, p.Name)
		if cmd.Description != "" {
			fmt.Printf("  Descripción: %s\n", cmd.Description)
		}
		if cmd.Usage != "" {
			fmt.Printf("  Uso:         %s\n", cmd.Usage)
		}
		return true
	}
	return false
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
