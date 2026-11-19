package llvm

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jossecurity/joss/pkg/backend/native"
)

func init() {
	clangPath, _ := exec.LookPath("clang")
	if clangPath == "" {
		return
	}
	lldPath, _ := exec.LookPath("ld.lld")
	if lldPath == "" {
		lldPath, _ = exec.LookPath("lld")
	}
	native.RegisterExternalLLVMCompiler(func(llvmIR string, outputPath string, opts native.BuildOptions) error {
		return compileLLVMWithOptions(clangPath, lldPath, llvmIR, outputPath, opts)
	})
}

func findJossRuntimeC(hint string) (string, error) {
	if hint != "" {
		if _, err := os.Stat(hint); err == nil {
			return hint, nil
		}
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := cwd
	for i := 0; i < 6; i++ {
		candidate := filepath.Join(dir, "runtime", "native", "joss_rt.c")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("no se encontró runtime/native/joss_rt.c desde %s", cwd)
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

func compileLLVMWithOptions(clangPath, lldPath, llvmIR, outputPath string, opts native.BuildOptions) error {
	rtCPath, err := findJossRuntimeC(opts.RuntimeSource)
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

	runClang := func(args ...string) error {
		var stderr bytes.Buffer
		cmd := exec.Command(clangPath, args...)
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("clang %v failed: %w\n%s", args, err, stderr.String())
		}
		return nil
	}

	compileArgs := func(src, obj string, extra ...string) []string {
		args := []string{"-c", optLevel, "-ffunction-sections", "-fdata-sections", "-Wno-override-module"}
		if !opts.Debug && strings.HasSuffix(src, ".c") {
			args = append(args, "-fvisibility=hidden", "-fno-unwind-tables", "-fno-asynchronous-unwind-tables")
		}
		if triple != "" {
			args = append(args, "--target="+triple)
		}
		args = append(args, extra...)
		args = append(args, "-o", obj, src)
		return args
	}

	if err := runClang(compileArgs(llPath, progObjPath)...); err != nil {
		return err
	}
	if err := runClang(compileArgs(rtCPath, rtObjPath)...); err != nil {
		return err
	}

	linkArgs := []string{"-o", outputPath, progObjPath, rtObjPath}

	rtDir := filepath.Dir(rtCPath)
	needsDB := strings.Contains(llvmIR, "joss___joss_db_")
	needsCrypto := strings.Contains(llvmIR, "joss_sha256") || strings.Contains(llvmIR, "joss_password_") || strings.Contains(llvmIR, "joss_random_hex") || strings.Contains(llvmIR, "joss_jwt_") || strings.Contains(llvmIR, "joss_totp_")
	needsWeb := needsDB || llvmNeedsWebRuntime(llvmIR)
	needsThreads := strings.Contains(llvmIR, "@joss_async") || strings.Contains(llvmIR, "@joss_make_chan")
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

	optionalUnits := []struct {
		file    string
		include bool
		defines []string
	}{
		{"joss_web_rt.c", needsWeb, nil},
		{"joss_concurrency_rt.c", needsConcurrency, nil},
		{"joss_db_rt.c", needsDB, dbDefines},
		{"joss_crypto_rt.c", needsCrypto, nil},
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
	if needsDB {
		if dbDrivers.sqlite {
			if _, err := os.Stat("/usr/include/sqlite3.h"); err == nil {
				linkArgs = append(linkArgs, "-lsqlite3")
			}
		}
		if dbDrivers.postgres {
			if _, err := os.Stat("/usr/include/postgresql/libpq-fe.h"); err == nil {
				linkArgs = append(linkArgs, "-lpq")
			} else if _, err := os.Stat("/usr/include/libpq-fe.h"); err == nil {
				linkArgs = append(linkArgs, "-lpq")
			}
		}
		if dbDrivers.mysql {
			if _, err := os.Stat("/usr/include/mariadb/mysql.h"); err == nil {
				linkArgs = append(linkArgs, "-lmariadb")
			} else if _, err := os.Stat("/usr/include/mysql/mysql.h"); err == nil {
				linkArgs = append(linkArgs, "-lmysqlclient")
			}
		}
		if dbDrivers.odbc {
			if _, err := os.Stat("/usr/include/sql.h"); err == nil {
				linkArgs = append(linkArgs, "-lodbc")
			}
		}
	}
	if needsCrypto {
		if _, err := os.Stat("/usr/include/openssl/evp.h"); err == nil {
			linkArgs = append(linkArgs, "-lcrypto")
		}
	}
	if needsThreads && (opts.Target.OS == "linux" || opts.Target.OS == "") {
		linkArgs = append(linkArgs, "-pthread")
	}
	if lldPath != "" {
		linkArgs = append(linkArgs, "-fuse-ld=lld")
	}
	if opts.Target.OS == "linux" || opts.Target.OS == "android" || opts.Target.OS == "" {
		linkArgs = append(linkArgs, "-Wl,--gc-sections")
		if !opts.Debug {
			linkArgs = append(linkArgs, "-Wl,--strip-all")
			if lldPath != "" {
				linkArgs = append(linkArgs, "-Wl,--icf=safe")
			}
		}
	} else if opts.Target.OS == "darwin" {
		linkArgs = append(linkArgs, "-Wl,-dead_strip")
		if !opts.Debug {
			linkArgs = append(linkArgs, "-Wl,-x")
		}
	}
	if triple != "" {
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
		return dbDriverSelection{sqlite: hasSQLite, postgres: hasPG, mysql: hasMySQL, odbc: hasODBC}
	}
	// Si no se puede determinar estáticamente, habilitar todos los disponibles
	return dbDriverSelection{sqlite: true, postgres: true, mysql: true, odbc: true}
}

func llvmNeedsWebRuntime(llvmIR string) bool {
	markers := []string{
		"@fn_Socket_", "@fn_Request_",
		"@joss_strval", "@joss_to_string", "@joss_str_concat_i64",
		"@joss_strpos", "@joss_substr", "@joss_trim", "@joss_strtolower", "@joss_strtoupper",
		"@joss_urldecode", "@joss_rawurldecode", "@joss_explode", "@joss_count",
		"@joss_is_truthy", "@joss_is_array", "@joss_is_map", "@joss_is_string", "@joss_is_null",
		"@joss_boolval", "@joss_array_push", "@joss_floatval", "@joss_decimal",
		"@joss_sha1_base64", "@joss_time", "@joss_date", "@joss_strtotime",
		"@joss_env", "@joss_file_", "@joss_json", "@joss_response", "@joss_redirect",
		"@joss_back", "@joss_view", "@joss_request", "@joss_session", "@joss_is_empty",
		"@joss___joss_rt_", "@joss___joss_http_", "@joss___joss_get_embedded_",
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
