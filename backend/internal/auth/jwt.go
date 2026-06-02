package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type AccessClaims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

type TokenIssuer struct {
	secret    []byte
	accessTTL time.Duration
}

func NewTokenIssuer(secret string, accessTTL time.Duration) *TokenIssuer {
	return &TokenIssuer{secret: []byte(secret), accessTTL: accessTTL}
}

func (t *TokenIssuer) IssueAccess(userPublicID uuid.UUID, role string) (token string, jti string, expiresAt time.Time, err error) {
	jti = uuid.New().String()
	expiresAt = time.Now().Add(t.accessTTL)
	claims := AccessClaims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userPublicID.String(),
			ID:        jti,
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token, err = tok.SignedString(t.secret)
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("sign access token: %w", err)
	}
	return token, jti, expiresAt, nil
}

func (t *TokenIssuer) ParseAccess(token string) (*AccessClaims, error) {
	claims := &AccessClaims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(tok *jwt.Token) (interface{}, error) {
		if _, ok := tok.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return t.secret, nil
	})
	if err != nil {
		return nil, err
	}
	if !parsed.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}

func GenerateRefreshToken() (plain string, hash string, err error) {
	plain = uuid.New().String() + uuid.New().String()
	return plain, HashToken(plain), nil
}

func HashToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}
