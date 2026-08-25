// Package jwks validates JWTs against a Keycloak realm's JWKS endpoint.
//
// The validator caches keys in memory and refreshes on signature failure.
// Refresh-on-miss is the standard pattern: a key rotation invalidates
// the cached JWKS, so the next signature check that fails triggers a
// re-fetch.
package jwks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Config controls how the validator fetches the JWKS and accepts tokens.
type Config struct {
	// JWKSURL is the full URL to Keycloak's JWKS endpoint, e.g.
	// http://keycloak:8080/realms/strata-dev/protocol/openid-connect/certs
	JWKSURL string
	// Issuer is the expected `iss` claim, e.g.
	// http://keycloak:8080/realms/strata-dev
	//
	// For dev convenience we accept a comma-separated list. The first
	// match wins. Phase 2 production should pin a single issuer.
	Issuer string
	// Audience is the expected `aud` claim. The TUI's tokens are
	// minted for the client `strata-tui`; the web's tokens are
	// minted for `strata-web`. We accept either here.
	Audience string
	// HTTPClient is reused for the JWKS fetch. nil falls back to
	// http.DefaultClient with a 10s timeout.
	HTTPClient *http.Client
	// RefreshTimeout caps how long a single JWKS fetch can take.
	RefreshTimeout time.Duration
}

// issuers returns the configured issuer(s), split on commas.
func (c Config) issuers() []string {
	out := []string{}
	for _, s := range strings.Split(c.Issuer, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// Claims is the subset of OIDC claims Strata uses.
type Claims struct {
	Subject  string   `json:"sub"`
	Email    string   `json:"email"`
	Name     string   `json:"name"`
	Username string   `json:"preferred_username"`
	Audience []string `json:"aud"`
}

// Validator validates JWTs against a cached JWKS.
type Validator struct {
	cfg Config

	mu          sync.RWMutex
	cachedKeys  map[string]any
	lastFetched time.Time
}

// New returns a Validator. The JWKS is fetched lazily on the first
// call to Validate so that a service can boot before Keycloak is up.
func New(cfg Config) *Validator {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 10 * time.Second}
	}
	if cfg.RefreshTimeout == 0 {
		cfg.RefreshTimeout = 10 * time.Second
	}
	return &Validator{cfg: cfg, cachedKeys: map[string]any{}}
}

// Validate parses the bearer token, verifies the signature against
// the cached JWKS (refreshing on miss), checks the issuer and
// audience, and returns the claims.
func (v *Validator) Validate(ctx context.Context, raw string) (*Claims, error) {
	if raw == "" {
		return nil, errors.New("jwks: empty token")
	}
	parser := jwt.NewParser(jwt.WithValidMethods([]string{"RS256", "ES256"}))
	token, err := parser.Parse(raw, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		return v.keyFunc(ctx, kid)
	})
	if err != nil {
		return nil, fmt.Errorf("jwks: parse: %w", err)
	}
	if !token.Valid {
		return nil, errors.New("jwks: token invalid")
	}
	mc, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("jwks: unexpected claims type")
	}
	if iss, _ := mc.GetIssuer(); !v.acceptsIssuer(iss) {
		want := strings.Join(v.cfg.issuers(), ",")
		return nil, fmt.Errorf("jwks: issuer mismatch (got %q want one of %q)", iss, want)
	}
	if v.cfg.Audience != "" {
		// Accept either an `aud` claim containing the audience OR an
		// `azp` claim (Keycloak's "Authorized Party") matching it.
		// Keycloak omits `aud` for direct grants by default.
		if !audienceContains(mc["aud"], v.cfg.Audience) {
			if azp, _ := mc["azp"].(string); azp != v.cfg.Audience {
				return nil, fmt.Errorf("jwks: audience mismatch (want %q)", v.cfg.Audience)
			}
		}
	}
	sub, _ := mc.GetSubject()
	if sub == "" {
		if s, ok := mc["sub"].(string); ok {
			sub = s
		}
	}
	claims := &Claims{
		Subject:  sub,
		Audience: toStringSlice(mc["aud"]),
	}
	if e, ok := mc["email"].(string); ok {
		claims.Email = e
	}
	if n, ok := mc["name"].(string); ok {
		claims.Name = n
	}
	if u, ok := mc["preferred_username"].(string); ok {
		claims.Username = u
	}
	return claims, nil
}

// acceptsIssuer reports whether iss is in the validator's accepted
// issuer list.
func (v *Validator) acceptsIssuer(iss string) bool {
	for _, allowed := range v.cfg.issuers() {
		if iss == allowed {
			return true
		}
	}
	return false
}

// keyFunc returns the public key for the given kid. Refreshes the
// JWKS cache if the kid is unknown.
func (v *Validator) keyFunc(ctx context.Context, kid string) (any, error) {
	v.mu.RLock()
	k, ok := v.cachedKeys[kid]
	v.mu.RUnlock()
	if ok {
		return k, nil
	}
	if err := v.refresh(ctx); err != nil {
		return nil, err
	}
	v.mu.RLock()
	k, ok = v.cachedKeys[kid]
	v.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("jwks: kid %q not in JWKS", kid)
	}
	return k, nil
}

func (v *Validator) refresh(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.cfg.JWKSURL, nil)
	if err != nil {
		return fmt.Errorf("jwks: build request: %w", err)
	}
	client := v.cfg.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("jwks: fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("jwks: fetch returned %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("jwks: read: %w", err)
	}
	var jwks struct {
		Keys []json.RawMessage `json:"keys"`
	}
	if err := json.Unmarshal(body, &jwks); err != nil {
		return fmt.Errorf("jwks: unmarshal: %w", err)
	}
	parsed := make(map[string]any, len(jwks.Keys))
	for _, raw := range jwks.Keys {
		key, err := parseJWK(raw)
		if err != nil {
			return fmt.Errorf("jwks: parse key: %w", err)
		}
		kid, _ := key.kid()
		if kid == "" {
			continue
		}
		parsed[kid] = key.publicKey()
	}
	v.mu.Lock()
	v.cachedKeys = parsed
	v.lastFetched = time.Now()
	v.mu.Unlock()
	return nil
}

func audienceContains(aud any, want string) bool {
	switch a := aud.(type) {
	case string:
		return a == want
	case []any:
		for _, x := range a {
			if s, ok := x.(string); ok && s == want {
				return true
			}
		}
	}
	return false
}

func toStringSlice(v any) []string {
	switch x := v.(type) {
	case string:
		return []string{x}
	case []any:
		out := make([]string, 0, len(x))
		for _, e := range x {
			if s, ok := e.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}
