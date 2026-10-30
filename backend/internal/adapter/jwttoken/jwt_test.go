package jwttoken

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"clodapp/backend/internal/config"
	"clodapp/backend/internal/domain"
	"clodapp/backend/internal/usecase/auth"
)

func newIssuer(secret string, now time.Time) *Issuer {
	i := New(config.Config{JWTSecret: []byte(secret), AccessTokenTTL: 15 * time.Minute})
	i.now = func() time.Time { return now }
	return i
}

const secret = "test-secret-test-secret-test-secret!"

func TestIssueAndParse(t *testing.T) {
	now := time.Now()
	i := newIssuer(secret, now)
	tok, err := i.Issue(42)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := i.Parse(tok.Value)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if claims.UserID != 42 || claims.ID != tok.ID || claims.ID == "" {
		t.Fatalf("unexpected claims %+v", claims)
	}
}

func TestExpired(t *testing.T) {
	now := time.Now()
	tok, _ := newIssuer(secret, now).Issue(1)

	withinSkew := newIssuer(secret, now.Add(15*time.Minute+auth.ClockSkew-time.Second))
	if _, err := withinSkew.Parse(tok.Value); err != nil {
		t.Fatalf("token inside the clock-skew window rejected: %v", err)
	}
	later := newIssuer(secret, now.Add(15*time.Minute+auth.ClockSkew+time.Second))
	if _, err := later.Parse(tok.Value); err != domain.ErrTokenExpired {
		t.Fatalf("want ErrTokenExpired, got %v", err)
	}
}

func TestRejectsForgedTokens(t *testing.T) {
	now := time.Now()
	i := newIssuer(secret, now)
	tok, _ := i.Issue(1)
	parts := strings.Split(tok.Value, ".")
	none := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))

	cases := map[string]string{
		"other secret":  func() string { t, _ := newIssuer(secret+"x", now).Issue(1); return t.Value }(),
		"bad signature": parts[0] + "." + parts[1] + "." + strings.Repeat("A", len(parts[2])),
		"alg none":      none + "." + parts[1] + ".",
		"garbage":       "not-a-jwt",
		"empty":         "",
	}
	for name, token := range cases {
		if _, err := i.Parse(token); err != domain.ErrTokenInvalid {
			t.Errorf("%s: want ErrTokenInvalid, got %v", name, err)
		}
	}
}
