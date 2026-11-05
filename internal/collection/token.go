package collection

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

// tokenCipherPrefix marks instance secrets encrypted with the server's
// session secret. Rows without it are legacy plaintext written before
// encryption existed; they stay readable and are re-sealed on the next save.
const tokenCipherPrefix = "enc:v1:"

// errNoTokenKey means the process has no session secret to derive a key from.
var errNoTokenKey = errors.New("session secret is not configured")

// deriveTokenKey turns the session secret into an AES-256 key. HMAC with a
// fixed label is used for domain separation, so the session cookie signer and
// the secret cipher never share raw key material.
func deriveTokenKey(secret string) [32]byte {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("the-shelf/instance-secret/v1"))
	var key [32]byte
	copy(key[:], mac.Sum(nil))
	return key
}

func tokenGCM(secret string) (cipher.AEAD, error) {
	key := deriveTokenKey(secret)
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// sealToken encrypts a secret for storage. The nonce is prepended to the
// ciphertext and the result is base64url-encoded and version-tagged.
func sealToken(secret, plaintext string) (string, error) {
	if secret == "" {
		return "", errNoTokenKey
	}
	gcm, err := tokenGCM(secret)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return tokenCipherPrefix + base64.RawURLEncoding.EncodeToString(sealed), nil
}

// openToken decrypts a stored secret. Legacy plaintext values are returned
// unchanged so upgrading an instance does not lose its token.
func openToken(secret, stored string) (string, error) {
	stored = strings.TrimSpace(stored)
	if stored == "" {
		return "", nil
	}
	if !strings.HasPrefix(stored, tokenCipherPrefix) {
		return stored, nil
	}
	if secret == "" {
		return "", errNoTokenKey
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(stored, tokenCipherPrefix))
	if err != nil {
		return "", errors.New("encrypted token is not valid base64")
	}
	gcm, err := tokenGCM(secret)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("encrypted token is truncated")
	}
	nonce, ciphertext := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", errors.New("encrypted token failed authentication")
	}
	return string(plain), nil
}

// describeSealError keeps key problems actionable without revealing material.
func describeSealError(err error) error {
	if errors.Is(err, errNoTokenKey) {
		return fmt.Errorf("set SESSION_SECRET so the BGG API token can be encrypted: %w", err)
	}
	return err
}
