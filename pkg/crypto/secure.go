package crypto

import (
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"io"

	"golang.org/x/crypto/pbkdf2"
)

// DeriveKey generates a 32-byte key from a password and salt using PBKDF2.
func DeriveKey(password, salt []byte) []byte {
	return pbkdf2.Key(password, salt, 100000, 32, sha256.New)
}

// EncryptAES encrypts data using AES-256-GCM.
// It returns nonce + ciphertext.
func EncryptAES(data, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	return gcm.Seal(nonce, nonce, data, nil), nil
}

// DecryptAES decrypts data using AES-256-GCM.
// It expects data to be nonce + ciphertext.
func DecryptAES(data, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// SignRSASHA256 signs data using the provided RSA private key in PEM format (PKCS#1 or PKCS#8)
// with SHA-256 and PKCS#1 v1.5 padding, returning the base64-encoded signature.
func SignRSASHA256(data string, pemKey string) (string, error) {
	block, _ := pem.Decode([]byte(pemKey))
	if block == nil {
		return "", errors.New("invalid PEM private key")
	}
	var rsaKey *rsa.PrivateKey
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		rsaKey = key
	} else if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if k, ok := key.(*rsa.PrivateKey); ok {
			rsaKey = k
		} else {
			return "", errors.New("parsed key is not RSA")
		}
	} else {
		return "", errors.New("failed to parse private key: neither PKCS#1 nor PKCS#8")
	}

	hashed := sha256.Sum256([]byte(data))
	sig, err := rsa.SignPKCS1v15(rand.Reader, rsaKey, crypto.SHA256, hashed[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}
