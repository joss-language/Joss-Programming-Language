package core

import (
	"crypto/hmac"
	"crypto/md5"
	crand "crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"html"
	"math"
	"math/rand"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/jossecurity/joss/pkg/crypto"
	"github.com/jossecurity/joss/pkg/i18n"
)

func (r *Runtime) callBuiltinString(name string, args []interface{}) (interface{}, bool) {
	switch name {
	case "html_escape":
		if len(args) > 0 {
			if args[0] == nil {
				return "", true
			}
			return html.EscapeString(fmt.Sprintf("%v", args[0])), true
		}
		return "", true

	case "__":
		if len(args) > 0 {
			if key, ok := args[0].(string); ok {
				locale := r.GetLocale()
				return i18n.GlobalManager.Get(locale, key, nil), true
			}
		}
		return "", true

	case "csrf_field":
		tokenVal := ""
		if sessVal, ok := r.Variables["$__session"]; ok {
			if sessInst, ok := sessVal.(*Instance); ok {
				if tok, ok := sessInst.Fields["csrf_token"]; ok {
					tokenVal = fmt.Sprintf("%v", tok)
				}
			}
		}
		return fmt.Sprintf(`<input type="hidden" name="_token" value="%s">`, tokenVal), true

	case "print", "echo":
		for _, arg := range args {
			fmt.Fprintln(r.Output(), arg)
		}
		return nil, true

	case "cout":
		for _, arg := range args {
			fmt.Fprint(r.Output(), arg)
		}
		return nil, true

	case "cerr":
		for _, arg := range args {
			fmt.Fprint(r.ErrorOutput(), arg)
		}
		return nil, true

	case "printf":
		if len(args) > 0 {
			if fmtStr, ok := args[0].(string); ok {
				fmt.Printf(fmtStr, args[1:]...)
			}
		}
		return nil, true

	case "strtolower":
		if len(args) > 0 {
			return strings.ToLower(fmt.Sprintf("%v", args[0])), true
		}
		return "", true

	case "strtoupper":
		if len(args) > 0 {
			return strings.ToUpper(fmt.Sprintf("%v", args[0])), true
		}
		return "", true

	case "trim":
		if len(args) > 0 {
			str := fmt.Sprintf("%v", args[0])
			if len(args) > 1 {
				cutset := fmt.Sprintf("%v", args[1])
				return strings.Trim(str, cutset), true
			}
			return strings.TrimSpace(str), true
		}
		return "", true

	case "ltrim":
		if len(args) > 0 {
			str := fmt.Sprintf("%v", args[0])
			if len(args) > 1 {
				cutset := fmt.Sprintf("%v", args[1])
				return strings.TrimLeft(str, cutset), true
			}
			return strings.TrimLeft(str, " \t\n\r\v\f"), true
		}
		return "", true

	case "rtrim":
		if len(args) > 0 {
			str := fmt.Sprintf("%v", args[0])
			if len(args) > 1 {
				cutset := fmt.Sprintf("%v", args[1])
				return strings.TrimRight(str, cutset), true
			}
			return strings.TrimRight(str, " \t\n\r\v\f"), true
		}
		return "", true

	case "substr":
		if len(args) >= 2 {
			str := fmt.Sprintf("%v", args[0])
			start := 0
			switch v := args[1].(type) {
			case int:
				start = v
			case int64:
				start = int(v)
			}
			runes := []rune(str)
			rLen := len(runes)
			if start < 0 {
				start = rLen + start
				if start < 0 {
					start = 0
				}
			}
			if start > rLen {
				return "", true
			}
			if len(args) >= 3 {
				length := 0
				switch v := args[2].(type) {
				case int:
					length = v
				case int64:
					length = int(v)
				}
				if length < 0 {
					end := rLen + length
					if end <= start {
						return "", true
					}
					return string(runes[start:end]), true
				}
				end := start + length
				if end > rLen {
					end = rLen
				}
				return string(runes[start:end]), true
			}
			return string(runes[start:]), true
		}
		return "", true

	case "strpos":
		if len(args) >= 2 {
			haystack := fmt.Sprintf("%v", args[0])
			needle := fmt.Sprintf("%v", args[1])
			byteIdx := strings.Index(haystack, needle)
			if byteIdx == -1 {
				return false, true
			}
			runeIdx := len([]rune(haystack[:byteIdx]))
			return int64(runeIdx), true
		}
		return false, true

	case "md5":
		if len(args) > 0 {
			h := md5.Sum([]byte(fmt.Sprintf("%v", args[0])))
			return hex.EncodeToString(h[:]), true
		}
		return "", true

	case "sha1":
		if len(args) > 0 {
			h := sha1.Sum([]byte(fmt.Sprintf("%v", args[0])))
			return hex.EncodeToString(h[:]), true
		}
		return "", true

	case "sha256":
		if len(args) > 0 {
			h := sha256.Sum256([]byte(fmt.Sprintf("%v", args[0])))
			return hex.EncodeToString(h[:]), true
		}
		return "", true

	case "base64_encode":
		if len(args) > 0 {
			return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%v", args[0]))), true
		}
		return "", true

	case "base64_decode":
		if len(args) > 0 {
			b, err := base64.StdEncoding.DecodeString(fmt.Sprintf("%v", args[0]))
			if err != nil {
				return false, true
			}
			return string(b), true
		}
		return false, true

	case "sha256_base64":
		if len(args) > 0 {
			h := sha256.Sum256([]byte(fmt.Sprintf("%v", args[0])))
			return base64.StdEncoding.EncodeToString(h[:]), true
		}
		return "", true

	case "sha1_base64":
		if len(args) > 0 {
			h := sha1.Sum([]byte(fmt.Sprintf("%v", args[0])))
			return base64.StdEncoding.EncodeToString(h[:]), true
		}
		return "", true

	case "password_hash":
		if len(args) == 0 {
			return "", true
		}
		return jossPBKDF2Hash(fmt.Sprintf("%v", args[0])), true

	case "password_verify":
		if len(args) < 2 {
			return false, true
		}
		return jossPBKDF2Verify(fmt.Sprintf("%v", args[0]), fmt.Sprintf("%v", args[1])), true

	case "jwt_sign":
		if len(args) < 2 {
			return "", true
		}
		return jossJWTSignHS256(fmt.Sprintf("%v", args[0]), fmt.Sprintf("%v", args[1])), true

	case "jwt_verify":
		if len(args) < 2 {
			return false, true
		}
		return jossJWTVerifyHS256(fmt.Sprintf("%v", args[0]), fmt.Sprintf("%v", args[1])), true

	case "totp_generate":
		if len(args) == 0 {
			return "000000", true
		}
		var unixTS int64
		if len(args) >= 2 {
			unixTS = toInt64Arg(args[1])
		}
		return jossTOTPGenerate(fmt.Sprintf("%v", args[0]), unixTS), true

	case "totp_verify":
		if len(args) < 2 {
			return false, true
		}
		var unixTS int64
		if len(args) >= 3 {
			unixTS = toInt64Arg(args[2])
		}
		return jossTOTPVerify(fmt.Sprintf("%v", args[0]), fmt.Sprintf("%v", args[1]), unixTS), true

	case "chr":
		if len(args) > 0 {
			var code int64
			switch v := args[0].(type) {
			case int:
				code = int64(v)
			case int32:
				code = int64(v)
			case int64:
				code = v
			case float64:
				code = int64(v)
			}
			return string([]byte{byte(code)}), true
		}
		return "", true

	case "ord":
		if len(args) > 0 {
			s := fmt.Sprintf("%v", args[0])
			if len(s) > 0 {
				return int64(s[0]), true
			}
		}
		return int64(0), true

	case "__joss_string_byte_at":
		if len(args) >= 2 {
			str, ok := args[0].(string)
			index := toIntVal(args[1])
			if ok && index >= 0 && index < len(str) {
				return int64(str[index]), true
			}
		}
		return int64(0), true

	case "__joss_byte_string":
		if len(args) > 0 {
			return string([]byte{byte(toIntVal(args[0]))}), true
		}
		return "", true

	case "rsa_sign_sha256":
		if len(args) >= 2 {
			data := fmt.Sprintf("%v", args[0])
			pemKey := fmt.Sprintf("%v", args[1])
			sig, err := crypto.SignRSASHA256(data, pemKey)
			if err != nil {
				return "", true
			}
			return sig, true
		}
		return "", true

	case "urlencode":
		if len(args) > 0 {
			return url.QueryEscape(fmt.Sprintf("%v", args[0])), true
		}
		return "", true

	case "urldecode":
		if len(args) > 0 {
			s, err := url.QueryUnescape(fmt.Sprintf("%v", args[0]))
			if err != nil {
				return fmt.Sprintf("%v", args[0]), true
			}
			return s, true
		}
		return "", true

	case "rawurlencode":
		if len(args) > 0 {
			return url.PathEscape(fmt.Sprintf("%v", args[0])), true
		}
		return "", true

	case "rawurldecode":
		if len(args) > 0 {
			s, err := url.PathUnescape(fmt.Sprintf("%v", args[0]))
			if err != nil {
				return fmt.Sprintf("%v", args[0]), true
			}
			return s, true
		}
		return "", true

	case "strlen":
		if len(args) > 0 {
			if args[0] == nil {
				return int64(0), true
			}
			str := fmt.Sprintf("%v", args[0])
			return int64(len([]rune(str))), true
		}
		return int64(0), true

	case "ucwords":
		if len(args) > 0 {
			str := fmt.Sprintf("%v", args[0])
			return titleCaseWords(str), true
		}
		return "", true

	case "ucfirst":
		if len(args) > 0 {
			str := fmt.Sprintf("%v", args[0])
			if len(str) == 0 {
				return "", true
			}
			runes := []rune(str)
			runes[0] = unicode.ToUpper(runes[0])
			return string(runes), true
		}
		return "", true

	case "lcfirst":
		if len(args) > 0 {
			str := fmt.Sprintf("%v", args[0])
			if len(str) == 0 {
				return "", true
			}
			runes := []rune(str)
			runes[0] = unicode.ToLower(runes[0])
			return string(runes), true
		}
		return "", true

	case "str_pad":
		if len(args) >= 2 {
			input := fmt.Sprintf("%v", args[0])
			padLen := 0
			switch v := args[1].(type) {
			case int:
				padLen = v
			case int64:
				padLen = int(v)
			}
			padStr := " "
			if len(args) >= 3 {
				padStr = fmt.Sprintf("%v", args[2])
			}
			if len(input) >= padLen {
				return input, true
			}
			diff := padLen - len(input)
			pad := strings.Repeat(padStr, (diff/len(padStr))+1)
			return input + pad[:diff], true
		}
		return "", true

	case "round":
		if len(args) > 0 {
			f := 0.0
			switch v := args[0].(type) {
			case float64:
				f = v
			case float32:
				f = float64(v)
			case int:
				f = float64(v)
			case int64:
				f = float64(v)
			}
			precision := 0
			if len(args) > 1 {
				switch p := args[1].(type) {
				case int:
					precision = p
				case int64:
					precision = int(p)
				}
			}
			ratio := math.Pow(10, float64(precision))
			return math.Round(f*ratio) / ratio, true
		}
		return float64(0), true

	case "floor":
		if len(args) > 0 {
			f := 0.0
			switch v := args[0].(type) {
			case float64:
				f = v
			case float32:
				f = float64(v)
			case int:
				return int64(v), true
			case int64:
				return v, true
			}
			return int64(math.Floor(f)), true
		}
		return int64(0), true

	case "ceil":
		if len(args) > 0 {
			f := 0.0
			switch v := args[0].(type) {
			case float64:
				f = v
			case float32:
				f = float64(v)
			case int:
				return int64(v), true
			case int64:
				return v, true
			}
			return int64(math.Ceil(f)), true
		}
		return int64(0), true

	case "abs":
		if len(args) > 0 {
			switch v := args[0].(type) {
			case int:
				if v < 0 {
					return int64(-v), true
				}
				return int64(v), true
			case int64:
				if v < 0 {
					return -v, true
				}
				return v, true
			case float64:
				return math.Abs(v), true
			case float32:
				return math.Abs(float64(v)), true
			}
		}
		return int64(0), true

	case "min":
		if len(args) == 0 {
			return nil, true
		}
		if len(args) == 1 {
			if list, ok := args[0].([]interface{}); ok && len(list) > 0 {
				m := list[0]
				for _, item := range list[1:] {
					if compareLessThan(item, m) {
						m = item
					}
				}
				return m, true
			}
			return args[0], true
		}
		m := args[0]
		for _, item := range args[1:] {
			if compareLessThan(item, m) {
				m = item
			}
		}
		return m, true

	case "max":
		if len(args) == 0 {
			return nil, true
		}
		if len(args) == 1 {
			if list, ok := args[0].([]interface{}); ok && len(list) > 0 {
				m := list[0]
				for _, item := range list[1:] {
					if compareGreaterThan(item, m) {
						m = item
					}
				}
				return m, true
			}
			return args[0], true
		}
		m := args[0]
		for _, item := range args[1:] {
			if compareGreaterThan(item, m) {
				m = item
			}
		}
		return m, true

	case "rand":
		minVal := int64(0)
		maxVal := int64(math.MaxInt32)
		if len(args) >= 2 {
			if m1, ok := toInt64Safe(args[0]); ok {
				minVal = m1
			}
			if m2, ok := toInt64Safe(args[1]); ok {
				maxVal = m2
			}
		}
		if maxVal <= minVal {
			return minVal, true
		}
		return minVal + rand.Int63n(maxVal-minVal+1), true
	}

	return nil, false
}

func titleCaseWords(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	capNext := true
	for _, r := range s {
		if unicode.IsSpace(r) {
			capNext = true
			b.WriteRune(r)
		} else if capNext {
			b.WriteRune(unicode.ToTitle(r))
			capNext = false
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func toInt64Arg(v interface{}) int64 {
	switch x := v.(type) {
	case int:
		return int64(x)
	case int32:
		return int64(x)
	case int64:
		return x
	case float64:
		return int64(x)
	case string:
		n, _ := strconv.ParseInt(strings.TrimSpace(x), 10, 64)
		return n
	}
	return 0
}

func pbkdf2HMACSHA256(password, salt []byte, iterations int) []byte {
	prf := hmac.New(sha256.New, password)
	var block [4]byte
	binary.BigEndian.PutUint32(block[:], 1)
	prf.Write(salt)
	prf.Write(block[:])
	u := prf.Sum(nil)
	out := make([]byte, len(u))
	copy(out, u)
	for i := 1; i < iterations; i++ {
		prf.Reset()
		prf.Write(u)
		u = prf.Sum(u[:0])
		for j := range out {
			out[j] ^= u[j]
		}
	}
	return out
}

func jossPBKDF2Hash(password string) string {
	const iterations = 100000
	salt := make([]byte, 16)
	_, _ = crand.Read(salt)
	dk := pbkdf2HMACSHA256([]byte(password), salt, iterations)
	return fmt.Sprintf("$pbkdf2-sha256$%d$%s$%s", iterations, hex.EncodeToString(salt), hex.EncodeToString(dk))
}

func jossPBKDF2Verify(hashStr, password string) bool {
	if !strings.HasPrefix(hashStr, "$pbkdf2-sha256$") {
		return false
	}
	parts := strings.Split(hashStr[len("$pbkdf2-sha256$"):], "$")
	if len(parts) != 3 {
		return false
	}
	iterations, err := strconv.Atoi(parts[0])
	if err != nil || iterations < 1000 || iterations > 5000000 {
		return false
	}
	salt, err := hex.DecodeString(parts[1])
	if err != nil || len(salt) != 16 {
		return false
	}
	expectedDK, err := hex.DecodeString(parts[2])
	if err != nil || len(expectedDK) != 32 {
		return false
	}
	actualDK := pbkdf2HMACSHA256([]byte(password), salt, iterations)
	return subtle.ConstantTimeCompare(actualDK, expectedDK) == 1
}

func jossJWTSignHS256(payloadJSON, secret string) string {
	if payloadJSON == "" {
		payloadJSON = "{}"
	}
	headerB64 := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payloadB64 := base64.RawURLEncoding.EncodeToString([]byte(payloadJSON))
	msg := headerB64 + "." + payloadB64
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(msg))
	sigB64 := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return msg + "." + sigB64
}

func jossJWTVerifyHS256(token, secret string) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[2] == "" {
		return false
	}
	msg := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(msg))
	expectedSig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return subtle.ConstantTimeCompare([]byte(expectedSig), []byte(parts[2])) == 1
}

func decodeBase32Key(secret string) []byte {
	cleaned := strings.ToUpper(strings.Map(func(r rune) rune {
		if r == ' ' || r == '-' || r == '=' || r == '\t' || r == '\r' || r == '\n' {
			return -1
		}
		return r
	}, secret))
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(cleaned)
	if err != nil || len(key) == 0 {
		return []byte(secret)
	}
	return key
}

func totpForStepGo(key []byte, step uint64) string {
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], step)
	mac := hmac.New(sha1.New, key)
	mac.Write(msg[:])
	digest := mac.Sum(nil)
	offset := digest[19] & 0x0f
	binCode := (uint32(digest[offset]&0x7f) << 24) |
		(uint32(digest[offset+1]) << 16) |
		(uint32(digest[offset+2]) << 8) |
		uint32(digest[offset+3])
	return fmt.Sprintf("%06d", binCode%1000000)
}

func jossTOTPGenerate(secret string, unixTime int64) string {
	if secret == "" {
		return "000000"
	}
	if unixTime <= 0 {
		unixTime = time.Now().Unix()
	}
	key := decodeBase32Key(secret)
	return totpForStepGo(key, uint64(unixTime/30))
}

func jossTOTPVerify(secret, code string, unixTime int64) bool {
	clean := strings.TrimSpace(code)
	if secret == "" || len(clean) != 6 {
		return false
	}
	for i := 0; i < 6; i++ {
		if clean[i] < '0' || clean[i] > '9' {
			return false
		}
	}
	if unixTime <= 0 {
		unixTime = time.Now().Unix()
	}
	key := decodeBase32Key(secret)
	baseStep := unixTime / 30
	for delta := int64(-1); delta <= 1; delta++ {
		if baseStep+delta < 0 {
			continue
		}
		expected := totpForStepGo(key, uint64(baseStep+delta))
		if subtle.ConstantTimeCompare([]byte(expected), []byte(clean)) == 1 {
			return true
		}
	}
	return false
}
