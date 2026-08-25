// Package config loads orchestrator-specific environment variables.
//
// All env vars are documented at the field declaration so the
// Helm values file can be the single source of truth for tunable
// values.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/strata/shared/pkg/config"
)

// Config is the orchestrator's full runtime configuration.
type Config struct {
	config.Base

	// DatabaseURL is the Postgres connection string. Phase 1 expects
	// CloudNativePG in the same namespace; the connection string is
	// injected via the orchestrator's Deployment env.
	DatabaseURL string

	// KeycloakURL is the base URL of the Keycloak realm's issuer
	// (e.g. "http://keycloak:8080"). The orchestrator constructs the
	// JWKS URL by appending
	// "/realms/{realm}/protocol/openid-connect/certs".
	KeycloakURL string

	// KeycloakAcceptedIssuers is a comma-separated list of acceptable
	// `iss` claims. Useful in dev so we can accept both the
	// cluster-internal URL and the localhost URL (when port-forwarding).
	// Phase 2 production should pin a single issuer.
	KeycloakAcceptedIssuers string

	// KeycloakRealm is the OIDC realm name.
	KeycloakRealm string

	// JWTAudience is the ``aud`` claim we expect on inbound tokens.
	// Phase 1 accepts either "strata-tui" (TUI device flow) or
	// "strata-web" (web auth-code flow).
	JWTAudience string

	// MCPK8sURL is the base URL of the MCP k8s server's
	// streamable-HTTP endpoint (e.g. "http://mcp-k8s:8000/mcp").
	MCPK8sURL string

	// BootstrapAdminToken is a static token the orchestrator accepts
	// in development to bypass the OIDC validation when Keycloak is
	// unreachable. Phase 1 dev-only. Empty disables it.
	BootstrapAdminToken string
}

// Load reads the orchestrator's config from the environment.
func Load() (Config, error) {
	base, err := config.LoadBase()
	if err != nil {
		return Config{}, err
	}
	cfg := Config{
		Base:        base,
		DatabaseURL: getEnv("DATABASE_URL", ""),
		KeycloakURL: getEnv("KEYCLOAK_URL", "http://keycloak:8080"),
		KeycloakAcceptedIssuers: getEnv(
			"KEYCLOAK_ACCEPTED_ISSUERS",
			"http://keycloak:8080/realms/strata-dev,http://localhost:8081/realms/strata-dev",
		),
		KeycloakRealm:       getEnv("KEYCLOAK_REALM", "strata-dev"),
		JWTAudience:         getEnv("JWT_AUDIENCE", "strata-tui"),
		MCPK8sURL:           getEnv("MCP_K8S_URL", "http://mcp-k8s:8000/mcp"),
		BootstrapAdminToken: getEnv("BOOTSTRAP_ADMIN_TOKEN", ""),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	return cfg, nil
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// JWKSURL is the URL the orchestrator fetches signing keys from.
func (c Config) JWKSURL() string {
	return c.KeycloakURL + "/realms/" + c.KeycloakRealm + "/protocol/openid-connect/certs"
}

// IssuerURL returns the canonical issuer URL — the first entry in the
// comma-separated KeycloakAcceptedIssuers list. The JWKS validator
// also accepts the rest.
func (c Config) IssuerURL() string {
	for _, iss := range strings.Split(c.KeycloakAcceptedIssuers, ",") {
		iss = strings.TrimSpace(iss)
		if iss != "" {
			return iss
		}
	}
	return c.KeycloakURL + "/realms/" + c.KeycloakRealm
}

// HTTPClientTimeout caps any individual HTTP call from the orchestrator.
func HTTPClientTimeout() time.Duration { return 10 * time.Second }
