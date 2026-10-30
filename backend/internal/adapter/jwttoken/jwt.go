// Package jwttoken issues and verifies access tokens as HS256-signed JWTs.
package jwttoken

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/fx"

	"clodapp/backend/internal/config"
	"clodapp/backend/internal/domain"
	"clodapp/backend/internal/usecase/auth"
)

var Module = fx.Module("jwt",
	fx.Provide(
		New,
		func(i *Issuer) auth.AccessTokens { return i },
	),
)

const (
	issuer   = "flowboard"
	audience = "flowboard-api"
)

type Issuer struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

func New(cfg config.Config) *Issuer {
	return &Issuer{secret: cfg.JWTSecret, ttl: cfg.AccessTokenTTL, now: time.Now}
}

func (i *Issuer) Issue(userID int64) (auth.AccessToken, error) {
	idBytes := make([]byte, 16)
	if _, err := rand.Read(idBytes); err != nil {
		return auth.AccessToken{}, err
	}
	now := i.now()
	claims := jwt.RegisteredClaims{
		Issuer:    issuer,
		Subject:   strconv.FormatInt(userID, 10),
		Audience:  jwt.ClaimStrings{audience},
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(i.ttl)),
		ID:        hex.EncodeToString(idBytes),
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(i.secret)
	if err != nil {
		return auth.AccessToken{}, err
	}
	return auth.AccessToken{Value: signed, ID: claims.ID, ExpiresAt: claims.ExpiresAt.Time}, nil
}

func (i *Issuer) Parse(token string) (auth.AccessClaims, error) {
	var claims jwt.RegisteredClaims
	_, err := jwt.ParseWithClaims(token, &claims,
		func(*jwt.Token) (any, error) { return i.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), // no "none", no algorithm confusion
		jwt.WithIssuer(issuer),
		jwt.WithAudience(audience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(auth.ClockSkew),
		jwt.WithTimeFunc(i.now),
	)
	if errors.Is(err, jwt.ErrTokenExpired) {
		return auth.AccessClaims{}, domain.ErrTokenExpired
	}
	if err != nil {
		return auth.AccessClaims{}, domain.ErrTokenInvalid
	}
	userID, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil || claims.ID == "" {
		return auth.AccessClaims{}, domain.ErrTokenInvalid
	}
	return auth.AccessClaims{UserID: userID, ID: claims.ID, ExpiresAt: claims.ExpiresAt.Time}, nil
}
