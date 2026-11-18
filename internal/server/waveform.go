package server

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
	"github.com/terrabitz/rpg-audio-streamer/internal/musicprocessing"
)

func (s *Server) handleFileWaveform(w http.ResponseWriter, r *http.Request, token *auth.Token) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	trackID, err := uuid.Parse(r.PathValue("trackID"))
	if err != nil {
		http.Error(w, "Invalid track ID", http.StatusBadRequest)
		return
	}
	track, err := s.store.GetTrackByID(r.Context(), trackID)
	if err != nil {
		http.Error(w, "Track not found", http.StatusNotFound)
		return
	}
	if !track.Ready() {
		http.Error(w, "Track is not ready", http.StatusConflict)
		return
	}
	waveform, err := musicprocessing.LoadWaveform(r.Context(), track.Path)
	if err != nil {
		s.logger.Error("failed to load waveform", "error", err, "trackID", trackID)
		http.Error(w, "Failed to generate waveform", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=3600")
	respondJSON(w, http.StatusOK, waveform)
}
