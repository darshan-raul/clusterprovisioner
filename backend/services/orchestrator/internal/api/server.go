// Package api wires the orchestrator's HTTP surface.
//
// Endpoints:
//
//	GET  /healthz                       — liveness/readiness
//	GET  /api/v1/me                     — current JWT claims (auth required)
//	GET  /api/v1/clusters               — list user's clusters (Phase 1+)
//	GET  /api/v1/clusters/{id}/pods     — proxy to MCP k8s list_pods
//
// Phase 1's only cluster-tool endpoint is /pods; other resource kinds
// land when we add them to the MCP server.
package api

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"

	"github.com/strata/orchestrator/internal/config"
	"github.com/strata/orchestrator/internal/store"
	"github.com/strata/shared/pkg/crypto"
	"github.com/strata/shared/pkg/jwks"
)

// Server is the orchestrator's HTTP handler.
type Server struct {
	cfg       config.Config
	log       zerolog.Logger
	jwks      *jwks.Validator
	store     ClusterStore
	mcp       MCPCaller
	healthzAt time.Time
}

// ClusterStore is the subset of *store.Store that handlers use.
// Defined as an interface so handlers can be unit-tested with a
// fake (see server_test.go).
type ClusterStore interface {
	EnsureUser(ctx context.Context, u store.User) error
	ListClusters(ctx context.Context, userID string) ([]store.Cluster, error)
	GetCluster(ctx context.Context, userID, clusterID string) (*store.Cluster, error)
	CreateCluster(ctx context.Context, c store.Cluster, creds store.ClusterCreds) error
	DeleteCluster(ctx context.Context, userID, clusterID string) error
	RecordAction(ctx context.Context, a store.ActionHistory) error
	ListHistory(ctx context.Context, userID, clusterID string, limit int) ([]store.ActionHistory, error)
}

// MCPCaller is the subset of the MCP client that the handlers use.
// Defined as an interface for testing.
type MCPCaller interface {
	CallToolWithHeaders(ctx context.Context, name string, args map[string]any, headers http.Header) (json.RawMessage, error)
}

// New constructs a Server. jwksValidator is required.
func New(cfg config.Config, log zerolog.Logger, jwksValidator *jwks.Validator, st ClusterStore, mcpClient MCPCaller) *Server {
	return &Server{
		cfg:       cfg,
		log:       log,
		jwks:      jwksValidator,
		store:     st,
		mcp:       mcpClient,
		healthzAt: time.Now(),
	}
}

// Router builds the chi router with all routes attached.
func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(s.requestLogger)
	r.Use(chimw.Recoverer)

	r.Get("/healthz", s.handleHealthz)
	r.Get("/api/v1/me", s.authMiddleware(s.handleMe))
	r.Get("/api/v1/history", s.authMiddleware(s.handleListHistory))

	// Cluster routes
	r.Route("/api/v1/clusters", func(r chi.Router) {
		r.Get("/", s.authMiddleware(s.handleListClusters))
		r.Post("/", s.authMiddleware(s.handleCreateCluster))
		r.Route("/{id}", func(r chi.Router) {
			r.Delete("/", s.authMiddleware(s.handleDeleteCluster))
			r.Get("/history", s.authMiddleware(s.handleListClusterHistory))
			r.Get("/pods", s.authMiddleware(s.handleListPods))
			r.Delete("/pods/{name}", s.authMiddleware(s.handleDeletePod))
			r.Post("/apply", s.authMiddleware(s.handleApplyManifest))
			r.Post("/pods/{name}/exec", s.authMiddleware(s.handleExecCommand))
		})
	})

	return r
}

// handleHealthz returns 200 if the process is alive. Phase 1
// doesn't probe Postgres or Keycloak here; liveness probes only
// need to confirm the HTTP server is up.
func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "ok",
		"started_at": s.healthzAt.UTC().Format(time.RFC3339),
	})
}

// handleMe returns the validated JWT claims for the current token.
// Useful for the TUI to confirm login state.
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsFrom(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "no claims")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"sub":                claims.Subject,
		"email":              claims.Email,
		"name":               claims.Name,
		"preferred_username": claims.Username,
		"aud":                claims.Audience,
	})
}

// handleListClusters returns the user's registered clusters.
func (s *Server) handleListClusters(w http.ResponseWriter, r *http.Request) {
	claims, _ := claimsFrom(r)
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "no claims")
		return
	}
	clusters, err := s.store.ListClusters(r.Context(), claims.Subject)
	if err != nil {
		s.log.Error().Err(err).Msg("list clusters")
		writeError(w, http.StatusInternalServerError, "list clusters failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"clusters": clusters})
}

type CreateClusterRequest struct {
	Name       string `json:"name"`
	Context    string `json:"context,omitempty"`
	Kubeconfig string `json:"kubeconfig"`
}

// handleCreateCluster registers a new Kubernetes cluster with encrypted credentials.
func (s *Server) handleCreateCluster(w http.ResponseWriter, r *http.Request) {
	claims, _ := claimsFrom(r)
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "no claims")
		return
	}

	var req CreateClusterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "malformed request body")
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeError(w, http.StatusBadRequest, "cluster name is required")
		return
	}
	kubeconfigRaw := strings.TrimSpace(req.Kubeconfig)
	if kubeconfigRaw == "" {
		writeError(w, http.StatusBadRequest, "kubeconfig is required")
		return
	}

	var parsedYAML struct {
		CurrentContext string `yaml:"current-context"`
		Contexts       []struct {
			Name string `yaml:"name"`
		} `yaml:"contexts"`
	}
	if err := yaml.Unmarshal([]byte(kubeconfigRaw), &parsedYAML); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid kubeconfig YAML: %v", err))
		return
	}

	contextName := strings.TrimSpace(req.Context)
	if contextName == "" {
		if parsedYAML.CurrentContext != "" {
			contextName = parsedYAML.CurrentContext
		} else if len(parsedYAML.Contexts) > 0 && parsedYAML.Contexts[0].Name != "" {
			contextName = parsedYAML.Contexts[0].Name
		} else {
			contextName = "default"
		}
	}

	randBytes := make([]byte, 6)
	if _, err := rand.Read(randBytes); err != nil {
		s.log.Error().Err(err).Msg("generate cluster id rand")
		writeError(w, http.StatusInternalServerError, "failed to generate cluster id")
		return
	}
	clusterID := fmt.Sprintf("cl-%x", randBytes)

	encSecret := s.cfg.EncryptionSecret
	if encSecret == "" {
		encSecret = "strata-dev-insecure-master-key-change-me"
	}
	key := crypto.DeriveKey(encSecret)
	encryptedKubeconfig, err := crypto.Encrypt(key, []byte(kubeconfigRaw))
	if err != nil {
		s.log.Error().Err(err).Msg("encrypt kubeconfig")
		writeError(w, http.StatusInternalServerError, "failed to encrypt cluster credentials")
		return
	}

	_ = s.store.EnsureUser(r.Context(), store.User{
		ID:       claims.Subject,
		Username: firstNonEmpty(claims.Username, claims.Email, claims.Subject),
		Email:    strPtr(claims.Email),
	})

	cluster := store.Cluster{
		ID:        clusterID,
		UserID:    claims.Subject,
		Name:      name,
		Context:   contextName,
		CreatedAt: time.Now().UTC(),
	}

	if err := s.store.CreateCluster(r.Context(), cluster, store.ClusterCreds{
		EncryptedKubeconfig: encryptedKubeconfig,
	}); err != nil {
		s.log.Error().Err(err).Msg("create cluster in store")
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to save cluster: %v", err))
		return
	}

	clientType := r.Header.Get("X-Strata-Client")
	if clientType == "" {
		clientType = "web"
	}
	s.recordAction(r.Context(), claims.Subject, clusterID, "create_cluster", name, "success", "", clientType)

	writeJSON(w, http.StatusCreated, map[string]any{"cluster": cluster})
}

// handleDeleteCluster deletes a user's cluster and associated credentials.
func (s *Server) handleDeleteCluster(w http.ResponseWriter, r *http.Request) {
	claims, _ := claimsFrom(r)
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "no claims")
		return
	}
	clusterID := chi.URLParam(r, "id")
	if clusterID == "" {
		writeError(w, http.StatusBadRequest, "cluster id required")
		return
	}
	err := s.store.DeleteCluster(r.Context(), claims.Subject, clusterID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "cluster not found")
			return
		}
		s.log.Error().Err(err).Str("cluster_id", clusterID).Msg("delete cluster")
		writeError(w, http.StatusInternalServerError, "delete cluster failed")
		return
	}

	clientType := r.Header.Get("X-Strata-Client")
	if clientType == "" {
		clientType = "web"
	}
	s.recordAction(r.Context(), claims.Subject, clusterID, "delete_cluster", clusterID, "success", "", clientType)

	writeJSON(w, http.StatusOK, map[string]any{"status": "deleted", "cluster_id": clusterID})
}

func (s *Server) recordAction(ctx context.Context, userID, clusterID, actionType, target, status, details, clientType string) {
	randBytes := make([]byte, 6)
	_, _ = rand.Read(randBytes)
	actID := fmt.Sprintf("act-%x", randBytes)

	if clientType == "" {
		clientType = "tui"
	}
	_ = s.store.RecordAction(ctx, store.ActionHistory{
		ID:         actID,
		UserID:     userID,
		ClusterID:  clusterID,
		ActionType: actionType,
		Target:     target,
		Status:     status,
		Details:    details,
		ClientType: clientType,
		CreatedAt:  time.Now().UTC(),
	})
}

// handleListHistory returns the authenticated user's action audit history.
func (s *Server) handleListHistory(w http.ResponseWriter, r *http.Request) {
	claims, _ := claimsFrom(r)
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "no claims")
		return
	}
	clusterID := r.URL.Query().Get("cluster_id")
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}
	items, err := s.store.ListHistory(r.Context(), claims.Subject, clusterID, limit)
	if err != nil {
		s.log.Error().Err(err).Msg("list history")
		writeError(w, http.StatusInternalServerError, "failed to list history")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"history": items})
}

// handleListClusterHistory returns action audit history for a specific cluster.
func (s *Server) handleListClusterHistory(w http.ResponseWriter, r *http.Request) {
	claims, _ := claimsFrom(r)
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "no claims")
		return
	}
	clusterID := chi.URLParam(r, "id")
	if clusterID == "" {
		writeError(w, http.StatusBadRequest, "cluster id required")
		return
	}
	if _, err := s.store.GetCluster(r.Context(), claims.Subject, clusterID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "cluster not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "cluster lookup failed")
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}
	items, err := s.store.ListHistory(r.Context(), claims.Subject, clusterID, limit)
	if err != nil {
		s.log.Error().Err(err).Str("cluster_id", clusterID).Msg("list cluster history")
		writeError(w, http.StatusInternalServerError, "failed to list cluster history")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"history": items})
}

func attachClusterCreds(cluster *store.Cluster, args map[string]any, headers http.Header) {
	if cluster.EncryptedKubeconfig != "" {
		args["kubeconfig_encrypted"] = cluster.EncryptedKubeconfig
		headers.Set("X-Strata-Encrypted-Kubeconfig", cluster.EncryptedKubeconfig)
	}
	if cluster.KubeconfigPath != "" {
		args["kubeconfig_path"] = cluster.KubeconfigPath
		headers.Set("X-Strata-Kubeconfig", cluster.KubeconfigPath)
	}
}

// handleListPods is the Phase 1 end-to-end flow:
//
//  1. Look up the cluster row (rejects cross-tenant access).
//  2. Call the MCP k8s server's list_pods tool, passing the user's
//     kubeconfig in args/headers so the MCP server can construct
//     a Kubernetes API client.
//  3. Return the tool result as JSON.
func (s *Server) handleListPods(w http.ResponseWriter, r *http.Request) {
	claims, _ := claimsFrom(r)
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "no claims")
		return
	}
	clusterID := chi.URLParam(r, "id")
	if clusterID == "" {
		writeError(w, http.StatusBadRequest, "cluster id required")
		return
	}
	cluster, err := s.store.GetCluster(r.Context(), claims.Subject, clusterID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "cluster not found")
			return
		}
		s.log.Error().Err(err).Str("cluster_id", clusterID).Msg("get cluster")
		writeError(w, http.StatusInternalServerError, "cluster lookup failed")
		return
	}

	// Best-effort upsert of the user so the foreign key stays satisfied
	// even when /me hasn't been called yet.
	_ = s.store.EnsureUser(r.Context(), store.User{
		ID:       claims.Subject,
		Username: firstNonEmpty(claims.Username, claims.Email, claims.Subject),
		Email:    strPtr(claims.Email),
	})

	namespace := r.URL.Query().Get("namespace")
	labelSelector := r.URL.Query().Get("label-selector")

	args := map[string]any{
		"cluster_id": cluster.ID,
	}
	if namespace != "" {
		args["namespace"] = namespace
	} else {
		args["namespace"] = "default"
	}
	if labelSelector != "" {
		args["label_selector"] = labelSelector
	}

	headers := http.Header{}
	headers.Set("X-Strata-User", claims.Subject)
	attachClusterCreds(cluster, args, headers)

	result, err := s.mcp.CallToolWithHeaders(r.Context(), "list_pods", args, headers)
	if err != nil {
		s.log.Error().Err(err).Str("cluster_id", clusterID).Msg("mcp call_tool")
		writeError(w, http.StatusBadGateway, fmt.Sprintf("mcp call failed: %v", err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(result)
}

// handleDeletePod proxies to the MCP k8s delete_pod tool.
func (s *Server) handleDeletePod(w http.ResponseWriter, r *http.Request) {
	claims, _ := claimsFrom(r)
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "no claims")
		return
	}
	clusterID := chi.URLParam(r, "id")
	podName := chi.URLParam(r, "name")
	if clusterID == "" || podName == "" {
		writeError(w, http.StatusBadRequest, "cluster id and pod name required")
		return
	}
	cluster, err := s.store.GetCluster(r.Context(), claims.Subject, clusterID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "cluster not found")
			return
		}
		s.log.Error().Err(err).Str("cluster_id", clusterID).Msg("get cluster")
		writeError(w, http.StatusInternalServerError, "cluster lookup failed")
		return
	}

	namespace := r.URL.Query().Get("namespace")
	if namespace == "" {
		namespace = "default"
	}

	args := map[string]any{
		"cluster_id": cluster.ID,
		"name":       podName,
		"namespace":  namespace,
	}
	if gpStr := r.URL.Query().Get("grace-period-seconds"); gpStr != "" {
		if gp, err := strconv.Atoi(gpStr); err == nil {
			args["grace_period_seconds"] = gp
		}
	}

	headers := http.Header{}
	headers.Set("X-Strata-User", claims.Subject)
	attachClusterCreds(cluster, args, headers)

	result, err := s.mcp.CallToolWithHeaders(r.Context(), "delete_pod", args, headers)
	clientType := r.Header.Get("X-Strata-Client")
	if err != nil {
		s.log.Error().Err(err).Str("cluster_id", clusterID).Str("pod", podName).Msg("mcp delete_pod")
		s.recordAction(r.Context(), claims.Subject, clusterID, "delete_pod", namespace+"/"+podName, "failed", err.Error(), clientType)
		writeError(w, http.StatusBadGateway, fmt.Sprintf("mcp call failed: %v", err))
		return
	}
	s.recordAction(r.Context(), claims.Subject, clusterID, "delete_pod", namespace+"/"+podName, "success", "", clientType)
	w.Header().Set("Content-Type", "application/json")
	w.Write(result)
}

type ApplyRequest struct {
	Manifest  string `json:"manifest"`
	Namespace string `json:"namespace"`
}

// handleApplyManifest proxies to the MCP k8s apply_manifest tool.
func (s *Server) handleApplyManifest(w http.ResponseWriter, r *http.Request) {
	claims, _ := claimsFrom(r)
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "no claims")
		return
	}
	clusterID := chi.URLParam(r, "id")
	if clusterID == "" {
		writeError(w, http.StatusBadRequest, "cluster id required")
		return
	}
	cluster, err := s.store.GetCluster(r.Context(), claims.Subject, clusterID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "cluster not found")
			return
		}
		s.log.Error().Err(err).Str("cluster_id", clusterID).Msg("get cluster")
		writeError(w, http.StatusInternalServerError, "cluster lookup failed")
		return
	}

	var req ApplyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "malformed request body")
		return
	}
	if strings.TrimSpace(req.Manifest) == "" {
		writeError(w, http.StatusBadRequest, "manifest is required")
		return
	}
	if req.Namespace == "" {
		req.Namespace = "default"
	}

	args := map[string]any{
		"cluster_id":    cluster.ID,
		"manifest_yaml": req.Manifest,
		"namespace":     req.Namespace,
	}

	headers := http.Header{}
	headers.Set("X-Strata-User", claims.Subject)
	attachClusterCreds(cluster, args, headers)

	result, err := s.mcp.CallToolWithHeaders(r.Context(), "apply_manifest", args, headers)
	clientType := r.Header.Get("X-Strata-Client")
	if err != nil {
		s.log.Error().Err(err).Str("cluster_id", clusterID).Msg("mcp apply_manifest")
		s.recordAction(r.Context(), claims.Subject, clusterID, "apply_manifest", req.Namespace, "failed", err.Error(), clientType)
		writeError(w, http.StatusBadGateway, fmt.Sprintf("mcp call failed: %v", err))
		return
	}
	s.recordAction(r.Context(), claims.Subject, clusterID, "apply_manifest", req.Namespace, "success", "", clientType)
	w.Header().Set("Content-Type", "application/json")
	w.Write(result)
}

type ExecRequest struct {
	Command   any    `json:"command"`
	Namespace string `json:"namespace"`
	Container string `json:"container"`
}

// handleExecCommand proxies to the MCP k8s exec_command tool.
func (s *Server) handleExecCommand(w http.ResponseWriter, r *http.Request) {
	claims, _ := claimsFrom(r)
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "no claims")
		return
	}
	clusterID := chi.URLParam(r, "id")
	podName := chi.URLParam(r, "name")
	if clusterID == "" || podName == "" {
		writeError(w, http.StatusBadRequest, "cluster id and pod name required")
		return
	}
	cluster, err := s.store.GetCluster(r.Context(), claims.Subject, clusterID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "cluster not found")
			return
		}
		s.log.Error().Err(err).Str("cluster_id", clusterID).Msg("get cluster")
		writeError(w, http.StatusInternalServerError, "cluster lookup failed")
		return
	}

	var req ExecRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "malformed request body")
		return
	}
	if req.Command == nil {
		writeError(w, http.StatusBadRequest, "command is required")
		return
	}
	if req.Namespace == "" {
		req.Namespace = "default"
	}

	args := map[string]any{
		"cluster_id": cluster.ID,
		"pod":        podName,
		"command":    req.Command,
		"namespace":  req.Namespace,
	}
	if req.Container != "" {
		args["container"] = req.Container
	}

	headers := http.Header{}
	headers.Set("X-Strata-User", claims.Subject)
	attachClusterCreds(cluster, args, headers)

	result, err := s.mcp.CallToolWithHeaders(r.Context(), "exec_command", args, headers)
	clientType := r.Header.Get("X-Strata-Client")
	cmdStr := fmt.Sprintf("%v", req.Command)
	if err != nil {
		s.log.Error().Err(err).Str("cluster_id", clusterID).Str("pod", podName).Msg("mcp exec_command")
		s.recordAction(r.Context(), claims.Subject, clusterID, "exec_command", req.Namespace+"/"+podName, "failed", err.Error(), clientType)
		writeError(w, http.StatusBadGateway, fmt.Sprintf("mcp call failed: %v", err))
		return
	}
	s.recordAction(r.Context(), claims.Subject, clusterID, "exec_command", req.Namespace+"/"+podName, "success", cmdStr, clientType)
	w.Header().Set("Content-Type", "application/json")
	w.Write(result)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// requestLogger is a chi middleware that logs each request.
func (s *Server) requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		s.log.Info().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Int("status", ww.Status()).
			Dur("dur", time.Since(start)).
			Msg("request")
	})
}

// authMiddleware validates the Authorization: Bearer <jwt> header
// against Keycloak. The dev-only BOOTSTRAP_ADMIN_TOKEN bypasses JWT
// validation so smoke tests work before Keycloak is reachable.
func (s *Server) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		raw, err := extractBearer(r.Header.Get("Authorization"))
		if err != nil {
			writeError(w, http.StatusUnauthorized, err.Error())
			return
		}
		if s.cfg.BootstrapAdminToken != "" && raw == s.cfg.BootstrapAdminToken {
			// Dev shortcut: synthesize claims so handlers can still
			// pull a user identity. The "subject" is the literal
			// string "bootstrap-admin".
			ctx := r.Context()
			ctx = withClaims(ctx, &jwks.Claims{
				Subject:  "bootstrap-admin",
				Username: "bootstrap-admin",
				Audience: []string{s.cfg.JWTAudience},
			})
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}
		claims, err := s.jwks.Validate(r.Context(), raw)
		if err != nil {
			s.log.Warn().Err(err).Msg("jwt validation failed")
			writeError(w, http.StatusUnauthorized, "invalid token")
			return
		}
		ctx := withClaims(r.Context(), claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

func extractBearer(header string) (string, error) {
	if header == "" {
		return "", errors.New("missing Authorization header")
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", errors.New("malformed Authorization header")
	}
	tok := strings.TrimSpace(header[len(prefix):])
	if tok == "" {
		return "", errors.New("empty bearer token")
	}
	return tok, nil
}

// claimsFrom is a typed accessor for the request's validated claims.
func claimsFrom(r *http.Request) (*jwks.Claims, bool) {
	c, ok := r.Context().Value(claimsKey{}).(*jwks.Claims)
	return c, ok
}

type claimsKey struct{}

// withClaims stores the validated claims on the request context.
func withClaims(ctx context.Context, c *jwks.Claims) context.Context {
	return context.WithValue(ctx, claimsKey{}, c)
}

// writeJSON marshals v as JSON to w with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error envelope. The optional details param
// is included under the "details" key.
func writeError(w http.ResponseWriter, status int, msg string, details ...any) {
	body := map[string]any{"error": msg}
	if len(details) > 0 {
		body["details"] = details[0]
	}
	writeJSON(w, status, body)
}
