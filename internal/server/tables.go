package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
)

func (s *Server) handleTables(w http.ResponseWriter, r *http.Request, token *auth.Token) {
	switch r.Method {
	case http.MethodGet:
		tables, err := s.store.GetTables(r.Context())
		if err != nil {
			s.tableError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, tables)
	case http.MethodPost:
		name, ok := readTableName(w, r)
		if !ok {
			return
		}
		table, err := s.store.CreateTable(r.Context(), Table{
			ID: uuid.New(), Name: name,
		})
		if err != nil {
			s.tableError(w, err)
			return
		}
		respondJSON(w, http.StatusCreated, table)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleTable(w http.ResponseWriter, r *http.Request, token *auth.Token) {
	id, ok := s.tableID(w, r, token)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		table, err := s.store.GetTable(r.Context(), id)
		if err != nil {
			s.tableError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, table)
	case http.MethodPut:
		if token.Role != auth.RoleGM {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		name, ok := readTableName(w, r)
		if !ok {
			return
		}
		table, err := s.store.UpdateTable(r.Context(), id, name)
		if err != nil {
			s.tableError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, table)
	case http.MethodDelete:
		if token.Role != auth.RoleGM {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		if err := s.store.DeleteTable(r.Context(), id); err != nil {
			s.tableError(w, err)
			return
		}
		s.hub.CloseTable(id.String())
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleTableTracks(w http.ResponseWriter, r *http.Request, token *auth.Token) {
	id, ok := s.tableID(w, r, token)
	if !ok {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	tracks, err := s.store.GetTableTracks(r.Context(), id)
	if err != nil {
		s.tableError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, tracks)
}

func (s *Server) handleTableTrack(w http.ResponseWriter, r *http.Request, token *auth.Token) {
	id, ok := s.tableID(w, r, token)
	if !ok {
		return
	}
	trackID, err := uuid.Parse(r.PathValue("trackID"))
	if err != nil {
		http.Error(w, "Invalid track ID", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodPut:
		err = s.store.AddTableTrack(r.Context(), id, trackID)
	case http.MethodDelete:
		err = s.store.RemoveTableTrack(r.Context(), id, trackID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err != nil {
		s.tableError(w, err)
		return
	}
	if r.Method == http.MethodDelete {
		s.hub.RemoveTrack(id.String(), trackID.String())
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) tableID(w http.ResponseWriter, r *http.Request, token *auth.Token) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("tableID"))
	if err != nil {
		http.Error(w, "Invalid table ID", http.StatusBadRequest)
		return uuid.Nil, false
	}
	if token.Role == auth.RolePlayer && token.TableID != id.String() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return uuid.Nil, false
	}
	if _, err := s.store.GetTable(r.Context(), id); err != nil {
		s.tableError(w, err)
		return uuid.Nil, false
	}
	return id, true
}

func (s *Server) tableError(w http.ResponseWriter, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "Table or track not found", http.StatusNotFound)
		return
	}
	s.logger.Error("table operation failed", "error", err)
	http.Error(w, "Table operation failed", http.StatusInternalServerError)
}

func readTableName(w http.ResponseWriter, r *http.Request) (string, bool) {
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return "", false
	}
	name := strings.TrimSpace(body.Name)
	if name == "" || utf8.RuneCountInString(name) > 200 {
		http.Error(w, "Table name must contain 1 to 200 characters", http.StatusBadRequest)
		return "", false
	}
	return name, true
}
