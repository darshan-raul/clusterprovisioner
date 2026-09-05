package api

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
	"github.com/rs/zerolog"

	"github.com/strata/orchestrator/internal/config"
	"github.com/strata/shared/pkg/jwks"
)

// testServer wires an api.Server backed by a fake JWKS and a fake
// store + MCP client. It returns the httptest server, the API server,
// and the signing key so tests can mint valid tokens.
func testServer(t *testing.T) (*httptest.Server, *Server, *rsa.PrivateKey) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	jwksBody := buildJWKS(t, "test-kid", &priv.PublicKey)
	jwksSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(jwksBody)
	}))
	t.Cleanup(jwksSrv.Close)

	v := jwks.New(jwks.Config{
		JWKSURL:  jwksSrv.URL,
		Issuer:   "http://test/realms/strata-dev",
		Audience: "strata-tui",
	})
	cfg := config.Config{
		KeycloakURL:      "http://test",
		KeycloakRealm:    "strata-dev",
		JWTAudience:      "strata-tui",
		EncryptionSecret: "test-encryption-secret",
	}
	log := zerolog.Nop()
	st := newFakeStore()
	mcp := newFakeMCP()
	srv := New(cfg, log, v, st, mcp)
	ts := httptest.NewServer(srv.Router())
	t.Cleanup(ts.Close)
	return ts, srv, priv
}

func mintToken(t *testing.T, priv *rsa.PrivateKey, kid, sub, aud string, expired bool) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub":                sub,
		"iss":                "http://test/realms/strata-dev",
		"aud":                []string{aud},
		"exp":                time.Now().Add(time.Hour).Unix(),
		"iat":                time.Now().Unix(),
		"preferred_username": sub,
		"email":              sub + "@example.com",
	})
	if expired {
		tok = jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
			"sub": sub,
			"iss": "http://test/realms/strata-dev",
			"aud": []string{aud},
			"exp": time.Now().Add(-time.Hour).Unix(),
		})
	}
	tok.Header["kid"] = kid
	signed, err := tok.SignedString(priv)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func TestHealthz_OK(t *testing.T) {
	ts, _, _ := testServer(t)
	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d", resp.StatusCode)
	}
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Errorf("body = %v", body)
	}
}

func TestMe_RequiresAuth(t *testing.T) {
	ts, _, _ := testServer(t)
	resp, err := http.Get(ts.URL + "/api/v1/me")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestMe_ReturnsClaims(t *testing.T) {
	ts, _, priv := testServer(t)
	tok := mintToken(t, priv, "test-kid", "alice", "strata-tui", false)

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d", resp.StatusCode)
	}
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body["sub"] != "alice" {
		t.Errorf("sub = %v", body["sub"])
	}
}

func TestMe_RejectsExpiredToken(t *testing.T) {
	ts, _, priv := testServer(t)
	tok := mintToken(t, priv, "test-kid", "alice", "strata-tui", true)
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestMe_RejectsWrongAudience(t *testing.T) {
	ts, _, priv := testServer(t)
	tok := mintToken(t, priv, "test-kid", "alice", "wrong-aud", false)
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestExtractBearer(t *testing.T) {
	cases := []struct {
		in   string
		want string
		err  bool
	}{
		{"", "", true},
		{"foo", "", true},
		{"Bearer ", "", true},
		{"Bearer abc", "abc", false},
		{"Bearer  xyz ", "xyz", false},
		{"bearer abc", "", true}, // case sensitive
	}
	for _, c := range cases {
		got, err := extractBearer(c.in)
		if (err != nil) != c.err {
			t.Errorf("extractBearer(%q) err = %v, want err = %v", c.in, err, c.err)
		}
		if !c.err && got != c.want {
			t.Errorf("extractBearer(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestClaimsFromRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if _, ok := claimsFrom(req); ok {
		t.Error("expected no claims")
	}
	want := &jwks.Claims{Subject: "x"}
	ctx := withClaims(req.Context(), want)
	req = req.WithContext(ctx)
	got, ok := claimsFrom(req)
	if !ok || got.Subject != "x" {
		t.Errorf("claims = %v, ok = %v", got, ok)
	}
}

func TestClaimsContext_PropagatesThroughContext(t *testing.T) {
	ctx := context.Background()
	c := &jwks.Claims{Subject: "y"}
	ctx = withClaims(ctx, c)
	if got, ok := ctx.Value(claimsKey{}).(*jwks.Claims); !ok || got != c {
		t.Errorf("got = %v, ok = %v", got, ok)
	}
}

// buildJWKS is a small helper kept in tests because no production
// code in the orchestrator needs to mint JWKs.
func buildJWKS(t *testing.T, kid string, pub *rsa.PublicKey) []byte {
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
