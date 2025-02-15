package server

import (
	"encoding/json"
	"net/http"
)

func (s *Server) listFiles(w http.ResponseWriter, r *http.Request) {
	tracks, err := s.store.GetTracks(r.Context())
	if err != nil {
		s.logger.Error("failed to retrieve tracks", "error", err)
		http.Error(w, "Failed to list files", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(tracks); err != nil {
		s.logger.Error("failed to encode response", "error", err)
	}
}
