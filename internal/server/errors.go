package server

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
)

var statusByKind = map[error]int{
	domain.ErrNotFound:    http.StatusNotFound,
	domain.ErrInvalid:     http.StatusBadRequest,
	domain.ErrForbidden:   http.StatusForbidden,
	domain.ErrConflict:    http.StatusConflict,
	domain.ErrUnsupported: http.StatusUnprocessableEntity,
	domain.ErrUnavailable: http.StatusServiceUnavailable,
	domain.ErrUpstream:    http.StatusBadGateway,
	domain.ErrTimeout:     http.StatusGatewayTimeout,
	domain.ErrInternal:    http.StatusInternalServerError,
}

// writeError turns a use case failure into an HTTP response without leaking
// internal details to the client.
func (s *Server) writeError(w http.ResponseWriter, err error) {
	status, message := errorResponse(err)
	if status >= http.StatusInternalServerError {
		s.logger.Error("request failed", "status", status, "error", err)
	}
	http.Error(w, message, status)
}

func errorResponse(err error) (int, string) {
	var userErr *domain.Error
	if errors.As(err, &userErr) {
		if status, ok := statusByKind[userErr.Kind()]; ok {
			return status, userErr.Error()
		}
	}
	if errors.Is(err, domain.ErrNotFound) {
		return http.StatusNotFound, "Table or track not found"
	}
	return http.StatusInternalServerError, "Internal server error"
}

// viewer describes the authenticated caller to the domain.
func viewer(token *auth.Token) domain.Viewer {
	if token.Role == auth.RoleGM {
		return domain.GMViewer()
	}
	tableID, err := uuid.Parse(token.TableID)
	if err != nil {
		return domain.PlayerViewer(uuid.Nil)
	}
	return domain.PlayerViewer(tableID)
}
