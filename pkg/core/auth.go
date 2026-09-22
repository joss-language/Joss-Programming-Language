package core

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// Auth Implementation
func (r *Runtime) executeAuthMethod(instance *Instance, method string, args []interface{}) interface{} {
	prefix := r.dbPrefix()
	usersTable := prefix + "users"
	rolesTable := prefix + "roles"

	fmt.Printf("[Auth Debug] Prefix: '%s', Users Table: '%s'\n", prefix, usersTable)

	// Asegurar que las tablas y columnas existan (Auto-Migración)
	r.ensureAuthTables(usersTable, rolesTable, prefix)

	switch method {
	case "hash":
		if len(args) >= 1 {
			hashedBytes, err := bcrypt.GenerateFromPassword([]byte(fmt.Sprintf("%v", args[0])), bcrypt.DefaultCost)
			if err != nil {
				return nil
			}
			return string(hashedBytes)
		}
		return nil

	case "complete2FA":
		if len(args) >= 1 {
			var userId int
			switch v := args[0].(type) {
			case int:
				userId = v
			case float64:
				userId = int(v)
			case int64:
				userId = int(v)
			default:
				fmt.Sscanf(fmt.Sprintf("%v", v), "%d", &userId)
			}

			if r.GetDB() == nil {
				return nil
			}

			var email, username, roleName sql.NullString
			query := fmt.Sprintf(`
				SELECT u.email, u.username, r.name 
				FROM %s u 
				LEFT JOIN %s r ON u.role_id = r.id 
				WHERE u.id = ?`, usersTable, rolesTable)

			err := r.databaseExecutor().QueryRow(query, userId).Scan(&email, &username, &roleName)
			if err == nil {
				return r.generateJWT(userId, email.String, username.String, roleName.String, false)
			}
			return nil
		}
		return nil

	case "verify2FAChallenge":
		if len(args) >= 2 {
			tokenString := strings.TrimSpace(fmt.Sprintf("%v", args[0]))
			code := strings.TrimSpace(fmt.Sprintf("%v", args[1]))
			if strings.HasPrefix(tokenString, "Bearer ") {
				tokenString = strings.TrimSpace(strings.TrimPrefix(tokenString, "Bearer "))
			}
			if tokenString == "" || code == "" {
				return false
			}

			claims, valid := r.parseJWT(tokenString)
			if !valid || fmt.Sprintf("%v", claims["token_type"]) != "mfa_challenge" {
				return false
			}

			userId := claimUserID(claims["user_id"])
			if userId <= 0 {
				return false
			}
			verified := r.executeTwoFactorMethod(nil, "verify", []interface{}{userId, code})
			if verified != true {
				return false
			}
			challengeID := strings.TrimSpace(fmt.Sprintf("%v", claims["jti"]))
			if challengeID == "" {
				return false
			}
			if _, alreadyUsed := usedMFAChallenges.LoadOrStore(challengeID, time.Now()); alreadyUsed {
				return false
			}
			return r.executeAuthMethod(instance, "complete2FA", []interface{}{userId})
		}
		return false

	case "login":
		if len(args) >= 2 {
			email := normalizeAuthEmail(fmt.Sprintf("%v", args[0]))
			password := fmt.Sprintf("%v", args[1])

			resultFields := make(map[string]interface{})
			resultFields["email"] = email
			resultFields["password"] = password
			resultFields["runtime"] = r
			resultFields["requires_2fa"] = false

			jwt := r.executeAuthMethod(instance, "attempt", []interface{}{email, password})
			if jwtVal, ok := jwt.(string); ok && jwtVal != "" {
				resultFields["success"] = true
				resultFields["jwt"] = jwtVal
				var userId int
				query := fmt.Sprintf("SELECT id FROM %s WHERE email = ?", usersTable)
				err := r.databaseExecutor().QueryRow(query, email).Scan(&userId)
				if err == nil {
					resultFields["user_id"] = userId
				}
			} else {
				resultFields["success"] = false
				resultFields["error"] = "Credenciales incorrectas o cuenta no verificada"
			}

			return &Instance{
				Class:  r.Classes["AuthLoginResult"],
				Fields: resultFields,
			}
		}
		return nil

	case "create":
		if len(args) > 0 {
			if data, ok := args[0].(map[string]interface{}); ok {
				userToken := uuid.New().String()

				username := getString(data, "username", "")
				firstName := getString(data, "first_name", "")
				lastName := getString(data, "last_name", "")
				email := normalizeAuthEmail(getString(data, "email", ""))
				phone := getString(data, "phone", "")
				password := getString(data, "password", "")

				roleId := 2
				if rId, ok := data["role_id"].(int64); ok {
					roleId = int(rId)
				}

				hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
				if err != nil {
					panic(fmt.Sprintf("Auth Error: Fallo al encriptar contraseña: %v", err))
				}
				hashedPassword := string(hashedBytes)

				if r.GetDB() == nil {
					panic("Auth Error: No hay conexión a la base de datos configurada")
				}

				tokenExpires := time.Now().UTC().Add(24 * time.Hour).Format("2006-01-02 15:04:05")

				insertData := map[string]interface{}{
					"user_token":       userToken,
					"username":         username,
					"first_name":       firstName,
					"last_name":        lastName,
					"email":            email,
					"phone":            phone,
					"password":         hashedPassword,
					"role_id":          roleId,
					"token_expires_at": tokenExpires,
					"verificado":       0,
				}

				insertResult := r.insertFromMap(usersTable, insertData, false)
				if insertResult != nil && insertResult != false {
					fmt.Println("[Security] Usuario registrado exitosamente.")
					return userToken
				}
				fmt.Println("[Security] Error creando usuario.")
				return false
			}
		}

	case "attempt":
		if len(args) >= 2 {
			if args[0] == nil || args[1] == nil {
				LogError("[Auth] Attempt failed: Email or Password is nil")
				return false
			}
			email := normalizeAuthEmail(fmt.Sprintf("%v", args[0]))
			password := args[1].(string)

			if r.GetDB() == nil {
				panic("Auth Error: No hay conexión a la base de datos configurada")
			}

			var storedHash sql.NullString
			var userId int
			var userName sql.NullString
			var userToken sql.NullString
			var roleName sql.NullString
			var verificado int

			query := fmt.Sprintf(`
				SELECT u.id, u.user_token, u.username, u.password, u.verificado, r.name 
				FROM %s u 
				LEFT JOIN %s r ON u.role_id = r.id 
				WHERE u.email = ?`, usersTable, rolesTable)

			err := r.databaseExecutor().QueryRow(query, email).Scan(&userId, &userToken, &userName, &storedHash, &verificado, &roleName)
			if err != nil {
				if err == sql.ErrNoRows {
					LogError("[Auth] User not found for email: '%s'", email)
				} else {
					LogError("[Auth] Database error looking up '%s': %v", email, err)
				}
				return false
			}

			if verificado == 0 {
				LogError("[Auth] Account not verified for '%s'", email)
				return false
			}

			err = bcrypt.CompareHashAndPassword([]byte(storedHash.String), []byte(password))
			if err != nil {
				LogError("[Auth] Password mismatch for '%s'", email)
				return false
			}

			LogInfo("[Auth] Login successful for '%s' (ID: %d)", email, userId)

			if sessVal, ok := r.Variables["$__session"]; ok {
				if sessInst, ok := sessVal.(*Instance); ok {
					sessInst.Fields["user_id"] = userId
					sessInst.Fields["user_token"] = userToken.String
					sessInst.Fields["user_name"] = userName.String
					sessInst.Fields["user_email"] = email
					sessInst.Fields["user_role"] = roleName.String
					sessInst.Fields["last_login_at"] = time.Now().Format("2006-01-02 15:04:05")
				}
			}

			updateQuery := fmt.Sprintf("UPDATE %s SET last_login_at = %s WHERE id = ?", usersTable, "CURRENT_TIMESTAMP")
			if val, ok := r.Env["DB"]; ok && val == "mysql" {
				updateQuery = fmt.Sprintf("UPDATE %s SET last_login_at = NOW() WHERE id = ?", usersTable)
			}
			r.databaseExecutor().Exec(updateQuery, userId)

			return r.generateJWT(userId, email, userName.String, roleName.String, false)
		}

	case "check":
		if sessVal, ok := r.Variables["$__session"]; ok {
			if sessInst, ok := sessVal.(*Instance); ok {
				if _, ok := sessInst.Fields["user_id"]; ok {
					return true
				}
			}
		}
		return false

	case "verify":
		if len(args) == 1 {
			token := strings.TrimSpace(fmt.Sprintf("%v", args[0]))
			if token == "" {
				return false
			}
			if r.GetDB() == nil {
				return false
			}
			var id int
			var expiresAtStr sql.NullString

			query := fmt.Sprintf("SELECT id, token_expires_at FROM %s WHERE user_token = ? AND verificado = 0 LIMIT 1", usersTable)
			err := r.databaseExecutor().QueryRow(query, token).Scan(&id, &expiresAtStr)

			if err != nil {
				return false
			}

			if expiresAtStr.Valid && expiresAtStr.String != "" {
				expiryTime, ok := parseAuthExpiry(expiresAtStr.String)
				if !ok || time.Now().After(expiryTime) {
					return false
				}
			}

			update := fmt.Sprintf("UPDATE %s SET verificado = 1, user_token = '', token_expires_at = NULL WHERE id = ?", usersTable)
			_, err = r.databaseExecutor().Exec(update, id)

			if err == nil {
				return true
			}
			return false
		}

	case "forgotPassword":
		if len(args) == 1 {
			email := normalizeAuthEmail(fmt.Sprintf("%v", args[0]))
			if email == "" {
				return false
			}
			if r.GetDB() == nil {
				return false
			}

			var userId int
			queryCheck := fmt.Sprintf("SELECT id FROM %s WHERE LOWER(email) = ?", usersTable)
			err := r.databaseExecutor().QueryRow(queryCheck, email).Scan(&userId)
			if err != nil {
				return false
			}

			resetsTable := prefix + "password_resets"
			var existingToken string
			var existingExpiry sql.NullString
			existingQuery := fmt.Sprintf("SELECT token, expires_at FROM %s WHERE LOWER(email) = ? AND used = 0 ORDER BY id DESC LIMIT 1", resetsTable)
			if err = r.databaseExecutor().QueryRow(existingQuery, email).Scan(&existingToken, &existingExpiry); err == nil {
				if expiry, ok := parseAuthExpiry(existingExpiry.String); ok && time.Now().Before(expiry) {
					return existingToken
				}
			}

			token := uuid.New().String()
			expiresAt := time.Now().UTC().Add(1 * time.Hour).Format("2006-01-02 15:04:05")

			query := fmt.Sprintf("INSERT INTO %s (email, token, expires_at) VALUES (?, ?, ?)", resetsTable)
			_, err = r.databaseExecutor().Exec(query, email, token, expiresAt)

			if err == nil {
				return token
			}
			LogError("[Auth] Could not create password reset token: %v", err)
		}
		return false

	case "resetPassword":
		if len(args) == 2 {
			token := strings.TrimSpace(fmt.Sprintf("%v", args[0]))
			newPass := fmt.Sprintf("%v", args[1])
			if token == "" {
				return "invalid_token"
			}
			if len(newPass) < 8 {
				return "weak_password"
			}

			if r.GetDB() == nil {
				return "database_error"
			}
			if r.activeTx != nil {
				return "database_error"
			}

			resetsTable := prefix + "password_resets"

			var email string
			var expiresAtStr sql.NullString
			var used int

			tx, err := r.GetDB().Begin()
			if err != nil {
				return "database_error"
			}
			defer tx.Rollback()

			query := fmt.Sprintf("SELECT email, expires_at, used FROM %s WHERE token = ? LIMIT 1", resetsTable)
			err = tx.QueryRow(query, token).Scan(&email, &expiresAtStr, &used)

			if err != nil {
				return "invalid_token"
			}

			if used == 1 {
				return "used_token"
			}

			if expiresAtStr.Valid && expiresAtStr.String != "" {
				expiryTime, ok := parseAuthExpiry(expiresAtStr.String)
				if !ok || time.Now().After(expiryTime) {
					return "expired_token"
				}
			}

			hashedBytes, err := bcrypt.GenerateFromPassword([]byte(newPass), bcrypt.DefaultCost)
			if err != nil {
				return "database_error"
			}
			hashedPassword := string(hashedBytes)

			claimToken := fmt.Sprintf("UPDATE %s SET used = 1 WHERE token = ? AND used = 0", resetsTable)
			claimResult, err := tx.Exec(claimToken, token)
			if err != nil {
				return "database_error"
			}
			claimed, err := claimResult.RowsAffected()
			if err != nil || claimed != 1 {
				return "used_token"
			}

			updUser := fmt.Sprintf("UPDATE %s SET password = ?, verificado = 1, user_token = '', token_expires_at = NULL WHERE LOWER(email) = ?", usersTable)
			userResult, err := tx.Exec(updUser, hashedPassword, normalizeAuthEmail(email))
			if err != nil {
				return "database_error"
			}
			updated, err := userResult.RowsAffected()
			if err != nil || updated != 1 {
				return "invalid_token"
			}
			if err = tx.Commit(); err != nil {
				return "database_error"
			}

			return true
		}

	case "resendVerification":
		if len(args) == 1 {
			email := normalizeAuthEmail(fmt.Sprintf("%v", args[0]))
			if email == "" {
				return false
			}
			if r.GetDB() == nil {
				return false
			}

			var id int
			var verificado int
			var currentToken sql.NullString
			var currentExpiry sql.NullString
			query := fmt.Sprintf("SELECT id, verificado, user_token, token_expires_at FROM %s WHERE LOWER(email) = ?", usersTable)
			err := r.databaseExecutor().QueryRow(query, email).Scan(&id, &verificado, &currentToken, &currentExpiry)

			if err != nil {
				return false
			}

			if verificado == 1 {
				return false
			}

			if currentToken.Valid && strings.TrimSpace(currentToken.String) != "" && currentExpiry.Valid {
				if expiry, ok := parseAuthExpiry(currentExpiry.String); ok && time.Now().Before(expiry) {
					return currentToken.String
				}
			}

			newToken := uuid.New().String()
			newExpiry := time.Now().UTC().Add(24 * time.Hour).Format("2006-01-02 15:04:05")

			update := fmt.Sprintf("UPDATE %s SET user_token = ?, token_expires_at = ? WHERE id = ?", usersTable)
			_, err = r.databaseExecutor().Exec(update, newToken, newExpiry, id)

			if err == nil {
				return newToken
			}
		}
		return false

	case "verificationStatus":
		if len(args) == 1 {
			email := normalizeAuthEmail(fmt.Sprintf("%v", args[0]))
			if email == "" || r.GetDB() == nil {
				return "not_found"
			}

			var verified int
			query := fmt.Sprintf("SELECT verificado FROM %s WHERE LOWER(email) = ? LIMIT 1", usersTable)
			err := r.databaseExecutor().QueryRow(query, email).Scan(&verified)
			if err != nil {
				return "not_found"
			}
			if verified == 1 {
				return "verified"
			}
			return "unverified"
		}
		return "not_found"

	case "user":
		if sessVal, ok := r.Variables["$__session"]; ok {
			if sessInst, ok := sessVal.(*Instance); ok {
				if uid, ok := sessInst.Fields["user_id"]; ok {
					if r.GetDB() == nil {
						return nil
					}

					user := make(map[string]interface{})

					var id, roleId int
					var username, email, firstName, lastName, userToken, createdAt, roleName sql.NullString
					var pPhone sql.NullString

					query := fmt.Sprintf(`SELECT u.id, u.username, u.first_name, u.last_name, u.email, u.phone, u.role_id, r.name, u.user_token, u.created_at 
						FROM %s u 
						LEFT JOIN %s r ON u.role_id = r.id 
						WHERE u.id = ?`, usersTable, rolesTable)

					err := r.databaseExecutor().QueryRow(query, uid).Scan(&id, &username, &firstName, &lastName, &email, &pPhone, &roleId, &roleName, &userToken, &createdAt)
					if err != nil {
						fmt.Printf("[Auth Error] User Query Failed for ID %v: %v\n", uid, err)
					}
					if err == nil {
						user["id"] = id
						user["username"] = username.String
						user["first_name"] = firstName.String
						user["last_name"] = lastName.String
						user["full_name"] = firstName.String + " " + lastName.String
						user["email"] = email.String
						user["phone"] = pPhone.String
						user["role_id"] = roleId
						user["role"] = roleName.String
						user["user_token"] = userToken.String
						user["created_at"] = createdAt.String
						user["name"] = firstName.String

						fmt.Printf("[Auth] User found: %s (ID: %d)\n", firstName.String, id)

						return &Instance{
							Fields: user,
						}
					}
				}
			}
		}
		return nil

	case "guest":
		if sessVal, ok := r.Variables["$__session"]; ok {
			if sessInst, ok := sessVal.(*Instance); ok {
				if _, ok := sessInst.Fields["user_id"]; ok {
					return false
				}
			}
		}
		return true

	case "hasRole":
		if len(args) == 1 {
			roleToCheck := args[0].(string)
			if sessVal, ok := r.Variables["$__session"]; ok {
				if sessInst, ok := sessVal.(*Instance); ok {
					if currentRole, ok := sessInst.Fields["user_role"]; ok {
						if currentRole == roleToCheck {
							return true
						}
						if currentRole == "admin" {
							return true
						}
					}
				}
			}
		}
		return false

	case "id":
		if sessVal, ok := r.Variables["$__session"]; ok {
			if sessInst, ok := sessVal.(*Instance); ok {
				if uid, ok := sessInst.Fields["user_id"]; ok {
					return uid
				}
			}
		}
		return nil

	case "refresh":
		if len(args) == 1 {
			id := toInt(args[0])
			if id > 0 {
				var email, username, roleName string
				prefix := r.dbPrefix()
				usersTable := prefix + "users"
				rolesTable := prefix + "roles"

				query := fmt.Sprintf(`
					SELECT u.email, u.username, r.name 
					FROM %s u 
					LEFT JOIN %s r ON u.role_id = r.id 
					WHERE u.id = ?`, usersTable, rolesTable)

				err := r.databaseExecutor().QueryRow(query, id).Scan(&email, &username, &roleName)
				if err != nil {
					return false
				}
				return r.generateJWT(id, email, username, roleName, false)
			}
		}

	case "update":
		if len(args) == 2 {
			id := toInt(args[0])
			data, ok2 := args[1].(map[string]interface{})

			if id > 0 && ok2 {
				if r.GetDB() == nil {
					return false
				}

				if pwd, ok := data["password"]; ok {
					passwordStr := fmt.Sprintf("%v", pwd)
					if passwordStr != "" {
						hashedBytes, err := bcrypt.GenerateFromPassword([]byte(passwordStr), bcrypt.DefaultCost)
						if err == nil {
							data["password"] = string(hashedBytes)
						}
					} else {
						delete(data, "password")
					}
				}

				var sets []string
				var vals []interface{}

				if val, ok := r.Env["DB"]; ok && val == "sqlite" {
					sets = append(sets, "updated_at = CURRENT_TIMESTAMP")
				} else {
					sets = append(sets, "updated_at = NOW()")
				}

				for k, v := range data {
					if k != "id" && k != "user_token" && k != "created_at" && k != "updated_at" {
						sets = append(sets, fmt.Sprintf("%s = ?", k))
						vals = append(vals, v)
					}
				}
				vals = append(vals, id)

				if len(sets) == 0 {
					return true
				}

				query := fmt.Sprintf("UPDATE %s SET %s WHERE id = ?", usersTable, strings.Join(sets, ", "))
				_, err := r.databaseExecutor().Exec(query, vals...)
				return err == nil
			}
		}

	case "delete":
		if len(args) == 1 {
			id := toInt(args[0])
			if id > 0 {
				if r.GetDB() == nil {
					return false
				}
				query := fmt.Sprintf("DELETE FROM %s WHERE id = ?", usersTable)
				_, err := r.databaseExecutor().Exec(query, id)
				return err == nil
			}
		}

	case "logout":
		if sessVal, ok := r.Variables["$__session"]; ok {
			if sessInst, ok := sessVal.(*Instance); ok {
				delete(sessInst.Fields, "user_id")
				delete(sessInst.Fields, "user_token")
				delete(sessInst.Fields, "user_name")
				delete(sessInst.Fields, "user_email")
				delete(sessInst.Fields, "user_role")
				delete(sessInst.Fields, "last_login_at")
			}
		}
		return true

	case "validateToken":
		if len(args) == 1 {
			tokenString := args[0].(string)
			if len(tokenString) > 7 && tokenString[:7] == "Bearer " {
				tokenString = tokenString[7:]
			}

			claims, valid := r.ValidateJWT(tokenString)
			if valid {
				if sessVal, ok := r.Variables["$__session"]; ok {
					if sessInst, ok := sessVal.(*Instance); ok {
						sessInst.Fields["user_id"] = int(claims["user_id"].(float64))
						sessInst.Fields["user_email"] = claims["email"]
						sessInst.Fields["user_name"] = claims["name"]
						sessInst.Fields["user_role"] = claims["role"]
					}
				}
				return true
			}
			return false
		}
	}
	return nil
}
