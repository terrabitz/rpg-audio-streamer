package server

import (
	"context"
	"database/sql"
	"errors"
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
	waveform, err := s.loadWaveform(r.Context(), track)
	if err != nil {
		s.logger.Error("failed to load waveform", "error", err, "trackID", trackID)
		http.Error(w, "Failed to load waveform", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=3600")
	respondJSON(w, http.StatusOK, waveform)
}

func (s *Server) loadWaveform(ctx context.Context, track Track) (musicprocessing.Waveform, error) {
	waveform, err := s.store.GetTrackWaveform(ctx, track.ID)
	if !errors.Is(err, sql.ErrNoRows) {
		return waveform, err
	}
	waveform, err = musicprocessing.GenerateWaveform(ctx, track.Path)
	if err != nil {
		return musicprocessing.Waveform{}, err
	}
	if err := s.store.SaveTrackWaveform(ctx, track.ID, waveform); err != nil {
		return musicprocessing.Waveform{}, err
	}
	return waveform, nil
}
