package handler

import (
	"net/http"

	"github.com/anhtuanlc/mediahub/internal/auth"
	"github.com/gin-gonic/gin"
)

type PasswordTransport struct {
	Cipher           *auth.PasswordCipher
	RequireEncrypted bool
}

func (p *PasswordTransport) Resolve(c *gin.Context, plain, encrypted string) (string, error) {
	password, err := p.Cipher.ResolveFromPayload(plain, encrypted, p.RequireEncrypted)
	if err != nil {
		if err == auth.ErrInvalidCiphertext {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "invalid_ciphertext",
				"message": "could not decrypt password",
			})
			return "", err
		}
		if err == auth.ErrPasswordRequired {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "validation_error",
				"message": "encrypted_password is required",
			})
			return "", err
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": err.Error(),
		})
		return "", err
	}
	return password, nil
}

func (p *PasswordTransport) PublicKey(c *gin.Context) {
	pem, err := p.Cipher.PublicKeyPEM()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "failed to export public key",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"algorithm":  "RSA-OAEP-256",
		"public_key": pem,
	})
}
