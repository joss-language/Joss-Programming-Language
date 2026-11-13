package core

import (
	"database/sql"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/jossecurity/joss/pkg/parser"
	"github.com/jossecurity/joss/std"
)

// TestPureJossAuthFlowsMySQL exercises the Joss Auth policy against a disposable
// MySQL schema. It deliberately refuses databases whose name does not identify
// them as a test database and never opens the application's configured DB.
func TestPureJossAuthFlowsMySQL(t *testing.T) {
	dsn := os.Getenv("JOSS_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set JOSS_TEST_MYSQL_DSN to a disposable MySQL test database")
	}
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("JOSS_TEST_MYSQL_DSN is invalid: %v", err)
	}
	if !strings.Contains(strings.ToLower(cfg.DBName), "test") {
		t.Fatalf("refusing Auth integration tests: MySQL database name must contain 'test'")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(8)
	if err := db.Ping(); err != nil {
		t.Fatalf("cannot connect to disposable MySQL test database: %v", err)
	}

	prefix := "joss_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:24] + "_"
	tables := []string{"password_resets", "user_social_accounts", "user_mfa_challenges", "user_recovery_codes", "user_mfa_methods", "security_logs", "users", "roles"}
	for _, table := range tables {
		name := prefix + table
		if _, err := db.Exec("DROP TABLE IF EXISTS " + name); err != nil {
			t.Fatalf("prepare disposable table %s: %v", table, err)
		}
	}
	defer func() {
		for _, table := range tables {
			_, _ = db.Exec("DROP TABLE IF EXISTS " + prefix + table)
		}
	}()

	runtime := NewRuntime()
	runtime.DB = db
	runtime.Env = map[string]string{"DB": "mysql", "PREFIX": prefix, "JWT_SECRET": "disposable-auth-test-secret"}
	runtime.Variables["$__session"] = &Instance{Fields: map[string]interface{}{}}
	defer runtime.Free()
	installJossAuthFlows(t, runtime)
	if ready := runtime.executeAuthMethod(nil, "databaseReady", nil); ready != true {
		t.Fatalf("Auth::databaseReady failed to prepare the fresh disposable MySQL schema: %#v", ready)
	}
	for _, table := range []string{"roles", "users", "password_resets", "user_social_accounts"} {
		var exists string
		if err := db.QueryRow("SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?", prefix+table).Scan(&exists); err != nil {
			t.Fatalf("Auth::databaseReady did not create %s: %v", prefix+table, err)
		}
	}
	if hash := runtime.executeAuthMethod(nil, "hash", []interface{}{"correct horse battery staple"}); hash == nil {
		t.Fatal("Auth::hash primitive failed before registration")
	}
	create := authMethod(t, runtime, "create")
	data := map[string]interface{}{
		"username":   "auth_" + prefix,
		"first_name": "Ada",
		"last_name":  "Lovelace",
		"email":      strings.ToUpper("ada_" + prefix + "@example.test"),
		"password":   "correct horse battery staple",
		"phone":      "",
		"role_id":    int64(1), // untrusted input must not grant the admin role
	}
	tokenValue := runtime.CallMethodEvaluated(create, nil, []interface{}{data})
	verificationToken, ok := tokenValue.(string)
	if !ok || verificationToken == "" {
		var inserted int
		_ = db.QueryRow("SELECT count(*) FROM " + prefix + "users").Scan(&inserted)
		t.Fatalf("Auth::create returned %#v, want a verification token (users inserted: %d)", tokenValue, inserted)
	}
	var assignedRole int64
	if err := db.QueryRow("SELECT role_id FROM "+prefix+"users WHERE user_token = ?", verificationToken).Scan(&assignedRole); err != nil {
		t.Fatal(err)
	}
	if assignedRole != 2 {
		t.Fatalf("Auth::create accepted caller-selected role_id=%d; public registration must assign client role 2", assignedRole)
	}
	if got := runtime.CallMethodEvaluated(create, nil, []interface{}{data}); got != false {
		t.Fatalf("duplicate Auth::create returned %#v, want false", got)
	}
	weak := map[string]interface{}{"username": "weak_" + prefix, "email": "weak_" + prefix + "@example.test", "password": "short"}
	if got := runtime.CallMethodEvaluated(create, nil, []interface{}{weak}); got != false {
		t.Fatalf("weak-password Auth::create returned %#v, want false", got)
	}

	var userID int64
	if err := db.QueryRow("SELECT id FROM "+prefix+"users WHERE user_token = ?", verificationToken).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("UPDATE "+prefix+"users SET verificado = 1 WHERE id = ?", userID); err != nil {
		t.Fatal(err)
	}
	login := authMethod(t, runtime, "login")
	loginValue := runtime.CallMethodEvaluated(login, nil, []interface{}{strings.ToLower(data["email"].(string)), data["password"]})
	loginResult, ok := loginValue.(*Instance)
	if !ok || loginResult.Fields["success"] != true {
		t.Fatalf("valid Auth::login returned %#v, want successful AuthLoginResult", loginValue)
	}
	if role := runtime.Variables["$__session"].(*Instance).Fields["user_role"]; role != "client" {
		t.Fatalf("login session role = %#v, want client", role)
	}
	if bad := runtime.CallMethodEvaluated(login, nil, []interface{}{data["email"], "wrong password"}); bad.(*Instance).Fields["success"] != false {
		t.Fatalf("invalid credentials were accepted: %#v", bad)
	}

	forgot := authMethod(t, runtime, "forgotPassword")
	resetTokenValue := runtime.CallMethodEvaluated(forgot, nil, []interface{}{data["email"]})
	resetToken, ok := resetTokenValue.(string)
	if !ok || resetToken == "" {
		t.Fatalf("existing account reset request = %#v, want token", resetTokenValue)
	}
	if got := runtime.CallMethodEvaluated(forgot, nil, []interface{}{"missing_" + prefix + "@example.test"}); got != false {
		t.Fatalf("unknown account reset request = %#v, want no token", got)
	}
	reset := authMethod(t, runtime, "resetPassword")
	if got := runtime.CallMethodEvaluated(reset, nil, []interface{}{resetToken, "short"}); got != "weak_password" {
		t.Fatalf("weak reset password = %#v, want weak_password", got)
	}
	if got := runtime.CallMethodEvaluated(reset, nil, []interface{}{resetToken, "new strong password"}); got != true {
		t.Fatalf("valid password reset = %#v, want true", got)
	}
	if got := runtime.CallMethodEvaluated(reset, nil, []interface{}{resetToken, "another strong password"}); got != "used_token" {
		t.Fatalf("reused reset token = %#v, want used_token", got)
	}
	var updatedHash string
	if err := db.QueryRow("SELECT password FROM "+prefix+"users WHERE id = ?", userID).Scan(&updatedHash); err != nil {
		t.Fatal(err)
	}
	if verified := runtime.executeAuthMethod(nil, "verifyHash", []interface{}{updatedHash, "new strong password"}); verified != true {
		t.Fatal("password reset did not store the new bcrypt hash")
	}

	expiredAt := time.Now().Add(-time.Hour).Format("2006-01-02 15:04:05")
	if _, err := db.Exec("INSERT INTO "+prefix+"password_resets (email, token, expires_at, used) VALUES (?, ?, ?, 0)", data["email"], "expired_"+verificationToken, expiredAt); err != nil {
		t.Fatal(err)
	}
	if got := runtime.CallMethodEvaluated(reset, nil, []interface{}{"expired_" + verificationToken, "another strong password"}); got != "expired_token" {
		t.Fatalf("expired reset token = %#v, want expired_token", got)
	}

	concurrentToken := "race_" + verificationToken
	concurrentExpiry := time.Now().Add(time.Hour).Format("2006-01-02 15:04:05")
	if _, err := db.Exec("INSERT INTO "+prefix+"password_resets (email, token, expires_at, used) VALUES (?, ?, ?, 0)", data["email"], concurrentToken, concurrentExpiry); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan interface{}, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			isolatedRuntime := NewRuntime()
			defer isolatedRuntime.Free()
			isolatedRuntime.DB = db
			isolatedRuntime.Env = runtime.Env
			results <- isolatedRuntime.executeAuthMethod(nil, "claimPasswordReset", []interface{}{concurrentToken, updatedHash})
		}()
	}
	wg.Wait()
	close(results)
	var successes, usedTokens int
	var unexpected []interface{}
	for result := range results {
		if result == true {
			successes++
		}
		if result == "used_token" {
			usedTokens++
		}
		if result != true && result != "used_token" {
			unexpected = append(unexpected, result)
		}
	}
	if successes != 1 || usedTokens != 1 {
		t.Fatalf("concurrent reset claims: successes=%d used=%d unexpected=%#v, want one of each", successes, usedTokens, unexpected)
	}
}

func installJossAuthFlows(t *testing.T, runtime *Runtime) {
	t.Helper()
	wanted := map[string]bool{"Auth": false, "AuthLoginResult": false, "GranDB": false}
	for _, unit := range std.GetStdlibUnits() {
		if unit.Program == nil {
			continue
		}
		for _, statement := range unit.Program.Statements {
			class, ok := statement.(*parser.ClassStatement)
			if !ok || class.Name == nil {
				continue
			}
			if _, exists := wanted[class.Name.Value]; exists {
				runtime.registerClass(class)
				wanted[class.Name.Value] = true
			}
		}
	}
	for name, installed := range wanted {
		if !installed {
			t.Fatalf("required Pure Joss Auth class %s was not embedded", name)
		}
	}
}
