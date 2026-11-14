package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
)

func (s *Server) handleTableSettings(w http.ResponseWriter, r *http.Request, token *auth.Token) {
	id, ok := s.tableID(w, r, token)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		settings, err := s.store.GetTableSettings(r.Context(), id)
		if err != nil {
			s.tableError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, settings)
	case http.MethodPatch:
		var patch *TableSettingsPatch
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024*1024))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&patch); err != nil || patch == nil {
			http.Error(w, "Invalid table settings", http.StatusBadRequest)
			return
		}
		if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) || !validTableSettings(*patch) {
			http.Error(w, "Invalid table settings", http.StatusBadRequest)
			return
		}
		settings, err := s.store.UpdateTableSettings(r.Context(), id, *patch)
		if err != nil {
			s.tableError(w, err)
			return
		}
		s.hub.SyncSettings(id.String(), *patch)
		respondJSON(w, http.StatusOK, settings)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func validTableSettings(patch TableSettingsPatch) bool {
	if patch.MasterVolume != nil && (*patch.MasterVolume < 0 || *patch.MasterVolume > 200) {
		return false
	}
	for name, volume := range patch.TypeVolumes {
		if strings.TrimSpace(name) == "" || volume < 0 || volume > 100 {
			return false
		}
	}
	for id, volume := range patch.TrackVolumes {
		if _, err := uuid.Parse(id); err != nil || volume < 0 || volume > 100 {
			return false
		}
	}
	if patch.TrackOrder != nil {
		seen := make(map[uuid.UUID]bool, len(*patch.TrackOrder))
		for _, id := range *patch.TrackOrder {
			if id == uuid.Nil || seen[id] {
				return false
			}
			seen[id] = true
		}
	}
	return true
}
