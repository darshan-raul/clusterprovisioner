package jwks

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestValidator_RSAToken(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	jwksBody := rsaJWKSBody(t, "test-kid", &priv.PublicKey)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(jwksBody)
	}))
	defer srv.Close()

	v := New(Config{
		JWKSURL:  srv.URL,
		Issuer:   "http://test/realms/strata-dev",
		Audience: "strata-tui",
	})

	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub":                "user-1",
		"iss":                "http://test/realms/strata-dev",
		"aud":                []string{"strata-tui", "account"},
		"exp":                time.Now().Add(time.Hour).Unix(),
		"iat":                time.Now().Unix(),
		"email":              "dev@example.com",
		"preferred_username": "dev",
	})
	tok.Header["kid"] = "test-kid"
	signed, err := tok.SignedString(priv)
	if err != nil {
		t.Fatal(err)
	}

	claims, err := v.Validate(context.Background(), signed)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if claims.Subject != "user-1" {
		t.Errorf("sub = %q", claims.Subject)
	}
	if claims.Email != "dev@example.com" {
		t.Errorf("email = %q", claims.Email)
	}
}

func TestValidator_AcceptsAzpWhenAudMissing(t *testing.T) {
	// Real Keycloak direct-access-grant tokens for a public client
	// (like our strata-tui) carry `aud=account` and put the client
	// id in `azp` (Authorized Party). The orchestrator must accept
	// these so the e2e + the TUI's device-code flow can talk to it.
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	jwksBody := rsaJWKSBody(t, "test-kid", &priv.PublicKey)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(jwksBody)
	}))
	defer srv.Close()

	v := New(Config{
		JWKSURL:  srv.URL,
		Issuer:   "http://test/realms/strata-dev",
		Audience: "strata-tui",
	})

	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub":                "user-1",
		"iss":                "http://test/realms/strata-dev",
		"aud":                "account", // Keycloak default for public clients
		"azp":                "strata-tui",
		"exp":                time.Now().Add(time.Hour).Unix(),
		"iat":                time.Now().Unix(),
		"email":              "dev@strata.local",
		"preferred_username": "dev",
	})
	tok.Header["kid"] = "test-kid"
	signed, err := tok.SignedString(priv)
	if err != nil {
		t.Fatal(err)
	}

	claims, err := v.Validate(context.Background(), signed)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if claims.Username != "dev" {
		t.Errorf("preferred_username = %q", claims.Username)
	}
	if claims.Email != "dev@strata.local" {
		t.Errorf("email = %q", claims.Email)
	}
}

func TestValidator_RejectsWrongIssuer(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	jwksBody := rsaJWKSBody(t, "test-kid", &priv.PublicKey)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(jwksBody)
	}))
	defer srv.Close()

	v := New(Config{
		JWKSURL:  srv.URL,
		Issuer:   "http://test/realms/strata-dev",
		Audience: "strata-tui",
	})

	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub": "u",
		"iss": "http://attacker.example/realms/x",
		"aud": "strata-tui",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	tok.Header["kid"] = "test-kid"
	signed, _ := tok.SignedString(priv)
	if _, err := v.Validate(context.Background(), signed); err == nil {
		t.Fatal("expected error for wrong issuer")
	}
}

func TestValidator_RefreshOnUnknownKid(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	bodyV1 := rsaJWKSBody(t, "kid-v1", &priv.PublicKey)
	bodyV2 := rsaJWKSBody(t, "kid-v2", &priv.PublicKey)

	var current []byte
	switcher := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switcher++
		if switcher == 1 {
			_, _ = w.Write(bodyV1)
		} else {
			_, _ = w.Write(bodyV2)
		}
	}))
	defer srv.Close()
	_ = current

	v := New(Config{
		JWKSURL:  srv.URL,
		Issuer:   "http://test/realms/strata-dev",
		Audience: "strata-tui",
	})

	tokV1 := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub": "u", "iss": "http://test/realms/strata-dev", "aud": "strata-tui",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	tokV1.Header["kid"] = "kid-v1"
	signedV1, _ := tokV1.SignedString(priv)

	if _, err := v.Validate(context.Background(), signedV1); err != nil {
		t.Fatalf("first validate: %v", err)
	}

	tokV2 := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub": "u", "iss": "http://test/realms/strata-dev", "aud": "strata-tui",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	tokV2.Header["kid"] = "kid-v2"
	signedV2, _ := tokV2.SignedString(priv)

	if _, err := v.Validate(context.Background(), signedV2); err != nil {
		t.Fatalf("second validate (should refresh): %v", err)
	}
}

func rsaJWKSBody(t *testing.T, kid string, pub *rsa.PublicKey) []byte {
	t.Helper()
	eBytes := []byte{0, 0, 0}
	e := pub.E
	for i := len(eBytes) - 1; i >= 0 && e > 0; i-- {
		eBytes[i] = byte(e & 0xff)
		e >>= 8
	}
	jwk := map[string]any{
		"kty": "RSA",
		"kid": kid,
		"use": "sig",
		"alg": "RS256",
		"n":   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(eBytes),
	}
	body, err := json.Marshal(map[string]any{"keys": []any{jwk}})
	if err != nil {
		t.Fatal(err)
	}
	return body
}
