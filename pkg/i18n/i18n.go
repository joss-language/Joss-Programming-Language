package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
)

//go:embed locales/*.arb
var embeddedLocales embed.FS

type Manager struct {
	locales       map[string]map[string]string
	mu            sync.RWMutex
	DefaultLocale string
	loaded        bool
}

var GlobalManager = NewManager()

func NewManager() *Manager {
	return &Manager{
		locales:       make(map[string]map[string]string),
		DefaultLocale: "en",
	}
}

// LoadWALks the locales directory.
// If fs is provided, it tries to load from there (e.g., user project l10n).
// Otherwise/In addition, it loads from embedded locales.
func (m *Manager) Load(externalFS http.FileSystem) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.loaded {
		return nil
	}

	// 1. Load Embedded Locales (Core translations)
	embedFiles, err := fs.ReadDir(embeddedLocales, "locales")
	if err == nil {
		for _, file := range embedFiles {
			if !file.IsDir() && strings.HasSuffix(file.Name(), ".arb") {
				m.loadFromFile(embeddedLocales, "locales/"+file.Name())
			}
		}
	} else {
		fmt.Printf("[I18n] Warning: failed to read embedded locales: %v\n", err)
	}

	// 2. Load External/User Locales (if provided OR disk)
	if externalFS != nil {
		// VFS Mode
		dir, err := externalFS.Open("/assets/l10n")
		if err == nil {
			defer dir.Close()
			files, err := dir.Readdir(0)
			if err == nil {
				for _, file := range files {
					if !file.IsDir() && strings.HasSuffix(file.Name(), ".arb") {
						f, err := externalFS.Open(path.Join("/assets/l10n", file.Name()))
						if err == nil {
							m.loadFromReader(f, file.Name())
							f.Close()
						} else {
							fmt.Printf("[I18n] Warning: failed to open locale file %s: %v\n", file.Name(), err)
						}
					}
				}
			}
		}
	} else {
		// Disk Mode (Local Development)
		entries, err := os.ReadDir("assets/l10n")
		if err == nil {
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".arb") {
					f, err := os.Open(filepath.Join("assets", "l10n", e.Name()))
					if err == nil {
						m.loadFromReaderFile(f, e.Name())
						f.Close()
					} else {
						fmt.Printf("[I18n] Warning: failed to open locale file %s: %v\n", e.Name(), err)
					}
				}
			}
		}
	}

	m.loaded = true
	return nil
}

func (m *Manager) loadFromFile(fsys fs.FS, path string) {
	data, err := fs.ReadFile(fsys, path)
	if err != nil {
		fmt.Printf("[I18n] Warning: failed to read embedded file %s: %v\n", path, err)
		return
	}

	// Parse Filename for Locale (e.g. intl_es.arb -> es)
	filename := path
	if idx := strings.LastIndex(path, "/"); idx != -1 {
		filename = path[idx+1:]
	}

	m.parseAndMerge(filename, data)
}

func (m *Manager) loadFromReaderFile(r *os.File, filename string) {
	stat, _ := r.Stat()
	data := make([]byte, stat.Size())
	r.Read(data)
	m.parseAndMerge(filename, data)
}

func (m *Manager) loadFromReader(r http.File, filename string) {
	// Read all
	stat, _ := r.Stat()
	data := make([]byte, stat.Size())
	r.Read(data)
	m.parseAndMerge(filename, data)
}

func (m *Manager) parseAndMerge(filename string, data []byte) {
	// Extract locale
	namePart := strings.TrimSuffix(filename, ".arb")
	parts := strings.Split(namePart, "_")
	locale := parts[len(parts)-1]

	var content map[string]interface{}
	if err := json.Unmarshal(data, &content); err != nil {
		fmt.Printf("[I18n] Error decoding %s: %v\n", filename, err)
		return
	}

	if m.locales[locale] == nil {
		m.locales[locale] = make(map[string]string)
	}

	for k, v := range content {
		if strings.HasPrefix(k, "@") {
			continue
		}
		if strVal, ok := v.(string); ok {
			m.locales[locale][k] = strVal
		}
	}
	// fmt.Printf("[I18n] Loaded locale '%s' from %s\n", locale, filename)
}

// Get returns the translation for the given key and locale.
// Supports placeholders like {name}.
func (m *Manager) Get(locale, key string, args map[string]interface{}) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 1. Try requested locale
	val, ok := m.locales[locale][key]

	// 2. Try simple locale (e.g. es-MX -> es)
	if !ok && strings.Contains(locale, "-") {
		simple := strings.Split(locale, "-")[0]
		val, ok = m.locales[simple][key]
	}

	// 3. Fallback to DefaultLocale
	if !ok {
		val, ok = m.locales[m.DefaultLocale][key]
	}

	// 4. Return key if not found
	if !ok {
		return key
	}

	// Replace placeholders
	// We handle simple check {key}
	for splitK, splitV := range args {
		placeholder := "{" + splitK + "}"
		val = strings.ReplaceAll(val, placeholder, fmt.Sprintf("%v", splitV))
	}

	return val
}

// GetAvailableLocales returns a list of loaded locales
func (m *Manager) GetAvailableLocales() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	keys := make([]string, 0, len(m.locales))
	for k := range m.locales {
		keys = append(keys, k)
	}
	return keys
}

var CurrentLocale = "en"

func init() {
	// Detect locale from environment
	locale := os.Getenv("JOSS_LANG")
	if locale == "" {
		sysLang := os.Getenv("LANG")
		if sysLang != "" {
			parts := strings.FieldsFunc(sysLang, func(r rune) bool {
				return r == '_' || r == '.'
			})
			if len(parts) > 0 {
				locale = parts[0]
			}
		}
	}
	if locale == "" {
		locale = "en"
	}
	CurrentLocale = locale
}

// Tr translates a key based on the automatically detected current locale
func Tr(key string, args ...map[string]interface{}) string {
	var arg map[string]interface{}
	if len(args) > 0 {
		arg = args[0]
	}
	// Ensure loaded (Safe to call repeatedly, read lock inside)
	if !GlobalManager.loaded {
		_ = GlobalManager.Load(nil)
	}
	return GlobalManager.Get(CurrentLocale, key, arg)
}

// T is a concise alias for Tr
func T(key string, args ...map[string]any) string {
	if len(args) > 0 {
		return Tr(key, args[0])
	}
	return Tr(key)
}
