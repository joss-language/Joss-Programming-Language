package i18n_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/jossecurity/joss/pkg/i18n"
)

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
