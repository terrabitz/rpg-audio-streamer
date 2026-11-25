package server

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
)

func (s *Server) handleTables(w http.ResponseWriter, r *http.Request, token *auth.Token) {
	switch r.Method {
	case http.MethodGet:
		tables, err := s.tables.List(r.Context(), viewer(token))
		if err != nil {
			s.writeError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, tables)
	case http.MethodPost:
		name, ok := readTableName(w, r)
		if !ok {
			return
		}
		table, err := s.tables.Create(r.Context(), viewer(token), name)
		if err != nil {
			s.writeError(w, err)
			return
		}
		respondJSON(w, http.StatusCreated, table)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleTable(w http.ResponseWriter, r *http.Request, token *auth.Token) {
	externalID := r.PathValue("tableID")
	switch r.Method {
	case http.MethodGet:
		table, err := s.tables.Get(r.Context(), viewer(token), externalID)
		if err != nil {
			s.writeError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, table)
	case http.MethodPut:
		name, ok := readTableName(w, r)
		if !ok {
			return
		}
		table, err := s.tables.Rename(r.Context(), viewer(token), externalID, name)
		if err != nil {
			s.writeError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, table)
	case http.MethodDelete:
		if err := s.tables.Delete(r.Context(), viewer(token), externalID); err != nil {
			s.writeError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleRotateTableExternalID(w http.ResponseWriter, r *http.Request, token *auth.Token) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	table, err := s.tables.RotateLink(r.Context(), viewer(token), r.PathValue("tableID"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, table)
}

func (s *Server) handleTableSettings(w http.ResponseWriter, r *http.Request, token *auth.Token) {
	externalID := r.PathValue("tableID")
	switch r.Method {
	case http.MethodGet:
		settings, err := s.tables.Settings(r.Context(), viewer(token), externalID)
		if err != nil {
			s.writeError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, settings)
	case http.MethodPatch:
		var patch *domain.TableSettingsPatch
		if err := decodeStrict(w, r, 1<<20, &patch); err != nil || patch == nil {
			http.Error(w, "Invalid table settings", http.StatusBadRequest)
			return
		}
		settings, err := s.tables.UpdateSettings(r.Context(), viewer(token), externalID, *patch)
		if err != nil {
			s.writeError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, settings)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleTableTracks(w http.ResponseWriter, r *http.Request, token *auth.Token) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tracks, err := s.tables.Tracks(r.Context(), viewer(token), r.PathValue("tableID"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, tracks)
}

func (s *Server) handleTableTrack(w http.ResponseWriter, r *http.Request, token *auth.Token) {
	trackID, err := uuid.Parse(r.PathValue("trackID"))
	if err != nil {
		http.Error(w, "Invalid track ID", http.StatusBadRequest)
		return
	}
	externalID := r.PathValue("tableID")
	switch r.Method {
	case http.MethodPut:
		err = s.tables.AddTrack(r.Context(), viewer(token), externalID, trackID)
	case http.MethodDelete:
		err = s.tables.RemoveTrack(r.Context(), viewer(token), externalID, trackID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err != nil {
		s.writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func readTableName(w http.ResponseWriter, r *http.Request) (string, bool) {
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return "", false
	}
	return body.Name, true
}
