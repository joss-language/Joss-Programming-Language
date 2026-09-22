package core

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"math/rand"
	"net/url"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// executeAuthLoginResultMethod handles the fluent login builder methods
func (r *Runtime) executeAuthLoginResultMethod(instance *Instance, method string, args []interface{}) interface{} {
	switch method {
	case "require2FA":
		userId, okId := instance.Fields["user_id"].(int)
		success, okSuccess := instance.Fields["success"].(bool)

		if okId && okSuccess && success {
			prefix := r.dbPrefix()
			mfaMethodsTable := prefix + "user_mfa_methods"

			// Check if any MFA method is active for the user
			var count int
			query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE user_id = ? AND is_active = 1", mfaMethodsTable)
			err := r.databaseExecutor().QueryRow(query, userId).Scan(&count)
			if err == nil && count > 0 {
				instance.Fields["requires_2fa"] = true
			} else {
				instance.Fields["requires_2fa"] = false
			}
		} else {
			instance.Fields["requires_2fa"] = false
		}
		return instance

	case "onSuccess":
		if len(args) > 0 {
			success, _ := instance.Fields["success"].(bool)
			requires2FA, _ := instance.Fields["requires_2fa"].(bool)

			if success && !requires2FA {
				callback := args[0]
				jwtToken := instance.Fields["jwt"]
				instance.Fields["_response"] = r.CallFunction(callback, []interface{}{jwtToken})
			}
		}
		return instance

	case "onChallenge":
		if len(args) > 0 {
			success, _ := instance.Fields["success"].(bool)
			requires2FA, _ := instance.Fields["requires_2fa"].(bool)

			if success && requires2FA {
				callback := args[0]
				userId := instance.Fields["user_id"].(int)
				email := fmt.Sprintf("%v", instance.Fields["email"])

				tempToken := r.generateTemporaryChallengeJWT(userId, email)
				instance.Fields["_response"] = r.CallFunction(callback, []interface{}{tempToken})
			}
		}
		return instance

	case "onFail":
		if len(args) > 0 {
			success, _ := instance.Fields["success"].(bool)

			if !success {
				callback := args[0]
				errMessage := instance.Fields["error"]
				instance.Fields["_response"] = r.CallFunction(callback, []interface{}{errMessage})
			}
		}
		return instance

	case "response":
		if resp, ok := instance.Fields["_response"]; ok {
			return resp
		}
		return nil
	}
	return nil
}

// generateTemporaryChallengeJWT creates a short-lived token for 2FA validation
func (r *Runtime) generateTemporaryChallengeJWT(userId int, email string) string {
	jwtToken := r.generateMFAChallengeJWT(userId, email)
	return fmt.Sprintf("%v", jwtToken)
}

// executeMFAMethod handles the MFA class methods
func (r *Runtime) executeMFAMethod(instance *Instance, method string, args []interface{}) interface{} {
	switch method {
	case "generateTOTP":
		// MFA::generateTOTP() -> returns {secret, qr_uri, qr_url}
		secret := generateRandomBase32Secret()
		// Generate standard otpauth URI
		appName := "JossApp"
		if val, ok := r.Env["APP_NAME"]; ok {
			appName = val
		}
		rawUri := fmt.Sprintf("otpauth://totp/%s?secret=%s&issuer=Joss", appName, secret)
		qrUri := url.QueryEscape(rawUri)

		res := make(map[string]interface{})
		res["secret"] = secret
		res["qr_uri"] = qrUri
		res["qr_url"] = fmt.Sprintf("https://api.qrserver.com/v1/create-qr-code/?size=200x200&data=%s", qrUri)
		return res

	case "verifyTOTP":
		// MFA::verifyTOTP(secret, code)
		if len(args) >= 2 {
			secret := fmt.Sprintf("%v", args[0])
			code := fmt.Sprintf("%v", args[1])
			return verifyTOTPCode(secret, code)
		}
		return false

	case "generateRecoveryCodes":
		// MFA::generateRecoveryCodes() -> returns list of 8 codes
		codes := make([]interface{}, 8)
		for i := 0; i < 8; i++ {
			codes[i] = generateRandomRecoveryCode()
		}
		return codes

	case "verifyRecoveryCode":
		// MFA::verifyRecoveryCode($userId, $code)
		if len(args) >= 2 {
			userId := args[0]
			code := fmt.Sprintf("%v", args[1])

			prefix := r.dbPrefix()
			recoveryCodesTable := prefix + "user_recovery_codes"

			// We need to look up all unused codes for this user
			rows, err := r.databaseExecutor().Query(fmt.Sprintf("SELECT id, code_hash FROM %s WHERE user_id = ? AND used = 0", recoveryCodesTable), userId)
			if err != nil {
				return false
			}
			defer rows.Close()

			for rows.Next() {
				var id int
				var codeHash string
				if err := rows.Scan(&id, &codeHash); err == nil {
					if bcrypt.CompareHashAndPassword([]byte(codeHash), []byte(code)) == nil {
						r.databaseExecutor().Exec(fmt.Sprintf("UPDATE %s SET used = 1, used_at = CURRENT_TIMESTAMP WHERE id = ?", recoveryCodesTable), id)
						return true
					}
				}
			}
		}
		return false
	}
	return nil
}

// executeTwoFactorMethod handles the simplified TwoFactor class methods built on top of MFA
func (r *Runtime) executeTwoFactorMethod(instance *Instance, method string, args []interface{}) interface{} {
	switch method {
	case "required":
		// TwoFactor::required($userInstance)
		if len(args) > 0 {
			if userInst, ok := args[0].(*Instance); ok {
				userId := userInst.Fields["id"]
				prefix := r.dbPrefix()
				mfaMethodsTable := prefix + "user_mfa_methods"

				var count int
				query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE user_id = ? AND is_active = 1", mfaMethodsTable)
				err := r.databaseExecutor().QueryRow(query, userId).Scan(&count)
				if err == nil && count > 0 {
					return true
				}
			}
		}
		return false

	case "verify":
		// TwoFactor::verify($userId, $code)
		if len(args) >= 2 {
			userId := args[0]
			code := fmt.Sprintf("%v", args[1])

			prefix := r.dbPrefix()
			mfaMethodsTable := prefix + "user_mfa_methods"

			// Get active TOTP secret
			var secret string
			query := fmt.Sprintf("SELECT secret FROM %s WHERE user_id = ? AND method_type = 'totp' AND is_active = 1 LIMIT 1", mfaMethodsTable)
			err := r.databaseExecutor().QueryRow(query, userId).Scan(&secret)
			if err == nil && secret != "" {
				// Criptografía: Desencriptar secreto usando APP_KEY
				// Para simplificar por ahora, validemos directamente
				return verifyTOTPCode(secret, code)
			}
		}
		return false
	}
	return nil
}

// TOTP Implementation Helpers
func generateRandomBase32Secret() string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"
	var sb strings.Builder
	for i := 0; i < 16; i++ {
		sb.WriteByte(alphabet[rand.Intn(len(alphabet))])
	}
	return sb.String()
}

func generateRandomRecoveryCode() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	var sb strings.Builder
	for i := 0; i < 10; i++ {
		sb.WriteByte(chars[rand.Intn(len(chars))])
	}
	return sb.String()
}

func verifyTOTPCode(secret string, code string) bool {
	// Base32 decode
	key, err := base32.StdEncoding.DecodeString(strings.ToUpper(secret))
	if err != nil {
		return false
	}

	// Current time counter
	counter := time.Now().Unix() / 30

	// Check window size of 2 (current, -2, -1, +1, +2 intervals)
	for i := -2; i <= 2; i++ {
		val := counter + int64(i)
		buf := make([]byte, 8)
		binary.BigEndian.PutUint64(buf, uint64(val))

		h := hmac.New(sha1.New, key)
		h.Write(buf)
		sum := h.Sum(nil)

		offset := sum[len(sum)-1] & 0xf
		binaryVal := binary.BigEndian.Uint32(sum[offset : offset+4])
		binaryVal = binaryVal & 0x7fffffff
		otp := binaryVal % 1000000

		if fmt.Sprintf("%06d", otp) == code {
			return true
		}
	}
	return false
}
