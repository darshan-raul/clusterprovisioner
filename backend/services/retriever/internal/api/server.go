package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"

	"github.com/strata/retriever/internal/config"
	"github.com/strata/retriever/internal/embedder"
	"github.com/strata/retriever/internal/vectorstore"
	"github.com/strata/shared/pkg/jwks"
)

type Server struct {
	cfg      config.Config
	log      zerolog.Logger
	jwks     *jwks.Validator
	embedder embedder.Embedder
	store    vectorstore.Store
}

func New(
	cfg config.Config,
	log zerolog.Logger,
	jwksValidator *jwks.Validator,
	emb embedder.Embedder,
	st vectorstore.Store,
) *Server {
	return &Server{
		cfg:      cfg,
		log:      log,
		jwks:     jwksValidator,
		embedder: emb,
		store:    st,
	}
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(s.requestLogger)
	r.Use(chimw.Recoverer)

	r.Get("/healthz", s.handleHealthz)

	r.Group(func(r chi.Router) {
		r.Use(s.authMiddleware)
		r.Post("/retrieve", s.handleRetrieve)
		r.Post("/index", s.handleIndex)
		r.Delete("/index/{collection}/*", s.handleDelete)
	})

	return r
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	err := s.store.Health(r.Context())
	status := "ok"
	if err != nil {
		status = "degraded"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":       status,
		"vector_store": status,
		"dimension":    s.embedder.Dimension(),
	})
}

type RetrieveRequest struct {
	Collection string         `json:"collection"`
	Query      string         `json:"query"`
	TopK       int            `json:"top_k,omitempty"`
	Filter     map[string]any `json:"filter,omitempty"`
}

func (s *Server) handleRetrieve(w http.ResponseWriter, r *http.Request) {
	userID := userFrom(r)
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "missing user identity")
		return
	}

	var req RetrieveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "malformed request body")
		return
	}

	colName := strings.TrimSpace(req.Collection)
	if colName == "" {
		colName = "clusters"
	}
	query := strings.TrimSpace(req.Query)
	if query == "" {
		writeError(w, http.StatusBadRequest, "query is required")
		return
	}

	topK := req.TopK
	if topK <= 0 || topK > 50 {
		topK = 5
	}

	scopedCollection := UserScopedCollection(userID, colName)

	queryVec, err := s.embedder.Embed(r.Context(), query)
	if err != nil {
		s.log.Error().Err(err).Str("user", userID).Msg("embed query failed")
		writeError(w, http.StatusInternalServerError, "failed to generate query embedding")
		return
	}

	filter := req.Filter
	if filter == nil {
		filter = make(map[string]any)
	}
	filter["user_id"] = userID

	results, err := s.store.Search(r.Context(), scopedCollection, queryVec, topK, filter)
	if err != nil {
		s.log.Error().Err(err).Str("collection", scopedCollection).Msg("vector search failed")
		writeError(w, http.StatusInternalServerError, "vector search failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"chunks": results,
		"count":  len(results),
	})
}

type IndexRequest struct {
	Collection string         `json:"collection"`
	ID         string         `json:"id"`
	Text       string         `json:"text"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	userID := userFrom(r)
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "missing user identity")
		return
	}

	var req IndexRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "malformed request body")
		return
	}

	colName := strings.TrimSpace(req.Collection)
	if colName == "" {
		colName = "clusters"
	}
	id := strings.TrimSpace(req.ID)
	if id == "" {
		writeError(w, http.StatusBadRequest, "point id is required")
		return
	}
	text := strings.TrimSpace(req.Text)
	if text == "" {
		writeError(w, http.StatusBadRequest, "point text is required")
		return
	}

	scopedCollection := UserScopedCollection(userID, colName)

	vec, err := s.embedder.Embed(r.Context(), text)
	if err != nil {
		s.log.Error().Err(err).Msg("embed point failed")
		writeError(w, http.StatusInternalServerError, "failed to generate vector embedding")
		return
	}

	meta := req.Metadata
	if meta == nil {
		meta = make(map[string]any)
	}
	meta["user_id"] = userID

	p := vectorstore.Point{
		ID:       id,
		Vector:   vec,
		Text:     text,
		Metadata: meta,
	}

	if err := s.store.Upsert(r.Context(), scopedCollection, []vectorstore.Point{p}); err != nil {
		s.log.Error().Err(err).Str("collection", scopedCollection).Msg("upsert point failed")
		writeError(w, http.StatusInternalServerError, "upsert point failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"upserted": true,
		"id":       id,
	})
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	userID := userFrom(r)
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "missing user identity")
		return
	}

	colName := chi.URLParam(r, "collection")
	id := chi.URLParam(r, "*")
	if id == "" {
		id = chi.URLParam(r, "id")
	}
	id = strings.TrimPrefix(id, "/")
	if colName == "" || id == "" {
		writeError(w, http.StatusBadRequest, "collection and id required")
		return
	}

	scopedCollection := UserScopedCollection(userID, colName)

	if err := s.store.Delete(r.Context(), scopedCollection, id); err != nil {
		s.log.Error().Err(err).Str("collection", scopedCollection).Str("id", id).Msg("delete point failed")
		writeError(w, http.StatusInternalServerError, "delete point failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"deleted": true,
		"id":      id,
	})
}

func UserScopedCollection(userID, collection string) string {
	cleanUser := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return '_'
	}, userID)
	cleanCol := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return '_'
	}, collection)
	return fmt.Sprintf("user_%s_%s", cleanUser, cleanCol)
}

type userKey struct{}

func userFrom(r *http.Request) string {
	if u, ok := r.Context().Value(userKey{}).(string); ok {
		return u
	}
	return ""
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Check internal service header X-Strata-User
		if u := strings.TrimSpace(r.Header.Get("X-Strata-User")); u != "" {
			ctx := context.WithValue(r.Context(), userKey{}, u)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// 2. Check Authorization Bearer token against JWKS if configured
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") && s.jwks != nil {
			token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
			claims, err := s.jwks.Validate(r.Context(), token)
			if err == nil && claims != nil && claims.Subject != "" {
				ctx := context.WithValue(r.Context(), userKey{}, claims.Subject)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}

		writeError(w, http.StatusUnauthorized, "unauthorized: missing valid token or X-Strata-User header")
	})
}

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
			Msg("retriever_request")
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg})
}
