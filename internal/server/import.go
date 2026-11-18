package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
	"github.com/terrabitz/rpg-audio-streamer/internal/youtube"
)

func (s *Server) handleImport(w http.ResponseWriter, r *http.Request, _ *auth.Token) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		URL     string    `json:"url"`
		Name    string    `json:"name"`
		TypeID  uuid.UUID `json:"typeID"`
		TableID uuid.UUID `json:"tableID"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "Invalid import request", http.StatusBadRequest)
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		http.Error(w, "Invalid import request", http.StatusBadRequest)
		return
	}
	if _, ok := youtube.CanonicalURL(req.URL); !ok {
		http.Error(w, youtube.ErrInvalidURL.Error(), http.StatusBadRequest)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || utf8.RuneCountInString(req.Name) > 200 {
		http.Error(w, "Track name must be between 1 and 200 characters", http.StatusBadRequest)
		return
	}
	if _, err := s.store.GetTrackTypeByID(r.Context(), req.TypeID); err != nil {
		http.Error(w, "Invalid track type", http.StatusBadRequest)
		return
	}
	if req.TableID != uuid.Nil {
		if _, err := s.store.GetTable(r.Context(), req.TableID); err != nil {
			s.tableError(w, err)
			return
		}
	}
	track, err := s.processing.Import(r.Context(), req.Name, req.TypeID, req.TableID, req.URL)
	if err != nil {
		s.logger.Error("failed to queue imported audio", "error", err)
		if errors.Is(err, youtube.ErrUnavailable) {
			http.Error(w, youtube.ErrUnavailable.Error(), http.StatusServiceUnavailable)
			return
		}
		http.Error(w, "Failed to queue audio processing. Please try again.", http.StatusServiceUnavailable)
		return
	}
	respondJSON(w, http.StatusAccepted, track)
}

func (s *Server) handleScan(w http.ResponseWriter, r *http.Request, _ *auth.Token) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		URL string `json:"url"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "Invalid scan request", http.StatusBadRequest)
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		http.Error(w, "Invalid scan request", http.StatusBadRequest)
		return
	}
	metadata, err := s.videos.Scan(r.Context(), req.URL)
	if err != nil {
		switch {
		case errors.Is(err, youtube.ErrInvalidURL):
			http.Error(w, err.Error(), http.StatusBadRequest)
		case errors.Is(err, youtube.ErrUnavailable):
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
		case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
			http.Error(w, "Video scan timed out. Please try again.", http.StatusGatewayTimeout)
		case errors.Is(err, youtube.ErrInvalidMetadata):
			http.Error(w, err.Error(), http.StatusBadGateway)
		case errors.Is(err, youtube.ErrLiveStream), errors.Is(err, youtube.ErrDurationLimit),
			errors.Is(err, youtube.ErrSizeLimit):
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		default:
			http.Error(w, "Could not retrieve video metadata. Check that the video is public and available.", http.StatusBadGateway)
		}
		return
	}
	respondJSON(w, http.StatusOK, metadata)
}
