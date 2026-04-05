package server

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/raysh454/moku/internal/filter"
	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/registry"
)

// handleListFilterRules lists all filter rules for a website.
func (s *Server) handleListFilterRules(w http.ResponseWriter, r *http.Request) {
	project := chi.URLParam(r, "project")
	site := chi.URLParam(r, "site")

	ws, err := s.orchestrator.Registry().GetWebsiteBySlug(r.Context(), project, site)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	rules, err := s.orchestrator.Registry().ListFilterRules(r.Context(), ws.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	resp := FilterRulesListResponse{Rules: make([]FilterRuleResponse, len(rules))}
	for i, rule := range rules {
		resp.Rules[i] = toFilterRuleResponse(rule)
	}

	writeJSON(w, http.StatusOK, resp)
}

// handleCreateFilterRule creates a new filter rule.
func (s *Server) handleCreateFilterRule(w http.ResponseWriter, r *http.Request) {
	project := chi.URLParam(r, "project")
	site := chi.URLParam(r, "site")

	ws, err := s.orchestrator.Registry().GetWebsiteBySlug(r.Context(), project, site)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	var req CreateFilterRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	ruleType := filter.RuleType(req.RuleType)
	if !ruleType.IsValid() {
		writeError(w, http.StatusBadRequest, "invalid rule_type: must be extension, pattern, or status_code")
		return
	}

	var rule *filter.FilterRule
	if req.Priority != nil {
		rule, err = s.orchestrator.Registry().AddFilterRuleWithPriority(r.Context(), ws.ID, ruleType, req.RuleValue, *req.Priority)
	} else {
		rule, err = s.orchestrator.Registry().AddFilterRule(r.Context(), ws.ID, ruleType, req.RuleValue)
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, toFilterRuleResponse(*rule))
}

// handleGetFilterRule gets a single filter rule.
func (s *Server) handleGetFilterRule(w http.ResponseWriter, r *http.Request) {
	ruleID := chi.URLParam(r, "ruleID")

	rule, err := s.orchestrator.Registry().GetFilterRule(r.Context(), ruleID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, toFilterRuleResponse(*rule))
}

// handleUpdateFilterRule updates a filter rule.
func (s *Server) handleUpdateFilterRule(w http.ResponseWriter, r *http.Request) {
	ruleID := chi.URLParam(r, "ruleID")

	var req UpdateFilterRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	// Fetch current rule to support partial updates
	currentRule, err := s.orchestrator.Registry().GetFilterRule(r.Context(), ruleID)
	if err != nil {
		if err == registry.ErrFilterRuleNotFound {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	// Use provided values or fall back to current values
	ruleType := currentRule.RuleType
	if req.RuleType != "" {
		ruleType = filter.RuleType(req.RuleType)
		if !ruleType.IsValid() {
			writeError(w, http.StatusBadRequest, "invalid rule_type: must be extension, pattern, or status_code")
			return
		}
	}

	ruleValue := currentRule.RuleValue
	if req.RuleValue != "" {
		ruleValue = req.RuleValue
	}

	enabled := currentRule.Enabled
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	if err := s.orchestrator.Registry().UpdateFilterRule(r.Context(), ruleID, ruleType, ruleValue, enabled); err != nil {
		if err == registry.ErrFilterRuleNotFound {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	rule, err := s.orchestrator.Registry().GetFilterRule(r.Context(), ruleID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, toFilterRuleResponse(*rule))
}

// handleToggleFilterRule toggles the enabled state of a filter rule.
func (s *Server) handleToggleFilterRule(w http.ResponseWriter, r *http.Request) {
	ruleID := chi.URLParam(r, "ruleID")

	// Fetch current rule
	rule, err := s.orchestrator.Registry().GetFilterRule(r.Context(), ruleID)
	if err != nil {
		if err == registry.ErrFilterRuleNotFound {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	// Toggle enabled state
	if err := s.orchestrator.Registry().UpdateFilterRule(r.Context(), ruleID, rule.RuleType, rule.RuleValue, !rule.Enabled); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Fetch updated rule
	rule, err = s.orchestrator.Registry().GetFilterRule(r.Context(), ruleID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, toFilterRuleResponse(*rule))
}

// handleDeleteFilterRule deletes a filter rule.
func (s *Server) handleDeleteFilterRule(w http.ResponseWriter, r *http.Request) {
	ruleID := chi.URLParam(r, "ruleID")

	if err := s.orchestrator.Registry().DeleteFilterRule(r.Context(), ruleID); err != nil {
		if err == registry.ErrFilterRuleNotFound {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleGetFilterConfig gets the website's filter configuration.
func (s *Server) handleGetFilterConfig(w http.ResponseWriter, r *http.Request) {
	project := chi.URLParam(r, "project")
	site := chi.URLParam(r, "site")

	ws, err := s.orchestrator.Registry().GetWebsiteBySlug(r.Context(), project, site)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	cfg, err := s.orchestrator.Registry().GetWebsiteFilterConfig(r.Context(), ws.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, FilterConfigResponse{
		SkipExtensions:  cfg.SkipExtensions,
		SkipPatterns:    cfg.SkipPatterns,
		SkipStatusCodes: cfg.SkipStatusCodes,
	})
}

// handleUpdateFilterConfig updates the website's filter configuration.
func (s *Server) handleUpdateFilterConfig(w http.ResponseWriter, r *http.Request) {
	project := chi.URLParam(r, "project")
	site := chi.URLParam(r, "site")

	ws, err := s.orchestrator.Registry().GetWebsiteBySlug(r.Context(), project, site)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	var req UpdateFilterConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	cfg := &filter.WebsiteFilterConfig{
		SkipExtensions:  req.SkipExtensions,
		SkipPatterns:    req.SkipPatterns,
		SkipStatusCodes: req.SkipStatusCodes,
	}

	if err := s.orchestrator.Registry().UpdateWebsiteFilterConfig(r.Context(), ws.ID, cfg); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, FilterConfigResponse{
		SkipExtensions:  cfg.SkipExtensions,
		SkipPatterns:    cfg.SkipPatterns,
		SkipStatusCodes: cfg.SkipStatusCodes,
	})
}

// handleUnfilterEndpoints resets filtered endpoints back to "new" status.
func (s *Server) handleUnfilterEndpoints(w http.ResponseWriter, r *http.Request) {
	project := chi.URLParam(r, "project")
	site := chi.URLParam(r, "site")

	_, err := s.orchestrator.Registry().GetWebsiteBySlug(r.Context(), project, site)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	var req UnfilterEndpointsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	idx, err := s.orchestrator.GetWebsiteIndexer(r.Context(), project, site)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var urls []string
	if req.All {
		filtered, err := idx.ListEndpoints(r.Context(), "filtered", 0)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		for _, ep := range filtered {
			urls = append(urls, ep.CanonicalURL)
		}
	} else {
		urls = req.CanonicalURLs
	}

	if len(urls) == 0 {
		writeJSON(w, http.StatusOK, UnfilterEndpointsResponse{Unfiltered: 0})
		return
	}

	if err := idx.UnfilterBatch(r.Context(), urls); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, UnfilterEndpointsResponse{Unfiltered: len(urls)})
}

// handleEndpointStats returns endpoint statistics.
func (s *Server) handleEndpointStats(w http.ResponseWriter, r *http.Request) {
	project := chi.URLParam(r, "project")
	site := chi.URLParam(r, "site")

	_, err := s.orchestrator.Registry().GetWebsiteBySlug(r.Context(), project, site)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	idx, err := s.orchestrator.GetWebsiteIndexer(r.Context(), project, site)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	byStatus, err := idx.GetEndpointStats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	total := 0
	for _, count := range byStatus {
		total += count
	}

	filteredByReason, err := idx.GetFilteredEndpointsByReason(r.Context())
	if err != nil {
		s.logger.Warn("failed to get filtered endpoint reasons", logging.Field{Key: "error", Value: err.Error()})
		filteredByReason = nil
	}

	writeJSON(w, http.StatusOK, EndpointStatsResponse{
		Total:            total,
		ByStatus:         byStatus,
		FilteredByReason: filteredByReason,
	})
}

// handleApplyFilters applies current filter rules to all non-filtered endpoints.
// This marks matching endpoints as "filtered" based on the website's filter rules.
func (s *Server) handleApplyFilters(w http.ResponseWriter, r *http.Request) {
	project := chi.URLParam(r, "project")
	site := chi.URLParam(r, "site")

	ws, err := s.orchestrator.Registry().GetWebsiteBySlug(r.Context(), project, site)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	// Load filter config from rules
	filterCfg, err := s.orchestrator.Registry().LoadFilterConfig(r.Context(), ws.ID, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load filter config: "+err.Error())
		return
	}

	if filterCfg.IsEmpty() {
		writeJSON(w, http.StatusOK, ApplyFiltersResponse{Filtered: 0, Message: "No filter rules configured"})
		return
	}

	// Get indexer
	idx, err := s.orchestrator.GetWebsiteIndexer(r.Context(), project, site)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// STEP 1: Reset all currently filtered endpoints back to "pending"
	// This ensures that disabled/removed rules take effect
	filteredEndpoints, err := idx.ListEndpoints(r.Context(), "filtered", 0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get filtered endpoints: "+err.Error())
		return
	}

	var toUnfilter []string
	for _, ep := range filteredEndpoints {
		toUnfilter = append(toUnfilter, ep.CanonicalURL)
	}

	if len(toUnfilter) > 0 {
		if err := idx.UnfilterBatch(r.Context(), toUnfilter); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to reset filtered endpoints: "+err.Error())
			return
		}
		s.logger.Info("reset filtered endpoints",
			logging.Field{Key: "project", Value: project},
			logging.Field{Key: "site", Value: site},
			logging.Field{Key: "reset_count", Value: len(toUnfilter)})
	}

	// STEP 2: Apply current filter rules to all endpoints
	// Get all endpoints (none are filtered now after reset)
	endpoints, err := idx.ListEndpoints(r.Context(), "all", 0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list endpoints: "+err.Error())
		return
	}

	// Build filter engine
	engine := filter.NewEngine(filterCfg)

	// Find matching endpoints
	var toFilter []string
	reasons := make(map[string]string)

	for _, ep := range endpoints {
		result := engine.ShouldFilter(ep.CanonicalURL)
		if result.Filtered {
			toFilter = append(toFilter, ep.CanonicalURL)
			reasons[ep.CanonicalURL] = result.Reason
		}
	}

	if len(toFilter) == 0 {
		writeJSON(w, http.StatusOK, ApplyFiltersResponse{Filtered: 0, Message: "No endpoints matched filter rules"})
		return
	}

	// Mark them as filtered
	if err := idx.MarkFilteredBatch(r.Context(), toFilter, reasons); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to mark endpoints as filtered: "+err.Error())
		return
	}

	s.logger.Info("applied filter rules",
		logging.Field{Key: "project", Value: project},
		logging.Field{Key: "site", Value: site},
		logging.Field{Key: "filtered_count", Value: len(toFilter)})

	writeJSON(w, http.StatusOK, ApplyFiltersResponse{
		Filtered: len(toFilter),
		Message:  "Successfully applied filters",
	})
}
