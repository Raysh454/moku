package server

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"

	"github.com/raysh454/moku/internal/app"
	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/registry"

	_ "modernc.org/sqlite" // SQLite driver
)

// Server is the HTTP + WebSocket API surface for Moku.
type Server struct {
	cfg          Config
	orchestrator *app.Orchestrator
	router       chi.Router
	upgrader     websocket.Upgrader
	logger       logging.Logger
	registryDB   *sql.DB
}

// NewServer creates a new Server with its own Orchestrator.
func NewServer(cfg Config) (*Server, error) {
	if cfg.AppConfig == nil {
		cfg.AppConfig = app.DefaultConfig()
	}

	logger := cfg.Logger
	if logger == nil {
		logger = logging.NewStdoutLogger("Server")
	}

	// Make sure storage root exists
	storageRoot, err := expandPath(cfg.AppConfig.StorageRoot)
	if err != nil {
		return nil, fmt.Errorf("expanding storage root path: %w", err)
	}
	cfg.AppConfig.StorageRoot = storageRoot
	err = os.MkdirAll(cfg.AppConfig.StorageRoot, 0755)
	if err != nil {
		logger.Warn("creating storage root directory", logging.Field{Key: "path", Value: cfg.AppConfig.StorageRoot}, logging.Field{Key: "error", Value: err.Error()})
	}

	// Set up registry DB
	db, err := sql.Open("sqlite", filepath.Join(cfg.AppConfig.StorageRoot, "registry.db"))
	if err != nil {
		return nil, fmt.Errorf("opening registry database: %w", err)
	}

	reg, err := registry.NewRegistry(db, cfg.AppConfig.StorageRoot, logger)
	if err != nil {
		return nil, fmt.Errorf("creating registry: %w", err)
	}

	orch := app.NewOrchestrator(cfg.AppConfig, reg, logger)

	r := chi.NewRouter()
	s := &Server{
		cfg:          cfg,
		orchestrator: orch,
		router:       r,
		logger:       logger,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// TODO: tighten for production
				return true
			},
		},
		registryDB: db,
	}

	s.routes()
	return s, nil
}

// Orchestrator returns the underlying orchestrator for advanced use (tests, etc.).
func (s *Server) Orchestrator() *app.Orchestrator {
	return s.orchestrator
}

func (s *Server) routes() {
	r := s.router

	r.Use(s.corsMiddleware)

	// CORS preflight
	r.Options("/projects", s.optionsHandler("GET, POST"))
	r.Options("/projects/{project}/websites", s.optionsHandler("GET, POST"))
	r.Options("/projects/{project}/websites/{site}/endpoints", s.optionsHandler("GET, POST"))
	r.Options("/projects/{project}/websites/{site}/endpoints/details", s.optionsHandler("GET"))
	r.Options("/projects/{project}/websites/{site}/jobs/fetch", s.optionsHandler("POST"))
	r.Options("/projects/{project}/websites/{site}/jobs/enumerate", s.optionsHandler("POST"))
	r.Options("/jobs", s.optionsHandler("GET"))
	r.Options("/jobs/{jobID}", s.optionsHandler("GET, DELETE"))
	r.Options("/ws/projects/{project}/websites/{site}/fetch", s.optionsHandler("GET"))
	r.Options("/ws/projects/{project}/websites/{site}/enumerate", s.optionsHandler("GET"))

	// Projects
	r.Post("/projects", s.handleCreateProject)
	r.Get("/projects", s.handleListProjects)

	// Websites
	r.Post("/projects/{project}/websites", s.handleCreateWebsite)
	r.Get("/projects/{project}/websites", s.handleListWebsites)

	// Endpoints
	r.Post("/projects/{project}/websites/{site}/endpoints", s.handleAddWebsiteEndpoints)
	r.Get("/projects/{project}/websites/{site}/endpoints", s.handleListWebsiteEndpoints)
	r.Get("/projects/{project}/websites/{site}/endpoints/details", s.handleGetEndpointDetails)

	// Jobs over REST
	r.Post("/projects/{project}/websites/{site}/jobs/fetch", s.handleStartFetchJob)
	r.Post("/projects/{project}/websites/{site}/jobs/enumerate", s.handleStartEnumerateJob)
	r.Get("/jobs", s.handleListJobs)
	r.Get("/jobs/{jobID}", s.handleGetJob)
	r.Delete("/jobs/{jobID}", s.handleCancelJob)

	// WebSockets for job progress
	r.Get("/ws/projects/{project}/websites/{site}/fetch", s.handleFetchWS)
	r.Get("/ws/projects/{project}/websites/{site}/enumerate", s.handleEnumerateWS)
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")

		next.ServeHTTP(w, r)
	})
}

func (s *Server) optionsHandler(methods string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Methods", methods)
		w.WriteHeader(http.StatusNoContent)
	}
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fields := []logging.Field{
		{Key: "method", Value: r.Method},
		{Key: "path", Value: r.URL.Path},
	}

	if q := r.URL.Query(); len(q) > 0 {
		fields = append(fields, logging.Field{Key: "query", Value: q})
	}

	if r.Body != nil && (r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch) {
		if bodyBytes, err := io.ReadAll(r.Body); err == nil {
			fields = append(fields, logging.Field{Key: "body", Value: string(bodyBytes)})
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}
	}

	s.logger.Info("http_request", fields...)

	s.router.ServeHTTP(w, r)
}

// Close shuts down the orchestrator and underlying resources.
func (s *Server) Close() {
	if s.registryDB != nil {
		s.registryDB.Close()
	}
	if s.orchestrator != nil {
		s.orchestrator.Close()
	}
}

// HTTPServer creates an *http.Server ready to ListenAndServe.
func (s *Server) HTTPServer() *http.Server {
	return &http.Server{
		Addr:         s.cfg.ListenAddr,
		Handler:      s,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 0, // allow streaming
	}
}

// --- JSON helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// --- HTTP handlers ---

// Projects

func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Slug        string `json:"slug"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	p, err := s.orchestrator.CreateProject(r.Context(), body.Slug, body.Name, body.Description)
	if err != nil {
		s.logger.Warn("creating project", logging.Field{Key: "error", Value: err.Error()})
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.logger.Info("created project", logging.Field{Key: "slug", Value: p.Slug})
	writeJSON(w, http.StatusCreated, p)
}

func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	ps, err := s.orchestrator.ListProjects(r.Context())
	if err != nil {
		s.logger.Warn("listing projects", logging.Field{Key: "error", Value: err.Error()})
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.logger.Info("listed projects", logging.Field{Key: "count", Value: len(ps)})
	writeJSON(w, http.StatusOK, ps)
}

// Websites

func (s *Server) handleCreateWebsite(w http.ResponseWriter, r *http.Request) {
	project := chi.URLParam(r, "project")

	var body struct {
		Slug   string `json:"slug"`
		Origin string `json:"origin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		s.logger.Warn("decoding create website body", logging.Field{Key: "error", Value: err.Error()})
		return
	}

	web, err := s.orchestrator.CreateWebsite(r.Context(), project, body.Slug, body.Origin)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		s.logger.Warn("creating website", logging.Field{Key: "error", Value: err.Error()})
		return
	}
	s.logger.Info("created website", logging.Field{Key: "project", Value: project}, logging.Field{Key: "site", Value: web.Slug})
	writeJSON(w, http.StatusCreated, web)
}

func (s *Server) handleListWebsites(w http.ResponseWriter, r *http.Request) {
	project := chi.URLParam(r, "project")

	ws, err := s.orchestrator.ListWebsites(r.Context(), project)
	if err != nil {
		s.logger.Warn("listing websites", logging.Field{Key: "error", Value: err.Error()})
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.logger.Info("listed websites", logging.Field{Key: "project", Value: project}, logging.Field{Key: "count", Value: len(ws)})
	writeJSON(w, http.StatusOK, ws)
}

// Endpoints

func (s *Server) handleAddWebsiteEndpoints(w http.ResponseWriter, r *http.Request) {
	project := chi.URLParam(r, "project")
	site := chi.URLParam(r, "site")

	var body struct {
		URLs   []string `json:"urls"`
		Source string   `json:"source"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.logger.Warn("decoding add endpoints body", logging.Field{Key: "error", Value: err.Error()})
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if body.Source == "" {
		body.Source = "api"
	}

	added, err := s.orchestrator.AddWebsiteEndpoints(r.Context(), project, site, body.URLs, body.Source)
	if err != nil {
		s.logger.Warn("adding website endpoints", logging.Field{Key: "error", Value: err.Error()})
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.logger.Info("added website endpoints", logging.Field{Key: "project", Value: project}, logging.Field{Key: "site", Value: site}, logging.Field{Key: "added_count", Value: added})
	writeJSON(w, http.StatusCreated, map[string]any{"added": added})
}

func (s *Server) handleListWebsiteEndpoints(w http.ResponseWriter, r *http.Request) {
	project := chi.URLParam(r, "project")
	site := chi.URLParam(r, "site")
	status := r.URL.Query().Get("status")
	limitStr := r.URL.Query().Get("limit")

	limit := 0
	if limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
			limit = v
		}
	}

	eps, err := s.orchestrator.ListWebsiteEndpoints(r.Context(), project, site, status, limit)
	if err != nil {
		s.logger.Warn("listing website endpoints", logging.Field{Key: "error", Value: err.Error()})
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.logger.Info("listed website endpoints", logging.Field{Key: "project", Value: project}, logging.Field{Key: "site", Value: site}, logging.Field{Key: "count", Value: len(eps)})
	writeJSON(w, http.StatusOK, eps)
}

func (s *Server) handleGetEndpointDetails(w http.ResponseWriter, r *http.Request) {
	project := chi.URLParam(r, "project")
	site := chi.URLParam(r, "site")
	url := r.URL.Query().Get("url")
	if url == "" {
		s.logger.Warn("getting endpoint details: missing url query parameter")
		writeError(w, http.StatusBadRequest, "missing url query parameter")
		return
	}

	details, err := s.orchestrator.GetEndpointDetails(r.Context(), project, site, url)
	if err != nil {
		s.logger.Warn("getting endpoint details", logging.Field{Key: "error", Value: err.Error()})
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.logger.Info("got endpoint details", logging.Field{Key: "project", Value: project}, logging.Field{Key: "site", Value: site}, logging.Field{Key: "url", Value: url})
	writeJSON(w, http.StatusOK, details)
}

// Jobs (REST)

func (s *Server) handleStartFetchJob(w http.ResponseWriter, r *http.Request) {
	project := chi.URLParam(r, "project")
	site := chi.URLParam(r, "site")

	var body struct {
		Status string `json:"status"`
		Limit  int    `json:"limit"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	//  Use defaults if not provided
	if body.Status == "" {
		body.Status = "new"
	}
	if body.Limit <= 0 {
		body.Limit = 100
	}

	job, err := s.orchestrator.StartFetchJob(context.Background(), project, site, body.Status, body.Limit)
	if err != nil {
		s.logger.Warn("starting fetch job", logging.Field{Key: "error", Value: err.Error()})
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.logger.Info("started fetch job", logging.Field{Key: "job_id", Value: job.ID}, logging.Field{Key: "status", Value: body.Status}, logging.Field{Key: "limit", Value: body.Limit})
	writeJSON(w, http.StatusAccepted, job)
}

func (s *Server) handleStartEnumerateJob(w http.ResponseWriter, r *http.Request) {
	project := chi.URLParam(r, "project")
	site := chi.URLParam(r, "site")

	maxDepth := 4

	job, err := s.orchestrator.StartEnumerateJob(r.Context(), project, site, maxDepth)
	if err != nil {
		s.logger.Warn("starting enumerate job", logging.Field{Key: "error", Value: err.Error()})
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.logger.Info("started enumerate job", logging.Field{Key: "job_id", Value: job.ID})
	writeJSON(w, http.StatusAccepted, job)
}

func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")
	job := s.orchestrator.GetJob(jobID)
	if job == nil {
		s.logger.Warn("getting job: not found", logging.Field{Key: "job_id", Value: jobID})
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	s.logger.Info("got job", logging.Field{Key: "job_id", Value: job.ID})
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) handleCancelJob(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")
	s.orchestrator.CancelJob(jobID)
	s.logger.Info("canceled job", logging.Field{Key: "job_id", Value: jobID})
	writeJSON(w, http.StatusNoContent, nil)
}

func (s *Server) handleListJobs(w http.ResponseWriter, r *http.Request) {
	jobs := s.orchestrator.ListJobs()
	s.logger.Info("listed jobs", logging.Field{Key: "count", Value: len(jobs)})
	writeJSON(w, http.StatusOK, jobs)
}

// WebSockets

func (s *Server) handleFetchWS(w http.ResponseWriter, r *http.Request) {
	project := chi.URLParam(r, "project")
	site := chi.URLParam(r, "site")

	status := r.URL.Query().Get("status")
	if status == "" {
		status = "new"
	}
	limit := 100
	if ls := r.URL.Query().Get("limit"); ls != "" {
		if v, err := strconv.Atoi(ls); err == nil && v > 0 {
			limit = v
		}
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Warn("upgrading to websocket", logging.Field{Key: "error", Value: err.Error()})
		return
	}
	defer conn.Close()

	ctx := r.Context()

	job, err := s.orchestrator.StartFetchJob(ctx, project, site, status, limit)
	if err != nil {
		s.logger.Warn("starting fetch job", logging.Field{Key: "error", Value: err.Error()})
		_ = conn.WriteJSON(map[string]string{"error": err.Error()})
		return
	}

	// Optionally send initial job struct
	s.logger.Info("started fetch job", logging.Field{Key: "job_id", Value: job.ID})
	_ = conn.WriteJSON(job)

	for ev := range job.Events {
		if err := conn.WriteJSON(ev); err != nil {
			// Assume client disconnected; cancel job
			s.orchestrator.CancelJob(job.ID)
			return
		}
	}
}

func (s *Server) handleEnumerateWS(w http.ResponseWriter, r *http.Request) {
	project := chi.URLParam(r, "project")
	site := chi.URLParam(r, "site")

	maxDepth := 4

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Warn("upgrading to websocket", logging.Field{Key: "error", Value: err.Error()})
		return
	}
	defer conn.Close()

	ctx := r.Context()

	job, err := s.orchestrator.StartEnumerateJob(ctx, project, site, maxDepth)
	if err != nil {
		_ = conn.WriteJSON(map[string]string{"error": err.Error()})
		s.logger.Warn("starting enumerate job", logging.Field{Key: "error", Value: err.Error()})
		return
	}

	s.logger.Info("started enumerate job", logging.Field{Key: "job_id", Value: job.ID})
	_ = conn.WriteJSON(job)

	for ev := range job.Events {
		if err := conn.WriteJSON(ev); err != nil {
			s.orchestrator.CancelJob(job.ID)
			return
		}
	}
}

func expandPath(p string) (string, error) {
	if len(p) > 0 && p[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, p[1:]), nil
	}
	return p, nil
}
