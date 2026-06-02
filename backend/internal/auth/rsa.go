package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"sync"
)

var (
	ErrInvalidCiphertext = errors.New("invalid encrypted password")
	ErrPasswordRequired  = errors.New("password or encrypted_password required")
)

// PasswordCipher RSA-OAEP (SHA-256) for passwords in transit (extra layer over HTTPS).
type PasswordCipher struct {
	mu         sync.RWMutex
	privateKey *rsa.PrivateKey
}

func NewPasswordCipher(privatePEM string) (*PasswordCipher, error) {
	c := &PasswordCipher{}
	if privatePEM != "" {
		if err := c.loadPrivatePEM(privatePEM); err != nil {
			return nil, err
		}
		return c, nil
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("generate rsa key: %w", err)
	}
	c.privateKey = key
	return c, nil
}

func (c *PasswordCipher) loadPrivatePEM(pemStr string) error {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return fmt.Errorf("invalid private key PEM")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		keyAny, err2 := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err2 != nil {
			return fmt.Errorf("parse private key: %w", err)
		}
		c.privateKey = keyAny
		return nil
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return fmt.Errorf("not an RSA private key")
	}
	c.privateKey = rsaKey
	return nil
}

func (c *PasswordCipher) PublicKeyPEM() (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.privateKey == nil {
		return "", fmt.Errorf("no key loaded")
	}
	pubDER, err := x509.MarshalPKIXPublicKey(&c.privateKey.PublicKey)
	if err != nil {
		return "", err
	}
	return string(pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubDER,
	})), nil
}

func (c *PasswordCipher) DecryptPassword(b64 string) (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.privateKey == nil {
		return "", fmt.Errorf("no key loaded")
	}
	ciphertext, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", ErrInvalidCiphertext
	}
	plain, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, c.privateKey, ciphertext, nil)
	if err != nil {
		return "", ErrInvalidCiphertext
	}
	return string(plain), nil
}

// ResolveFromPayload prefers encrypted_password; plain password allowed only when requireEncrypted is false.
func (c *PasswordCipher) ResolveFromPayload(plain, encrypted string, requireEncrypted bool) (string, error) {
	if encrypted != "" {
		return c.DecryptPassword(encrypted)
	}
	if plain != "" {
		if requireEncrypted {
			return "", ErrPasswordRequired
		}
		return plain, nil
	}
	return "", ErrPasswordRequired
}
