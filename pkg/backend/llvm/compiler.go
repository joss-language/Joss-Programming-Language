package llvm

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/jossecurity/joss/pkg/backend/native"
)

func init() {
	RegisterAvailableToolchain()
}

// RegisterAvailableToolchain registra el compilador LLVM/C si hay un toolchain
// bundled, gestionado por Joss (~/.joss/toolchain) o instalado en el sistema.
func RegisterAvailableToolchain() {
	if ResolveToolchain().Available() {
		native.RegisterExternalLLVMCompiler(func(llvmIR string, outputPath string, opts native.BuildOptions) error {
			tc, err := EnsureToolchain()
			if err != nil {
				return err
			}
			return compileLLVMWithToolchain(tc, llvmIR, outputPath, opts)
		})
	}
}

func llvmTargetTriple(t native.Target) string {
	switch t.OS {
	case "linux":
		if t.Arch == "arm64" {
			return "aarch64-linux-gnu"
		}
		return "x86_64-linux-gnu"
	case "darwin":
		if t.Arch == "arm64" {
			return "arm64-apple-darwin"
		}
		return "x86_64-apple-darwin"
	case "windows":
		if t.Arch == "arm64" {
			return "aarch64-w64-mingw32"
		}
		return "x86_64-w64-mingw32"
	case "android":
		return "aarch64-linux-android21"
	}
	return ""
}

func zigTargetTriple(t native.Target) string {
	switch t.OS {
	case "linux":
		if t.Arch == "arm64" {
			return "aarch64-linux-gnu"
		}
		return "x86_64-linux-gnu"
	case "darwin":
		if t.Arch == "arm64" {
			return "aarch64-macos"
		}
		return "x86_64-macos"
	case "windows":
		if t.Arch == "arm64" {
			return "aarch64-windows-gnu"
		}
		return "x86_64-windows-gnu"
	case "android":
		return "aarch64-linux-android"
	}
	return ""
}

func compileLLVMWithToolchain(tc ToolchainInfo, llvmIR, outputPath string, opts native.BuildOptions) error {
	if opts.Target.OS == "" {
		opts.Target.OS = runtime.GOOS
	}
	if opts.Target.Arch == "" {
		opts.Target.Arch = runtime.GOARCH
	}
	rtCPath, err := EnsureRuntimeC(opts.RuntimeSource)
	if err != nil {
		return err
	}
	tmpDir, err := os.MkdirTemp("", "joss-llvm-*")
	if err != nil {
		return fmt.Errorf("llvm compiler: crear directorio temporal: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	llPath := filepath.Join(tmpDir, "program.ll")
	progObjPath := filepath.Join(tmpDir, "program.o")
	rtObjPath := filepath.Join(tmpDir, "joss_rt.o")
	if err := os.WriteFile(llPath, []byte(llvmIR), 0644); err != nil {
		return fmt.Errorf("llvm compiler: escribir LLVM IR: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return err
	}

	optLevel := "-Os"
	if opts.Debug {
		optLevel = "-O0"
	}
	triple := llvmTargetTriple(opts.Target)
	zigTriple := zigTargetTriple(opts.Target)

	runCC := func(args ...string) error {
		var stderr bytes.Buffer
		var cmd *exec.Cmd
		if tc.UseZigCC {
			zigArgs := append([]string{"cc"}, args...)
			cmd = exec.Command(tc.CompilerPath, zigArgs...)
		} else {
			cmd = exec.Command(tc.CompilerPath, args...)
		}
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s %v failed: %w\n%s", tc.Kind, args, err, stderr.String())
		}
		return nil
	}

	var bundledInc, bundledLib string
	if tc.RootDir != "" {
		if fi, statErr := os.Stat(filepath.Join(tc.RootDir, "include")); statErr == nil && fi.IsDir() {
			bundledInc = filepath.Join(tc.RootDir, "include")
		}
		if fi, statErr := os.Stat(filepath.Join(tc.RootDir, "lib")); statErr == nil && fi.IsDir() {
			bundledLib = filepath.Join(tc.RootDir, "lib")
		}
	}

	compileArgs := func(src, obj string, extra ...string) []string {
		args := []string{"-c", optLevel, "-ffunction-sections", "-fdata-sections", "-Wno-override-module"}
		if !opts.Debug && strings.HasSuffix(src, ".c") {
			args = append(args, "-fvisibility=hidden", "-fno-unwind-tables", "-fno-asynchronous-unwind-tables")
		}
		if bundledInc != "" && strings.HasSuffix(src, ".c") {
			args = append(args, "-I"+bundledInc)
		}
		if tc.UseZigCC {
			if zigTriple != "" {
				args = append(args, "-target", zigTriple)
			}
		} else if triple != "" {
			args = append(args, "--target="+triple)
		}
		args = append(args, extra...)
		args = append(args, "-o", obj, src)
		return args
	}

	if err := runCC(compileArgs(llPath, progObjPath)...); err != nil {
		return err
	}
	if err := runCC(compileArgs(rtCPath, rtObjPath)...); err != nil {
		return err
	}

	linkArgs := []string{"-o", outputPath, progObjPath, rtObjPath}
	if bundledLib != "" {
		linkArgs = append(linkArgs, "-L"+bundledLib)
	}
	lldPath := tc.LLDPath
	runClang := runCC

	rtDir := filepath.Dir(rtCPath)
	needsDB := strings.Contains(llvmIR, "joss___joss_db_") || strings.Contains(llvmIR, "@fn_Schema_") || strings.Contains(llvmIR, "@fn_Blueprint_") || strings.Contains(llvmIR, "@fn_Model_")
	needsGUI := opts.GUI || strings.Contains(llvmIR, "@fn_Window_")
	needsCrypto := strings.Contains(llvmIR, "joss_sha256") || strings.Contains(llvmIR, "joss_password_") || strings.Contains(llvmIR, "joss_random_hex") || strings.Contains(llvmIR, "joss_jwt_") || strings.Contains(llvmIR, "joss_totp_") || strings.Contains(llvmIR, "joss_rsa_sign_")
	needsWeb := needsDB || needsGUI || llvmNeedsWebRuntime(llvmIR) || strings.Contains(llvmIR, "@joss_set_active_request") || strings.Contains(llvmIR, "@joss_register_embedded_asset_resolver")
	needsThreads := strings.Contains(llvmIR, "@joss_async") || strings.Contains(llvmIR, "@joss_make_chan") ||
		strings.Contains(llvmIR, "@fn_Mutex_") || strings.Contains(llvmIR, "@fn_RWMutex_") || strings.Contains(llvmIR, "@fn_WaitGroup_")
	needsConcurrency := needsThreads || strings.Contains(llvmIR, "@joss_dp_get") || strings.Contains(llvmIR, "@joss_dp_put")

	dbDrivers := detectNeededDBDrivers(llvmIR, needsDB)
	var dbDefines []string
	if needsDB {
		if !dbDrivers.sqlite {
			dbDefines = append(dbDefines, "-DJOSS_DB_DISABLE_SQLITE")
		}
		if !dbDrivers.postgres {
			dbDefines = append(dbDefines, "-DJOSS_DB_DISABLE_PG")
		}
		if !dbDrivers.mysql {
			dbDefines = append(dbDefines, "-DJOSS_DB_DISABLE_MYSQL")
		}
		if !dbDrivers.odbc {
			dbDefines = append(dbDefines, "-DJOSS_DB_DISABLE_ODBC")
		}
	}

	var webDefines []string
	if !needsCrypto {
		webDefines = append(webDefines, "-DJOSS_DISABLE_OPENSSL_SIGN")
	}

	optionalUnits := []struct {
		file    string
		include bool
		defines []string
	}{
		{"joss_web_rt.c", needsWeb, webDefines},
		{"joss_concurrency_rt.c", needsConcurrency, nil},
		{"joss_db_rt.c", needsDB, dbDefines},
		{"joss_crypto_rt.c", needsCrypto, nil},
		{"joss_gui_rt.c", needsGUI, nil},
	}
	for _, u := range optionalUnits {
		if !u.include {
			continue
		}
		srcPath := filepath.Join(rtDir, u.file)
		if _, statErr := os.Stat(srcPath); statErr == nil {
			objPath := filepath.Join(tmpDir, strings.TrimSuffix(u.file, ".c")+".o")
			if err := runClang(compileArgs(srcPath, objPath, u.defines...)...); err != nil {
				return err
			}
			linkArgs = append(linkArgs, objPath)
		}
	}
	for _, lib := range opts.StaticLibs {
		if lib != "" {
			linkArgs = append(linkArgs, lib)
		}
	}
	if opts.Target.OS == "linux" || opts.Target.OS == "android" || opts.Target.OS == "" {
		linkArgs = append(linkArgs, "-Wl,--as-needed")
	}
	hasHeader := func(rel string) bool {
		if bundledInc != "" {
			if _, err := os.Stat(filepath.Join(bundledInc, rel)); err == nil {
				return true
			}
		}
		if _, err := os.Stat(filepath.Join("/usr/include", rel)); err == nil {
			return true
		}
		return false
	}
	bundledSQLiteC := ""
	if needsDB && dbDrivers.sqlite {
		for _, cand := range []string{
			filepath.Join(rtDir, "sqlite3.c"),
			filepath.Join(bundledInc, "sqlite3.c"),
		} {
			if cand != "sqlite3.c" {
				if fi, statErr := os.Stat(cand); statErr == nil && !fi.IsDir() {
					bundledSQLiteC = cand
					break
				}
			}
		}
		if bundledSQLiteC != "" {
			sqliteObj := filepath.Join(tmpDir, "sqlite3.o")
			if err := runCC(compileArgs(bundledSQLiteC, sqliteObj, "-DSQLITE_THREADSAFE=1", "-DSQLITE_OMIT_LOAD_EXTENSION")...); err != nil {
				return err
			}
			linkArgs = append(linkArgs, sqliteObj)
		}
	}
	if needsDB {
		if dbDrivers.sqlite && bundledSQLiteC == "" && ((opts.Target.OS == "darwin" && runtime.GOOS == "darwin") || (opts.Target.OS == runtime.GOOS && hasHeader("sqlite3.h"))) {
			linkArgs = append(linkArgs, "-lsqlite3")
		}
		if dbDrivers.postgres && (hasHeader("postgresql/libpq-fe.h") || hasHeader("libpq-fe.h")) {
			linkArgs = append(linkArgs, "-lpq")
		}
		if dbDrivers.mysql {
			if hasHeader("mariadb/mysql.h") {
				linkArgs = append(linkArgs, "-lmariadb")
			} else if hasHeader("mysql/mysql.h") {
				linkArgs = append(linkArgs, "-lmysqlclient")
			}
		}
		if dbDrivers.odbc {
			if opts.Target.OS == "windows" {
				linkArgs = append(linkArgs, "-lodbc32")
			} else if hasHeader("sql.h") {
				linkArgs = append(linkArgs, "-lodbc")
			}
		}
	}
	if needsCrypto {
		if hasHeader("openssl/evp.h") {
			linkArgs = append(linkArgs, "-lcrypto")
		} else if opts.Target.OS == "windows" {
			linkArgs = append(linkArgs, "-lbcrypt", "-lcrypt32")
		}
	}
	if needsWeb && opts.Target.OS == "windows" {
		linkArgs = append(linkArgs, "-lws2_32", "-lbcrypt", "-lcrypt32")
	}
	if needsGUI {
		targetOS := opts.Target.OS
		if targetOS == "" {
			targetOS = runtime.GOOS
		}
		switch targetOS {
		case "windows":
			linkArgs = append(linkArgs, "-luser32", "-lole32", "-lshell32", "-ldwmapi", "-lshlwapi", "-ladvapi32")
			if opts.GUI {
				linkArgs = append(linkArgs, "-mwindows")
			}
		case "darwin":
			linkArgs = append(linkArgs, "-framework", "Cocoa", "-framework", "WebKit")
		case "linux":
			linkArgs = append(linkArgs, "-ldl")
		}
	}
	if (needsThreads || needsWeb || needsDB || needsGUI) && (opts.Target.OS == "linux" || opts.Target.OS == "") {
		linkArgs = append(linkArgs, "-pthread")
	}
	if !tc.UseZigCC && lldPath != "" && opts.Target.OS != "darwin" {
		linkArgs = append(linkArgs, "-fuse-ld=lld")
	}
	if opts.Target.OS == "linux" || opts.Target.OS == "android" || opts.Target.OS == "" {
		linkArgs = append(linkArgs, "-Wl,--gc-sections")
		if !opts.Debug {
			linkArgs = append(linkArgs, "-Wl,--strip-all")
			if lldPath != "" && !tc.UseZigCC {
				linkArgs = append(linkArgs, "-Wl,--icf=safe")
			}
		}
	} else if opts.Target.OS == "darwin" {
		linkArgs = append(linkArgs, "-Wl,-dead_strip")
		if !opts.Debug {
			linkArgs = append(linkArgs, "-Wl,-x")
		}
	}
	if tc.UseZigCC {
		if zigTriple != "" {
			linkArgs = append(linkArgs, "-target", zigTriple)
		}
	} else if triple != "" {
		linkArgs = append(linkArgs, "--target="+triple)
	}
	return runClang(linkArgs...)
}

type dbDriverSelection struct {
	sqlite   bool
	postgres bool
	mysql    bool
	odbc     bool
}

func detectNeededDBDrivers(llvmIR string, needsDB bool) dbDriverSelection {
	if !needsDB {
		return dbDriverSelection{}
	}
	envDrv := strings.ToLower(strings.TrimSpace(os.Getenv("DB_CONNECTION")))
	if envDrv == "" {
		envDrv = strings.ToLower(strings.TrimSpace(os.Getenv("DB_DRIVER")))
	}
	if envDrv == "" {
		envDrv = strings.ToLower(strings.TrimSpace(os.Getenv("DB")))
	}
	switch envDrv {
	case "sqlite", "sqlite3":
		return dbDriverSelection{sqlite: true}
	case "pgsql", "postgres", "postgresql", "pgx":
		return dbDriverSelection{postgres: true}
	case "mysql", "mariadb":
		return dbDriverSelection{mysql: true}
	case "sqlsrv", "mssql", "sqlserver", "odbc":
		return dbDriverSelection{odbc: true}
	}
	lowerIR := strings.ToLower(llvmIR)
	hasPG := strings.Contains(lowerIR, "postgres") || strings.Contains(lowerIR, "pgsql")
	hasMySQL := strings.Contains(lowerIR, "mysql") || strings.Contains(lowerIR, "mariadb")
	hasODBC := strings.Contains(lowerIR, "sqlsrv") || strings.Contains(lowerIR, "mssql") || strings.Contains(lowerIR, "odbc")
	hasSQLite := strings.Contains(lowerIR, "sqlite")
	if os.Getenv("DB_DATABASE") != "" {
		hasSQLite = true
	}
	if hasSQLite && !hasPG && !hasMySQL && !hasODBC {
		return dbDriverSelection{sqlite: true}
	}
	if hasPG || hasMySQL || hasODBC || hasSQLite {
		return dbDriverSelection{sqlite: true, postgres: hasPG, mysql: hasMySQL, odbc: hasODBC}
	}
	// Si no se puede determinar estáticamente, habilitar todos los disponibles
	return dbDriverSelection{sqlite: true, postgres: true, mysql: true, odbc: true}
}

func llvmNeedsWebRuntime(llvmIR string) bool {
	markers := []string{
		"@fn_Socket_", "@fn_Request_", "@fn_Cache_", "@fn_Markdown_",
		"@fn_Response_", "@fn_WebResponse_", "@fn_System_", "@fn_Http_",
		"@fn_WebSocket_", "@fn_AI_", "@fn_Notify_", "@fn_Plugin_",
		"@fn_Console_", "@fn_FileStream_", "@fn_StreamReader_", "@fn_StreamWriter_", "@fn_DateTime_", "@fn_DateInterval_", "@fn_Lang_", "@fn_Zip_",
		"@fn_Process_", "@fn_Blueprint_", "@joss_csrf_field",
		"@joss_microtime", "@joss_sleep", "@joss_usleep", "@joss_is_dir", "@joss_is_file",
		"@joss_rand", "@joss_ord", "@joss_round", "@joss_floor", "@joss_ceil",
		"@joss_config", "@joss_toon_",
		"@joss_strval", "@joss_to_string", "@joss_str_concat_i64",
		"@joss_strpos", "@joss_substr", "@joss_trim", "@joss_strtolower", "@joss_strtoupper",
		"@joss_urldecode", "@joss_rawurldecode", "@joss_urlencode", "@joss_explode", "@joss_count",
		"@joss_is_truthy", "@joss_is_array", "@joss_is_map", "@joss_is_string", "@joss_is_null",
		"@joss_boolval", "@joss_array_push", "@joss_floatval", "@joss_decimal",
		"@joss_sha1_base64", "@joss_sha256_base64", "@joss_md5", "@joss_gmdate", "@joss_rawurlencode", "@joss_rsa_sign_sha256",
		"@joss_time", "@joss_date", "@joss_strtotime",
		"@fn_Schema_", "@fn_Sitemap_", "@fn_Cron_", "@fn_IndexNow_", "@fn_Mail_", "@fn_SmtpClient_",
		"@joss_env", "@joss_file_", "@joss_json", "@joss_response", "@joss_redirect",
		"@joss_back", "@joss_view", "@joss_request", "@joss_session", "@joss_is_empty",
		"@joss_json_decode", "@joss_set_active_session", "@joss_get_active_session", "@joss_get_active_request",
		"@joss___joss_rt_", "@joss___joss_http_", "@joss___joss_get_embedded_",
		"@joss___joss_socket_", "@joss___joss_rate_", "@joss___joss_session_", "@joss___joss_jwt_",
		"@joss___joss_ws_", "@joss_register_plugin_dispatcher",
		"@joss___joss_byte_string", "@joss___joss_string_byte_at",
		"@joss___joss_local_utc_offset", "@joss___joss_timezone_data",
		"@with(", "@withCookie(", "@onSuccess(", "@onChallenge(", "@onFail(",
		"@response(", "@html_escape(", "@csrf_field(",
	}
	for _, m := range markers {
		if strings.Contains(llvmIR, m) {
			return true
		}
	}
	return false
}
