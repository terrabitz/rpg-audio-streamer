package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
	"github.com/terrabitz/rpg-audio-streamer/internal/library"
)

func (s *Server) handleFiles(w http.ResponseWriter, r *http.Request, token *auth.Token) {
	switch r.Method {
	case http.MethodGet:
		s.listFiles(w, r, token)
	case http.MethodPost:
		s.uploadFile(w, r, token)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) listFiles(w http.ResponseWriter, r *http.Request, token *auth.Token) {
	tracks, err := s.library.ListTracks(r.Context(), viewer(token))
	if err != nil {
		s.writeError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, tracks)
}

func (s *Server) uploadFile(w http.ResponseWriter, r *http.Request, token *auth.Token) {
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}
	defer r.MultipartForm.RemoveAll()

	tableID, err := optionalUUID(r.FormValue("tableID"))
	if err != nil {
		http.Error(w, "Invalid table ID", http.StatusBadRequest)
		return
	}
	file, _, err := r.FormFile("files")
	if err != nil {
		http.Error(w, "Failed to retrieve file", http.StatusBadRequest)
		return
	}
	defer file.Close()
	typeID, err := uuid.Parse(r.FormValue("typeID"))
	if err != nil {
		http.Error(w, "Invalid track type ID", http.StatusBadRequest)
		return
	}

	track, err := s.library.Upload(r.Context(), viewer(token), library.Upload{
		Name:    r.FormValue("name"),
		TypeID:  typeID,
		TableID: tableID,
		Audio:   file,
	})
	if err != nil {
		s.writeError(w, err)
		return
	}
	respondJSON(w, http.StatusAccepted, track)
}

func (s *Server) handleImport(w http.ResponseWriter, r *http.Request, token *auth.Token) {
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
	if err := decodeStrict(w, r, 16<<10, &req); err != nil {
		http.Error(w, "Invalid import request", http.StatusBadRequest)
		return
	}

	track, err := s.library.Import(r.Context(), viewer(token), library.Import{
		URL:     req.URL,
		Name:    req.Name,
		TypeID:  req.TypeID,
		TableID: req.TableID,
	})
	if err != nil {
		s.writeError(w, err)
		return
	}
	respondJSON(w, http.StatusAccepted, track)
}

func (s *Server) handleScan(w http.ResponseWriter, r *http.Request, token *auth.Token) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		URL string `json:"url"`
	}
	if err := decodeStrict(w, r, 16<<10, &req); err != nil {
		http.Error(w, "Invalid scan request", http.StatusBadRequest)
		return
	}

	metadata, err := s.library.Scan(r.Context(), viewer(token), req.URL)
	if err != nil {
		s.writeError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, metadata)
}

func (s *Server) handleFile(w http.ResponseWriter, r *http.Request, token *auth.Token) {
	trackID, err := uuid.Parse(r.PathValue("trackID"))
	if err != nil {
		http.Error(w, "Invalid track ID", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodPut:
		s.updateFile(w, r, token, trackID)
	case http.MethodDelete:
		s.deleteFile(w, r, token, trackID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) updateFile(w http.ResponseWriter, r *http.Request, token *auth.Token, trackID uuid.UUID) {
	var update domain.TrackUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	track, err := s.library.UpdateTrack(r.Context(), viewer(token), trackID, update)
	if err != nil {
		s.writeError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, track)
}

func (s *Server) deleteFile(w http.ResponseWriter, r *http.Request, token *auth.Token, trackID uuid.UUID) {
	expectedTableCount, err := parseExpectedTableCount(r)
	if err != nil {
		http.Error(w, "Invalid expected table count", http.StatusBadRequest)
		return
	}

	if err := s.library.DeleteTrack(r.Context(), viewer(token), trackID, expectedTableCount); err != nil {
		s.writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Track deleted successfully"))
}

func (s *Server) handleFileUsage(w http.ResponseWriter, r *http.Request, token *auth.Token) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	trackID, err := uuid.Parse(r.PathValue("trackID"))
	if err != nil {
		http.Error(w, "Invalid track ID", http.StatusBadRequest)
		return
	}

	count, err := s.library.TrackUsage(r.Context(), viewer(token), trackID)
	if err != nil {
		s.writeError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, struct {
		TableCount int64 `json:"tableCount"`
	}{
		TableCount: count,
	})
}

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

	waveform, err := s.library.Waveform(r.Context(), viewer(token), trackID)
	if err != nil {
		s.writeError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=3600")
	respondJSON(w, http.StatusOK, waveform)
}

func (s *Server) streamDirectory(w http.ResponseWriter, r *http.Request, token *auth.Token) {
	filePath, err := s.library.StreamFile(r.Context(), viewer(token), r.PathValue("tableID"), r.PathValue("path"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	http.ServeFile(w, r, filePath)
}

func (s *Server) handleTrackTypes(w http.ResponseWriter, r *http.Request, _ *auth.Token) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	trackTypes, err := s.library.TrackTypes(r.Context())
	if err != nil {
		s.writeError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, trackTypes)
}

func optionalUUID(value string) (uuid.UUID, error) {
	if value == "" {
		return uuid.Nil, nil
	}
	return uuid.Parse(value)
}

var errInvalidTableCount = errors.New("invalid expected table count")

// parseExpectedTableCount reads the optional usage guard that keeps a delete
// from removing a track another table started using after the GM confirmed.
func parseExpectedTableCount(r *http.Request) (*int64, error) {
	values, ok := r.URL.Query()["expectedTableCount"]
	if !ok {
		return nil, nil
	}
	if len(values) != 1 {
		return nil, errInvalidTableCount
	}
	count, err := strconv.ParseInt(values[0], 10, 64)
	if err != nil || count < 0 {
		return nil, errInvalidTableCount
	}
	return &count, nil
}

var errTrailingData = errors.New("unexpected data after JSON value")

// decodeStrict accepts exactly one JSON value with only known fields.
func decodeStrict(w http.ResponseWriter, r *http.Request, limit int64, out any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, limit))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return errTrailingData
	}
	return nil
}
