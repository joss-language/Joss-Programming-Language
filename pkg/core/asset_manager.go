package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// AssetManager handles dynamic asset discovery and optimization
type AssetManager struct {
	ProjectRoot  string
	NodeModules  map[string]PackageAssets // PkgName -> Assets
	ViewCSSCache map[string]string        // ViewName -> OptimizedCSS
	GlobalCSS    string                   // Content of public/css/app.css
	CacheMutex   sync.RWMutex
	initMutex    sync.Mutex
	Initialized  bool
}

type PackageAssets struct {
	Name    string
	Version string
	CSS     []string // Relative paths in node_modules
	JS      []string // Relative paths in node_modules
}

var (
	GlobalAssetManager *AssetManager
	onceAM             sync.Once
)

// GetAssetManager returns the singleton instance
func GetAssetManager() *AssetManager {
	onceAM.Do(func() {
		GlobalAssetManager = &AssetManager{
			ProjectRoot:  ".",
			NodeModules:  make(map[string]PackageAssets),
			ViewCSSCache: make(map[string]string),
		}
	})
	return GlobalAssetManager
}

// Initialize scans node_modules and loads global CSS
func (am *AssetManager) Initialize() {
	am.initMutex.Lock()
	defer am.initMutex.Unlock()
	if am.Initialized {
		return
	}

	fmt.Println("[AssetManager] Initializing...")
	am.ScanNodeModules()
	am.LoadGlobalCSS()
	am.Initialized = true
}

// LoadGlobalCSS reads the main compiled CSS file
func (am *AssetManager) LoadGlobalCSS() {
	// We look for public/css/app.css which is the result of SCSS compilation
	path := filepath.Join("public", "css", "app.css")
	data, err := os.ReadFile(path)
	if err == nil {
		am.CacheMutex.Lock()
		am.GlobalCSS = string(data)
		length := len(am.GlobalCSS)
		am.CacheMutex.Unlock()
		fmt.Printf("[AssetManager] Loaded Global CSS (%d bytes)\n", length)
	} else {
		fmt.Println("[AssetManager] Warning: public/css/app.css not found. Dynamic optimization might be empty until rebuild.")
	}
}

// ScanNodeModules discovers installed packages and their assets
func (am *AssetManager) ScanNodeModules() {
	am.CacheMutex.Lock()
	am.NodeModules = make(map[string]PackageAssets) // Reset state
	am.CacheMutex.Unlock()

	packageJsonPath := filepath.Join(am.ProjectRoot, "package.json")
	data, err := os.ReadFile(packageJsonPath)
	if err != nil {
		return // No package.json, skip
	}

	var pkg struct {
		Dependencies map[string]string `json:"dependencies"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		fmt.Println("[AssetManager] Error parsing package.json:", err)
		return
	}

	for dep := range pkg.Dependencies {
		am.detectPackageAssets(dep)
	}
}

func (am *AssetManager) detectPackageAssets(pkgName string) {
	pkgPath := filepath.Join(am.ProjectRoot, "node_modules", pkgName)
	if _, err := os.Stat(pkgPath); os.IsNotExist(err) {
		return
	}

	assets := PackageAssets{Name: pkgName}

	// 1. Check package.json for "style", "main", etc.
	pkgJsonPath := filepath.Join(pkgPath, "package.json")
	if data, err := os.ReadFile(pkgJsonPath); err == nil {
		var pJson map[string]interface{}
		json.Unmarshal(data, &pJson)

		// Check 'style' field
		if style, ok := pJson["style"].(string); ok {
			assets.CSS = append(assets.CSS, filepath.Join(pkgName, style))
		}
		// Check 'main' or 'browser' for JS
		if main, ok := pJson["main"].(string); ok && strings.HasSuffix(main, ".js") {
			assets.JS = append(assets.JS, filepath.Join(pkgName, main))
		}
	}

	// 2. Heuristics: Look for dist/ folders or standard files if nothing found
	if len(assets.CSS) == 0 {
		// Look for .min.css in dist/ or root
		candidates, _ := filepath.Glob(filepath.Join(pkgPath, "dist", "*.min.css"))
		candidates2, _ := filepath.Glob(filepath.Join(pkgPath, "*.min.css"))
		candidates = append(candidates, candidates2...)

		for _, c := range candidates {
			rel, _ := filepath.Rel(filepath.Join(am.ProjectRoot, "node_modules"), c)
			assets.CSS = append(assets.CSS, rel)
		}
	}

	if len(assets.JS) == 0 {
		// Look for .min.js in dist/ or root
		candidates, _ := filepath.Glob(filepath.Join(pkgPath, "dist", "*.min.js"))
		candidates2, _ := filepath.Glob(filepath.Join(pkgPath, "*.min.js"))
		candidates = append(candidates, candidates2...)

		for _, c := range candidates {
			rel, _ := filepath.Rel(filepath.Join(am.ProjectRoot, "node_modules"), c)
			assets.JS = append(assets.JS, rel)
		}
	}

	if len(assets.CSS) > 0 || len(assets.JS) > 0 {
		fmt.Printf("[AssetManager] Detected %s: %d CSS, %d JS\n", pkgName, len(assets.CSS), len(assets.JS))
		am.CacheMutex.Lock()
		am.NodeModules[pkgName] = assets
		am.CacheMutex.Unlock()
	}
}

// GetVendorIncludes returns HTML tags for detected assets
func (am *AssetManager) GetVendorIncludes() (string, string) {
	am.CacheMutex.RLock()
	defer am.CacheMutex.RUnlock()

	var css, js strings.Builder

	for _, assets := range am.NodeModules {
		for _, f := range assets.CSS {
			// Virtual path: /assets/vendor/PKG/FILE
			// But 'f' is already 'PKG/path/to/file'
			// So verify separator
			f = filepath.ToSlash(f)
			css.WriteString(fmt.Sprintf(`<link rel="stylesheet" href="/assets/vendor/%s">`+"\n", f))
		}
		for _, f := range assets.JS {
			f = filepath.ToSlash(f)
			js.WriteString(fmt.Sprintf(`<script src="/assets/vendor/%s"></script>`+"\n", f))
		}
	}
	return css.String(), js.String()
}

// OptimizeViewCSS returns a style block specific to the view
func (am *AssetManager) OptimizeViewCSS(viewName string, htmlContent string) string {
	am.CacheMutex.RLock()
	if css, ok := am.ViewCSSCache[viewName]; ok {
		am.CacheMutex.RUnlock()
		return fmt.Sprintf("<style>\n%s\n</style>", css)
	}
	am.CacheMutex.RUnlock()

	// Optimization Logic (Similar to strict optimizer but dynamic)
	am.CacheMutex.RLock()
	globalCSS := am.GlobalCSS
	am.CacheMutex.RUnlock()
	if globalCSS == "" {
		am.LoadGlobalCSS()
		am.CacheMutex.RLock()
		globalCSS = am.GlobalCSS
		am.CacheMutex.RUnlock()
	}

	// 1. Extract tokens from HTML
	viewTokens := make(map[string]bool)
	reToken := regexp.MustCompile(`[\w-]+`)
	matches := reToken.FindAllString(htmlContent, -1)
	for _, m := range matches {
		viewTokens[m] = true
	}
	// Whitelist tags
	htmlTags := []string{"body", "html", "div", "span", "a", "p", "h1", "h2", "h3", "h4", "h5", "h6", "ul", "ol", "li", "form", "input", "button", "nav", "header", "footer", "main", "section", "article", "*", ":root"}
	for _, tag := range htmlTags {
		viewTokens[tag] = true
	}

	// 2. Filter Global CSS
	// REGEX WARNING: A simple regex cannot safely parse nested CSS (like @media { .class {} }).
	// This was causing the layout breakage (stripping background/media queries).
	// For now, we utilize "Safe Mode": Serve valid, compressed CSS without purging.
	// Future: Implement a proper state-machine CSS parser.
	optimized := globalCSS

	/*
		reBlock := regexp.MustCompile(`(?s)([^{}]+)\{([^{}]+)\}`)
		optimized := reBlock.ReplaceAllStringFunc(am.GlobalCSS, func(block string) string {
			// ... (Purge Logic Disabled for Safety)
			return block
		})
	*/

	// Compress
	reSpace := regexp.MustCompile(`\s+`)
	optimized = reSpace.ReplaceAllString(optimized, " ")

	// Cache result
	am.CacheMutex.Lock()
	am.ViewCSSCache[viewName] = optimized
	am.CacheMutex.Unlock()

	return fmt.Sprintf("<style>\n%s\n</style>", optimized)
}
