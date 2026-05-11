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
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	httpSwagger "github.com/swaggo/http-swagger/v2"

	_ "github.com/raysh454/moku/docs/swagger"
	"github.com/raysh454/moku/internal/analyzer"
	"github.com/raysh454/moku/internal/api"
	"github.com/raysh454/moku/internal/app"
	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/registry"

	_ "modernc.org/sqlite" // SQLite driver
)

// Server is the HTTP + WebSocket API surface for Moku.
type Server struct {
	cfg            Config
	orchestrator   *app.Orchestrator
	router         chi.Router
	upgrader       websocket.Upgrader
	logger         logging.Logger
	registryDB     *sql.DB
	allowedOrigins []string
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

	// Seed default filter rules for existing websites that don't have any
	ctx := context.Background()
	if err := orch.SeedDefaultFiltersForAllWebsites(ctx); err != nil {
		logger.Warn("seeding default filter rules", logging.Field{Key: "error", Value: err.Error()})
	}

	r := chi.NewRouter()
	s := &Server{
		cfg:            cfg,
		orchestrator:   orch,
		router:         r,
		logger:         logger,
		registryDB:     db,
		allowedOrigins: resolveAllowedOrigins(cfg),
	}
	s.upgrader = websocket.Upgrader{CheckOrigin: s.checkWebSocketOrigin}

	s.routes()
	return s, nil
}

// checkWebSocketOrigin enforces the configured origin allowlist on WebSocket
// upgrades. Non-browser clients (no Origin header) are always allowed.
func (s *Server) checkWebSocketOrigin(r *http.Request) bool {
	return isOriginAllowed(r.Header.Get("Origin"), s.allowedOrigins)
}

// Orchestrator returns the underlying orchestrator for advanced use (tests, etc.).
func (s *Server) Orchestrator() *app.Orchestrator {
	return s.orchestrator
}

func (s *Server) routes() {
	r := s.router

	r.Use(s.corsMiddleware)

	// Interactive Swagger docs (development helper)
	r.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))

	// CORS preflight
	r.Options("/projects", s.optionsHandler("GET, POST"))
	r.Options("/projects/{project}/websites", s.optionsHandler("GET, POST"))
	r.Options("/projects/{project}/websites/{site}/endpoints", s.optionsHandler("GET, POST"))
	r.Options("/projects/{project}/websites/{site}/endpoints/details", s.optionsHandler("GET"))
	r.Options("/projects/{project}/websites/{site}/jobs/fetch", s.optionsHandler("POST"))
	r.Options("/projects/{project}/websites/{site}/jobs/enumerate", s.optionsHandler("POST"))
	r.Options("/projects/{project}/websites/{site}/jobs/scan", s.optionsHandler("POST"))
	r.Options("/projects/{project}/websites/{site}/analyzer/capabilities", s.optionsHandler("GET"))
	r.Options("/projects/{project}/websites/{site}/analyzer/health", s.optionsHandler("GET"))
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

	// Versions
	r.Get("/projects/{project}/websites/{site}/versions", s.handleListVersions)

	// Jobs over REST
	r.Post("/projects/{project}/websites/{site}/jobs/fetch", s.handleStartFetchJob)
	r.Post("/projects/{project}/websites/{site}/jobs/enumerate", s.handleStartEnumerateJob)
	r.Post("/projects/{project}/websites/{site}/jobs/scan", s.handleStartScanJob)
	r.Get("/projects/{project}/websites/{site}/analyzer/capabilities", s.handleGetAnalyzerCapabilities)
	r.Get("/projects/{project}/websites/{site}/analyzer/health", s.handleGetAnalyzerHealth)
	r.Get("/jobs", s.handleListJobs)
	r.Get("/jobs/{jobID}", s.handleGetJob)
	r.Delete("/jobs/{jobID}", s.handleCancelJob)

	// WebSockets for job progress
	r.Get("/ws/projects/{project}/websites/{site}/fetch", s.handleFetchWS)
	r.Get("/ws/projects/{project}/websites/{site}/enumerate", s.handleEnumerateWS)

	// Filter Rules CRUD
	r.Options("/projects/{project}/websites/{site}/filters", s.optionsHandler("GET, POST"))
	r.Options("/projects/{project}/websites/{site}/filters/{ruleID}", s.optionsHandler("GET, PUT, DELETE"))
	r.Options("/projects/{project}/websites/{site}/filters/config", s.optionsHandler("GET, PUT"))
	r.Options("/projects/{project}/websites/{site}/endpoints/filtered", s.optionsHandler("GET"))
	r.Options("/projects/{project}/websites/{site}/endpoints/unfilter", s.optionsHandler("POST"))
	r.Options("/projects/{project}/websites/{site}/endpoints/stats", s.optionsHandler("GET"))

	r.Get("/projects/{project}/websites/{site}/filters", s.handleListFilterRules)
	r.Post("/projects/{project}/websites/{site}/filters", s.handleCreateFilterRule)
	r.Get("/projects/{project}/websites/{site}/filters/{ruleID}", s.handleGetFilterRule)
	r.Put("/projects/{project}/websites/{site}/filters/{ruleID}", s.handleUpdateFilterRule)
	r.Delete("/projects/{project}/websites/{site}/filters/{ruleID}", s.handleDeleteFilterRule)
	r.Post("/projects/{project}/websites/{site}/filters/{ruleID}/toggle", s.handleToggleFilterRule)

	// Filter Config (website-level quick config)
	r.Get("/projects/{project}/websites/{site}/filters/config", s.handleGetFilterConfig)
	r.Put("/projects/{project}/websites/{site}/filters/config", s.handleUpdateFilterConfig)

	// Apply filters to existing endpoints
	r.Options("/projects/{project}/websites/{site}/filters/apply", s.optionsHandler("POST"))
	r.Post("/projects/{project}/websites/{site}/filters/apply", s.handleApplyFilters)

	// Filtered endpoints and stats
	r.Post("/projects/{project}/websites/{site}/endpoints/unfilter", s.handleUnfilterEndpoints)
	r.Get("/projects/{project}/websites/{site}/endpoints/stats", s.handleEndpointStats)
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")
		s.applyAllowOriginHeader(w, r)

		next.ServeHTTP(w, r)
	})
}

// applyAllowOriginHeader writes the appropriate Access-Control-Allow-Origin
// header based on the configured allowlist. With no allowlist (or "*"), the
// response keeps the permissive dev default. With a concrete allowlist, the
// caller's Origin is reflected only if it matches; otherwise no header is
// written at all (browsers will then block the response).
func (s *Server) applyAllowOriginHeader(w http.ResponseWriter, r *http.Request) {
	if isPermissiveAllowlist(s.allowedOrigins) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		return
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return
	}
	if !isOriginAllowed(origin, s.allowedOrigins) {
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Add("Vary", "Origin")
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
	writeJSON(w, status, ErrorResponse{Error: msg})
}

// --- HTTP handlers ---

// Projects

// handleCreateProject godoc
// @Summary Create a project
// @Description Creates a named project that groups websites.
// @Tags Projects
// @Accept json
// @Produce json
// @Param request body CreateProjectRequest true "Project payload"
// @Success 201 {object} registry.Project
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /projects [post]
func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	var body CreateProjectRequest
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

// handleListProjects godoc
// @Summary List projects
// @Description Returns all configured projects.
// @Tags Projects
// @Produce json
// @Success 200 {array} registry.Project
// @Failure 500 {object} ErrorResponse
// @Router /projects [get]
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

// handleCreateWebsite godoc
// @Summary Create a website inside a project
// @Description Registers a website origin scoped to the provided project slug.
// @Tags Websites
// @Accept json
// @Produce json
// @Param project path string true "Project slug"
// @Param request body CreateWebsiteRequest true "Website payload"
// @Success 201 {object} registry.Website
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /projects/{project}/websites [post]
func (s *Server) handleCreateWebsite(w http.ResponseWriter, r *http.Request) {
	project := chi.URLParam(r, "project")

	var body CreateWebsiteRequest
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

// handleListWebsites godoc
// @Summary List websites in a project
// @Description Retrieves websites scoped to a project slug.
// @Tags Websites
// @Produce json
// @Param project path string true "Project slug"
// @Success 200 {array} registry.Website
// @Failure 500 {object} ErrorResponse
// @Router /projects/{project}/websites [get]
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

// handleAddWebsiteEndpoints godoc
// @Summary Add website endpoints
// @Description Adds canonicalized URLs to the endpoint index for a website.
// @Tags Endpoints
// @Accept json
// @Produce json
// @Param project path string true "Project slug"
// @Param site path string true "Website slug"
// @Param request body AddWebsiteEndpointsRequest true "Endpoint payload"
// @Success 201 {object} AddedEndpointsResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /projects/{project}/websites/{site}/endpoints [post]
func (s *Server) handleAddWebsiteEndpoints(w http.ResponseWriter, r *http.Request) {
	project := chi.URLParam(r, "project")
	site := chi.URLParam(r, "site")

	var body AddWebsiteEndpointsRequest
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
	s.logger.Info("added website endpoints", logging.Field{Key: "project", Value: project}, logging.Field{Key: "site", Value: site}, logging.Field{Key: "added_count", Value: len(added)})
	writeJSON(w, http.StatusCreated, AddedEndpointsResponse{Added: len(added)})
}

// handleListWebsiteEndpoints godoc
// @Summary List website endpoints
// @Description Lists endpoints for the given project/site combination.
// @Tags Endpoints
// @Produce json
// @Param project path string true "Project slug"
// @Param site path string true "Website slug"
// @Param status query string false "Filter by status" default(all)
// @Param limit query int false "Maximum results" default(100)
// @Success 200 {array} indexer.Endpoint
// @Failure 500 {object} ErrorResponse
// @Router /projects/{project}/websites/{site}/endpoints [get]
func (s *Server) handleListWebsiteEndpoints(w http.ResponseWriter, r *http.Request) {
	project := chi.URLParam(r, "project")
	site := chi.URLParam(r, "site")
	status := normalizeEndpointStatus(r.URL.Query().Get("status"))
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

// handleGetEndpointDetails godoc
// @Summary Get detailed endpoint information
// @Description Returns snapshot, scoring, diff, and security info for a canonical URL. Optionally compare specific versions.
// @Tags Endpoints
// @Produce json
// @Param project path string true "Project slug"
// @Param site path string true "Website slug"
// @Param url query string true "Canonical URL"
// @Param base_version_id query string false "Base version ID for comparison"
// @Param head_version_id query string false "Head version ID for comparison"
// @Success 200 {object} app.EndpointDetails
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /projects/{project}/websites/{site}/endpoints/details [get]
func (s *Server) handleGetEndpointDetails(w http.ResponseWriter, r *http.Request) {
	project := chi.URLParam(r, "project")
	site := chi.URLParam(r, "site")
	url := r.URL.Query().Get("url")
	if url == "" {
		s.logger.Warn("getting endpoint details: missing url query parameter")
		writeError(w, http.StatusBadRequest, "missing url query parameter")
		return
	}

	baseVersionID := r.URL.Query().Get("base_version_id")
	headVersionID := r.URL.Query().Get("head_version_id")

	details, err := s.orchestrator.GetEndpointDetails(r.Context(), project, site, url, baseVersionID, headVersionID)
	if err != nil {
		s.logger.Warn("getting endpoint details", logging.Field{Key: "error", Value: err.Error()})
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.logger.Info("got endpoint details", logging.Field{Key: "project", Value: project}, logging.Field{Key: "site", Value: site}, logging.Field{Key: "url", Value: url})
	writeJSON(w, http.StatusOK, details)
}

// handleListVersions godoc
// @Summary List versions for a website
// @Description Returns a list of versions (commits) for the website, ordered by most recent first.
// @Tags Versions
// @Produce json
// @Param project path string true "Project slug"
// @Param site path string true "Website slug"
// @Param limit query int false "Maximum number of versions to return" default(100)
// @Success 200 {array} models.Version
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /projects/{project}/websites/{site}/versions [get]
func (s *Server) handleListVersions(w http.ResponseWriter, r *http.Request) {
	project := chi.URLParam(r, "project")
	site := chi.URLParam(r, "site")

	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	versions, err := s.orchestrator.ListVersions(r.Context(), project, site, limit)
	if err != nil {
		s.logger.Warn("listing versions", logging.Field{Key: "error", Value: err.Error()})
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.logger.Info("listed versions", logging.Field{Key: "project", Value: project}, logging.Field{Key: "site", Value: site}, logging.Field{Key: "count", Value: len(versions)})
	writeJSON(w, http.StatusOK, versions)
}

// Jobs (REST)

// handleStartFetchJob godoc
// @Summary Start a fetch job
// @Description Triggers a fetch of endpoints, optionally filtered by status/limit.
// @Tags Jobs
// @Accept json
// @Produce json
// @Param project path string true "Project slug"
// @Param site path string true "Website slug"
// @Param request body StartFetchJobRequest false "Fetch options"
// @Success 202 {object} app.Job
// @Failure 500 {object} ErrorResponse
// @Router /projects/{project}/websites/{site}/jobs/fetch [post]
func (s *Server) handleStartFetchJob(w http.ResponseWriter, r *http.Request) {
	project := chi.URLParam(r, "project")
	site := chi.URLParam(r, "site")

	var body StartFetchJobRequest
	_ = json.NewDecoder(r.Body).Decode(&body)

	//  Use defaults if not provided
	body.Status = normalizeEndpointStatus(body.Status)
	if body.Status == "" {
		body.Status = "new"
	}

	job, err := s.orchestrator.StartFetchJob(context.Background(), project, site, body.Status, body.Limit, body.Config)
	if err != nil {
		s.logger.Warn("starting fetch job", logging.Field{Key: "error", Value: err.Error()})
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.logger.Info("started fetch job", logging.Field{Key: "job_id", Value: job.ID}, logging.Field{Key: "status", Value: body.Status}, logging.Field{Key: "limit", Value: body.Limit})
	writeJSON(w, http.StatusAccepted, job)
}

// handleStartEnumerateJob godoc
// @Summary Start an enumeration job
// @Description Launches URL discovery using configured enumerators. Each enumerator (spider, sitemap, robots, wayback) is enabled by including its config object.
// @Tags Jobs
// @Accept json
// @Produce json
// @Param project path string true "Project slug"
// @Param site path string true "Website slug"
// @Param body body StartEnumerateJobRequest false "Enumeration configuration"
// @Success 202 {object} app.Job
// @Failure 500 {object} ErrorResponse
// @Router /projects/{project}/websites/{site}/jobs/enumerate [post]
func (s *Server) handleStartEnumerateJob(w http.ResponseWriter, r *http.Request) {
	project := chi.URLParam(r, "project")
	site := chi.URLParam(r, "site")

	var body StartEnumerateJobRequest
	_ = json.NewDecoder(r.Body).Decode(&body)

	job, err := s.orchestrator.StartEnumerateJob(context.Background(), project, site, body.Config)
	if err != nil {
		s.logger.Warn("starting enumerate job", logging.Field{Key: "error", Value: err.Error()})
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.logger.Info("started enumerate job", logging.Field{Key: "job_id", Value: job.ID})
	writeJSON(w, http.StatusAccepted, job)
}

// handleStartScanJob godoc
// @Summary Start a vulnerability scan job
// @Description Submits a URL for scanning via the configured analyzer backend (Moku native, Burp Suite, OWASP ZAP, etc.). Returns immediately with a job; poll /jobs/{jobID} or stream events to see results.
// @Tags Jobs
// @Accept json
// @Produce json
// @Param project path string true "Project slug"
// @Param site path string true "Website slug"
// @Param request body StartScanJobRequest true "Scan options"
// @Success 202 {object} app.Job
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /projects/{project}/websites/{site}/jobs/scan [post]
func (s *Server) handleStartScanJob(w http.ResponseWriter, r *http.Request) {
	project := chi.URLParam(r, "project")
	site := chi.URLParam(r, "site")

	var body StartScanJobRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.logger.Warn("decoding scan request", logging.Field{Key: "error", Value: err.Error()})
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.URL == "" {
		writeError(w, http.StatusBadRequest, "url is required")
		return
	}

	req := &analyzer.ScanRequest{
		URL:         body.URL,
		Profile:     body.Profile,
		Scope:       body.Scope,
		Auth:        body.Auth,
		MaxDuration: body.MaxDuration,
		RawOptions:  body.RawOptions,
	}

	job, err := s.orchestrator.StartScanJob(context.Background(), project, site, req)
	if err != nil {
		s.logger.Warn("starting scan job", logging.Field{Key: "error", Value: err.Error()})
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.logger.Info("started scan job", logging.Field{Key: "job_id", Value: job.ID}, logging.Field{Key: "url", Value: body.URL})
	writeJSON(w, http.StatusAccepted, job)
}

// handleGetAnalyzerCapabilities godoc
// @Summary Report analyzer capabilities for a site
// @Description Returns which analyzer backend a site is wired to (moku/burp/zap/…) and which optional ScanRequest fields it honors.
// @Tags Analyzer
// @Produce json
// @Param project path string true "Project slug"
// @Param site path string true "Website slug"
// @Success 200 {object} AnalyzerCapabilitiesResponse
// @Failure 404 {object} ErrorResponse
// @Router /projects/{project}/websites/{site}/analyzer/capabilities [get]
func (s *Server) handleGetAnalyzerCapabilities(w http.ResponseWriter, r *http.Request) {
	project := chi.URLParam(r, "project")
	site := chi.URLParam(r, "site")

	a, err := s.orchestrator.GetAnalyzer(r.Context(), project, site)
	if err != nil {
		s.logger.Warn("getting analyzer", logging.Field{Key: "error", Value: err.Error()})
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, AnalyzerCapabilitiesResponse{
		Backend:      a.Name(),
		Capabilities: a.Capabilities(),
	})
}

// handleGetAnalyzerHealth godoc
// @Summary Check analyzer health for a site
// @Description Probes the analyzer backend's readiness. Returns 200 with a status string when reachable, 503 when unavailable.
// @Tags Analyzer
// @Produce json
// @Param project path string true "Project slug"
// @Param site path string true "Website slug"
// @Success 200 {object} AnalyzerHealthResponse
// @Failure 404 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /projects/{project}/websites/{site}/analyzer/health [get]
func (s *Server) handleGetAnalyzerHealth(w http.ResponseWriter, r *http.Request) {
	project := chi.URLParam(r, "project")
	site := chi.URLParam(r, "site")

	a, err := s.orchestrator.GetAnalyzer(r.Context(), project, site)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	status, err := a.Health(r.Context())
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, AnalyzerHealthResponse{Backend: a.Name(), Status: status})
		return
	}
	writeJSON(w, http.StatusOK, AnalyzerHealthResponse{Backend: a.Name(), Status: status})
}

// handleGetJob godoc
// @Summary Get job details
// @Description Returns the current state of a background job.
// @Tags Jobs
// @Produce json
// @Param jobID path string true "Job identifier"
// @Success 200 {object} app.Job
// @Failure 404 {object} ErrorResponse
// @Router /jobs/{jobID} [get]
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

// handleCancelJob godoc
// @Summary Cancel a job
// @Description Signals the orchestrator to stop a running job.
// @Tags Jobs
// @Param jobID path string true "Job identifier"
// @Success 204 {string} string ""
// @Router /jobs/{jobID} [delete]
func (s *Server) handleCancelJob(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")
	s.orchestrator.CancelJob(jobID)
	s.logger.Info("canceled job", logging.Field{Key: "job_id", Value: jobID})
	writeJSON(w, http.StatusNoContent, nil)
}

// handleListJobs godoc
// @Summary List active jobs
// @Description Returns in-memory jobs tracked by the orchestrator.
// @Tags Jobs
// @Produce json
// @Success 200 {array} app.Job
// @Router /jobs [get]
func (s *Server) handleListJobs(w http.ResponseWriter, r *http.Request) {
	jobs := s.orchestrator.ListJobs()
	s.logger.Info("listed jobs", logging.Field{Key: "count", Value: len(jobs)})
	writeJSON(w, http.StatusOK, jobs)
}

// WebSockets

// handleFetchWS godoc
// @Summary Stream fetch job events over WebSocket
// @Description Upgrades the connection and streams fetch progress events.
// @Tags Jobs
// @Param project path string true "Project slug"
// @Param site path string true "Website slug"
// @Param status query string false "Endpoint status filter" default(new)
// @Param limit query int false "Maximum endpoints"
// @Success 101 {string} string "WebSocket Upgrade"
// @Failure 400 {object} ErrorResponse
// @Router /ws/projects/{project}/websites/{site}/fetch [get]
func (s *Server) handleFetchWS(w http.ResponseWriter, r *http.Request) {
	project := chi.URLParam(r, "project")
	site := chi.URLParam(r, "site")

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Warn("upgrading to websocket", logging.Field{Key: "error", Value: err.Error()})
		return
	}
	defer conn.Close()

	// Try to read config from first WebSocket message
	var fetchRequest StartFetchJobRequest
	if err := conn.ReadJSON(&fetchRequest); err != nil {
		// Fallback to query parameters for backward compatibility
		status := normalizeEndpointStatus(r.URL.Query().Get("status"))
		if status == "" {
			status = "new"
		}
		limit := 100
		if ls := r.URL.Query().Get("limit"); ls != "" {
			if v, err := strconv.Atoi(ls); err == nil && v > 0 {
				limit = v
			}
		}

		fetchRequest = StartFetchJobRequest{
			Status: status,
			Limit:  limit,
		}
	}

	// Normalize and apply defaults
	fetchRequest.Status = normalizeEndpointStatus(fetchRequest.Status)
	if fetchRequest.Status == "" {
		fetchRequest.Status = "new"
	}
	if fetchRequest.Limit <= 0 {
		fetchRequest.Limit = 100
	}

	ctx := r.Context()

	job, err := s.orchestrator.StartFetchJob(ctx, project, site, fetchRequest.Status, fetchRequest.Limit, fetchRequest.Config)
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

func normalizeEndpointStatus(status string) string {
	trimmed := strings.TrimSpace(status)
	if strings.EqualFold(trimmed, "all") {
		return "*"
	}
	return trimmed
}

// handleEnumerateWS godoc
// @Summary Stream enumerate job events over WebSocket
// @Description Upgrades the connection, receives EnumerationConfig as first message, and streams enumeration progress events.
// @Tags Jobs
// @Param project path string true "Project slug"
// @Param site path string true "Website slug"
// @Success 101 {string} string "WebSocket Upgrade"
// @Failure 400 {object} ErrorResponse
// @Router /ws/projects/{project}/websites/{site}/enumerate [get]
func (s *Server) handleEnumerateWS(w http.ResponseWriter, r *http.Request) {
	project := chi.URLParam(r, "project")
	site := chi.URLParam(r, "site")

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Warn("upgrading to websocket", logging.Field{Key: "error", Value: err.Error()})
		return
	}
	defer conn.Close()

	// Read config from first message
	var cfg api.EnumerationConfig
	if err := conn.ReadJSON(&cfg); err != nil {
		_ = conn.WriteJSON(map[string]string{"error": "invalid config: " + err.Error()})
		s.logger.Warn("reading enumerate config", logging.Field{Key: "error", Value: err.Error()})
		return
	}

	ctx := r.Context()

	job, err := s.orchestrator.StartEnumerateJob(ctx, project, site, cfg)
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
