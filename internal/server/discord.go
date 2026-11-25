package server

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
)

func (s *Server) handleDiscord(w http.ResponseWriter, r *http.Request, token *auth.Token) {
	w.Header().Set("Cache-Control", "no-store")
	externalID := r.PathValue("tableID")
	var (
		status domain.DiscordStatus
		err    error
	)
	switch r.Method {
	case http.MethodGet:
		status, err = s.voice.Status(r.Context(), viewer(token), externalID)
	case http.MethodPost:
		selection, ok := readDiscordSelection(w, r)
		if !ok {
			return
		}
		status, err = s.voice.Connect(r.Context(), viewer(token), externalID, selection)
	case http.MethodDelete:
		status, err = s.voice.Disconnect(r.Context(), viewer(token), externalID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err != nil {
		s.writeError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, status)
}

// readDiscordSelection returns nil when the request has no body, which asks
// the domain to reconnect to the table's saved destination.
func readDiscordSelection(w http.ResponseWriter, r *http.Request) (*domain.Destination, bool) {
	if r.ContentLength == 0 {
		return nil, true
	}
	var selection domain.Destination
	decoder := json.NewDecoder(io.LimitReader(r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&selection); err != nil {
		http.Error(w, "Select a Discord server and voice channel", http.StatusBadRequest)
		return nil, false
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		http.Error(w, "Invalid Discord selection", http.StatusBadRequest)
		return nil, false
	}
	return &selection, true
}

func (s *Server) handleDiscordSetup(w http.ResponseWriter, r *http.Request, token *auth.Token) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	if guildID := r.PathValue("guildID"); guildID != "" {
		channels, err := s.voice.Channels(r.Context(), viewer(token), guildID)
		if err != nil {
			s.writeError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, channels)
		return
	}

	setup, err := s.voice.Setup(r.Context(), viewer(token))
	if err != nil {
		s.writeError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, setup)
}
