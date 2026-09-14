package i18n_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/jossecurity/joss/pkg/i18n"
)

func TestLocaleFilesPreserveCanonicalKeysAndKeepCLISyntaxOutOfTranslations(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("locales", "intl_*.arb"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 30 {
		t.Fatalf("expected 30 ARB locale files, got %d", len(paths))
	}

	commandSyntax := regexp.MustCompile(`(?i)\bjoss\s+(?:run|new|server\s+start|migrate(?::fresh)?|make:[a-z]+|remove:crud|db:seed|format|lint|fix|test|analyze|build|userstorage|change\s+db|pub\s+(?:add|remove|install|publish|search|update)|plugin\s+(?:compile|inspect|verify))\b`)
	flagSyntax := regexp.MustCompile(`(?i)(?:^|[^-])--[a-z][a-z-]*`)
	exampleFilename := regexp.MustCompile(`(?i)\b(?:main|tu_script|file|archivo)\.joss\b`)
	promptSuffix := regexp.MustCompile(`(?i)[\(（][sy]\s*/\s*n[\)）]\s*:`)
	placeholder := regexp.MustCompile(`\{([a-zA-Z0-9_]+)\}`)

	var canonical []string
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var document map[string]json.RawMessage
		if err := json.Unmarshal(data, &document); err != nil {
			t.Fatalf("%s is not valid ARB JSON: %v", path, err)
		}

		keys := make([]string, 0, len(document))
		for key := range document {
			if !strings.HasPrefix(key, "@") {
				keys = append(keys, key)
			}
		}
		sort.Strings(keys)
		if canonical == nil {
			canonical = append([]string(nil), keys...)
		} else if strings.Join(keys, "\x00") != strings.Join(canonical, "\x00") {
			t.Errorf("%s does not have the canonical ARB key set", path)
		}

		for _, key := range keys {
			var value string
			if err := json.Unmarshal(document[key], &value); err != nil {
				t.Errorf("%s key %q is not a string: %v", path, key, err)
				continue
			}
			if commandSyntax.MatchString(value) || flagSyntax.MatchString(value) || exampleFilename.MatchString(value) || promptSuffix.MatchString(value) {
				t.Errorf("%s key %q contains CLI syntax, a selector or an example filename: %q", path, key, value)
			}

			used := map[string]bool{}
			for _, match := range placeholder.FindAllStringSubmatch(value, -1) {
				used[match[1]] = true
			}
			declared := map[string]json.RawMessage{}
			if metadata, ok := document["@"+key]; ok {
				var details struct {
					Placeholders map[string]json.RawMessage `json:"placeholders"`
				}
				if err := json.Unmarshal(metadata, &details); err != nil {
					t.Errorf("%s metadata for %q is invalid: %v", path, key, err)
				}
				declared = details.Placeholders
			}
			if len(used) != len(declared) {
				t.Errorf("%s key %q placeholder metadata differs: used=%v declared=%v", path, key, used, declared)
				continue
			}
			for name := range used {
				if _, ok := declared[name]; !ok {
					t.Errorf("%s key %q does not declare placeholder %q", path, key, name)
				}
			}
		}
	}
}

func TestI18nLoadAllLocales(t *testing.T) {
	mgr := i18n.NewManager()
	if err := mgr.Load(nil); err != nil {
		t.Fatalf("Failed to load embedded locales: %v", err)
	}

	locales := mgr.GetAvailableLocales()
	if len(locales) < 30 {
		t.Fatalf("Expected at least 30 locales, got %d: %v", len(locales), locales)
	}

	var hasEn, hasEs bool
	for _, l := range locales {
		if l == "en" {
			hasEn = true
		}
		if l == "es" {
			hasEs = true
		}
	}
	if !hasEn || !hasEs {
		t.Errorf("Expected both 'en' and 'es' to be present, got %v", locales)
	}
}

func TestI18nPlaceholderInterpolation(t *testing.T) {
	mgr := i18n.NewManager()
	_ = mgr.Load(nil)

	// Test serverStarting interpolation
	res := mgr.Get("es", "serverStarting", map[string]interface{}{
		"host": "127.0.0.1",
		"port": 8080,
	})
	expected := "Iniciando servidor web HTTP en http://127.0.0.1:8080"
	if res != expected {
		t.Errorf("Expected %q, got %q", expected, res)
	}

	// Test aiSelectedProvider interpolation
	resProvider := mgr.Get("en", "aiSelectedProvider", map[string]interface{}{
		"provider": "GROQ",
	})
	if !strings.Contains(resProvider, "GROQ") {
		t.Errorf("Expected resProvider to contain 'GROQ', got %q", resProvider)
	}
}

func TestI18nConsistencyAcrossAllLocales(t *testing.T) {
	mgr := i18n.NewManager()
	_ = mgr.Load(nil)

	locales := mgr.GetAvailableLocales()
	phRegex := regexp.MustCompile(`\{[a-zA-Z0-9_]+\}`)

	testKeys := []string{
		// cliUsage*, cliUsageRun, cliUsageServer eliminadas (→ Lote 5: cliUsageLabel)
		"version", "cliAppTitle", "cliCmdRun", "cliCmdRepl",
		"cliCmdServer", "cliCmdAnalyze", "cliCmdLint", "cliCmdFormat",
		"cliCmdTest", "cliCmdBuild", "cliCmdPub", "aiWizardTitle",
		"aiWizardConfig", "aiSelectProvider", "aiProviderGroq", "aiProviderOpenai",
		"aiProviderGemini", "aiChooseOption", "aiInvalidOption", "aiSelectedProvider",
		"aiModelToUse", "aiEnterApiKey", "aiNoApiKeyWarning", "aiSavingConfig",
		"aiActivatedSuccess", "aiTryScript", "createMigration", "removeCRUD",
		"serverStarting", "serverStopped", "lintSuccess", "analyzeSuccess",
		"cliErrNoMain", "cliErrRequireMain",
		"checkVerifying", "checkFormatOk", "checkResultOk", "formatAllOk",
		"fixAllClean", "testNoFilesFound", "testRunning", "migrateStarting",
		"migrateDbSuccess", "migrateCompleted",
		"buildWebStart", "buildMissingRequired", "buildStrictStructure", "buildCreatingDir",
		"buildCopyingFiles", "buildDbCopied", "buildNginxPortCreated", "buildWebSuccess",
		"buildProgramStart", "pkgBuildStarting", "pkgBuildSuccess",
		"cliUsageLabel", "cliSectionExec", "cliCmdMakeController",
		"migrateFreshStarting", "migrateFreshSuccess", "seedersRunning",
		"dbChangingEngine", "dbMigrationCompleted", "storageConfiguring",
		"storagePromptDownloadOci", "storageUploading",
		"nativeBuildMissingGo", "nativeBuildSuccessTitle",
		"updaterNewVersionTitle", "updaterAlreadyUpdated",
		"pubManagerTitle", "pubCommandsLabel", "pubCmdAddDesc",
		"pubConnectError", "pubLoginSuccess", "pubPackageDownloads", "pubPackageName",
		"pubSearchingPlatform", "pubInstallSuccessPlatform", "pubManifestNotFound",
		"pubResolvingDependencies", "pubPublishedSuccess", "pubCacheEmptied",
		"pluginUsageTitle", "pluginCompiling", "pluginCompileSuccess", "pluginPackageSize",
		"pluginTreeShaking", "pluginInspectTitle", "pluginSignatureValid",
		"seedersExecError", "seederItem", "migrateExecError", "dbMigrationError",
		"dbPrefixTablesRenamed", "storageOciInitError", "storageUploadError",
		"updaterPrepareRequestError", "updaterSelectedRelease", "nativeBuildDirError",
		"nativeBuildPrecompiledFiles", "buildCreateDirError", "pkgBuildEntryEscapes",
		"pkgBuildAuthorKey", "pkgInspectHeader", "formatNotCanonical", "fixAppliedFile",
		"checkDiagnosticsSummary", "programRunningMain", "cliUnknownCommand", "cliScriptNotFound",
	}

	esKeys := make(map[string]string)
	for _, k := range testKeys {
		esVal := mgr.Get("es", k, nil)
		if esVal == k || esVal == "" {
			t.Errorf("Key %q not found in es locale", k)
		}
		esKeys[k] = esVal
	}

	for _, loc := range locales {
		for _, k := range testKeys {
			val := mgr.Get(loc, k, nil)
			if val == k || val == "" {
				t.Errorf("Locale %s is missing key %q", loc, k)
				continue
			}

			// Verify placeholders match
			esPhs := phRegex.FindAllString(esKeys[k], -1)
			locPhs := phRegex.FindAllString(val, -1)
			if len(esPhs) != len(locPhs) {
				t.Errorf("Locale %s key %q placeholder mismatch: expected %v, got %v in %q", loc, k, esPhs, locPhs, val)
			}
		}
	}
}
