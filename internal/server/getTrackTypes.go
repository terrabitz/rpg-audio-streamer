package server

import (
	"encoding/json"
	"net/http"

	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
)

func (s *Server) handleTrackTypes(w http.ResponseWriter, r *http.Request, token *auth.Token) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	trackTypes, err := s.store.GetTrackTypes(r.Context())
	if err != nil {
		s.logger.Error("failed to get track types", "error", err)
		http.Error(w, "Failed to retrieve track types", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(trackTypes)
}
