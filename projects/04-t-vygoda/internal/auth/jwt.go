package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type TokenKind string

const (
	TokenAccess  TokenKind = "access"
	TokenRefresh TokenKind = "refresh"
)

type Claims struct {
	UserID int64     `json:"uid"`
	Phone  string    `json:"phone"`
	Kind   TokenKind `json:"kind"`
	jwt.RegisteredClaims
}

type Tokenizer struct {
	secret     []byte
	issuer     string
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewTokenizer(secret string, issuer string, accessTTL, refreshTTL time.Duration) *Tokenizer {
	return &Tokenizer{
		secret:     []byte(secret),
		issuer:     issuer,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

type Pair struct {
	Access    string    `json:"access_token"`
	Refresh   string    `json:"refresh_token"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (t *Tokenizer) Issue(userID int64, phone string) (Pair, error) {
	now := time.Now().UTC()
	access, err := t.sign(userID, phone, TokenAccess, now, t.accessTTL)
	if err != nil {
		return Pair{}, fmt.Errorf("sign access: %w", err)
	}
	refresh, err := t.sign(userID, phone, TokenRefresh, now, t.refreshTTL)
	if err != nil {
		return Pair{}, fmt.Errorf("sign refresh: %w", err)
	}
	return Pair{Access: access, Refresh: refresh, ExpiresAt: now.Add(t.accessTTL)}, nil
}

func (t *Tokenizer) sign(userID int64, phone string, kind TokenKind, now time.Time, ttl time.Duration) (string, error) {
	claims := Claims{
		UserID: userID,
		Phone:  phone,
		Kind:   kind,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    t.issuer,
			Subject:   fmt.Sprintf("%d", userID),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(t.secret)
}

func (t *Tokenizer) Parse(raw string) (*Claims, error) {
	parsed, err := jwt.ParseWithClaims(raw, &Claims{}, func(tok *jwt.Token) (any, error) {
		if _, ok := tok.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return t.secret, nil
	}, jwt.WithIssuer(t.issuer))
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}
	c, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid {
		return nil, errors.New("invalid claims")
	}
	return c, nil
}
