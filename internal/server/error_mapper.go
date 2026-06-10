package server

import (
	"errors"
	"net/http"

	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/registry"
)

// genericServerErrorMessage is the body returned for unmapped errors so raw
// internal failures (SQL text, file paths, etc.) never reach the client. The
// underlying error is logged server-side instead.
const genericServerErrorMessage = "internal server error"

// mapDomainError translates a domain error into an HTTP status and a
// client-safe message. Known sentinels keep their descriptive message; anything
// else collapses to a generic 500 so internal details are never exposed.
func mapDomainError(err error) (int, string) {
	switch {
	case errors.Is(err, registry.ErrProjectNotFound):
		return http.StatusNotFound, registry.ErrProjectNotFound.Error()
	case errors.Is(err, registry.ErrWebsiteNotFound):
		return http.StatusNotFound, registry.ErrWebsiteNotFound.Error()
	case errors.Is(err, registry.ErrDuplicateSlug):
		return http.StatusConflict, registry.ErrDuplicateSlug.Error()
	default:
		return http.StatusInternalServerError, genericServerErrorMessage
	}
}

// writeDomainError maps err to a safe status/message, logs the raw error at Warn
// with the operation name for server-side diagnosis, and writes the mapped
// response. op identifies the handler operation (e.g. "create project").
func (s *Server) writeDomainError(w http.ResponseWriter, err error, op string) {
	status, msg := mapDomainError(err)
	s.logger.Warn(op,
		logging.Field{Key: "error", Value: err.Error()},
		logging.Field{Key: "status", Value: status},
	)
	writeError(w, status, msg)
}
