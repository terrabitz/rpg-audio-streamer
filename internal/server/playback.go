package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
	"github.com/terrabitz/rpg-audio-streamer/internal/events"
	"github.com/terrabitz/rpg-audio-streamer/internal/playback"
)

type Playback interface {
	Snapshot(context.Context, uuid.UUID) (events.Snapshot, error)
	Update(context.Context, uuid.UUID, uuid.UUID, events.PlaybackPatch) error
}

func (s *Server) getPlayback(w http.ResponseWriter, r *http.Request, token *auth.Token) {
	w.Header().Set("Cache-Control", "no-store")
	id, ok := s.tableID(w, r, token)
	if !ok {
		return
	}
	snapshot, err := s.playback.Snapshot(r.Context(), id)
	if err != nil {
		s.tableError(w, err)
		return
	}
	if token.Role == auth.RolePlayer {
		snapshot = snapshot.ForPlayer()
	}
	respondJSON(w, http.StatusOK, snapshot)
}

func (s *Server) patchPlayback(w http.ResponseWriter, r *http.Request, token *auth.Token) {
	id, ok := s.tableID(w, r, token)
	if !ok {
		return
	}
	trackID, err := uuid.Parse(r.PathValue("trackID"))
	if err != nil {
		http.Error(w, "Invalid track ID", http.StatusBadRequest)
		return
	}
	var body struct {
		IsPlaying   *bool             `json:"isPlaying"`
		Volume      *float64          `json:"volume"`
		CurrentTime *float64          `json:"currentTime"`
		Duration    *float64          `json:"duration"`
		Playback    *PlaybackSettings `json:"playback"`
	}
	if !decodeCommand(w, r, &body) {
		return
	}
	err = s.playback.Update(r.Context(), id, trackID, events.PlaybackPatch{
		IsPlaying:   body.IsPlaying,
		Volume:      body.Volume,
		CurrentTime: body.CurrentTime,
		Duration:    body.Duration,
		Playback:    body.Playback,
	})
	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, playback.ErrInvalidPatch):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, playback.ErrTrackNotAssociated):
		http.Error(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, playback.ErrTrackNotReady):
		http.Error(w, err.Error(), http.StatusConflict)
	default:
		s.tableError(w, err)
	}
}

func (s *Server) putMix(w http.ResponseWriter, r *http.Request, token *auth.Token) {
	id, ok := s.tableID(w, r, token)
	if !ok {
		return
	}
	var body events.MixSnapshot
	if !decodeCommand(w, r, &body) {
		return
	}
	if body.Tracks == nil {
		http.Error(w, "Mix snapshot requires tracks", http.StatusBadRequest)
		return
	}
	s.dispatchMixCommand(w, events.MixChanged, events.TableEvent[events.MixSnapshot]{
		TableID: id.String(),
		Data:    body,
	})
}

func (s *Server) patchMix(w http.ResponseWriter, r *http.Request, token *auth.Token) {
	id, ok := s.tableID(w, r, token)
	if !ok {
		return
	}
	var body events.MixVolumePatch
	if !decodeCommand(w, r, &body) {
		return
	}
	if body.TypeVolumes == nil {
		http.Error(w, "Mix patch requires typeVolumes", http.StatusBadRequest)
		return
	}
	s.dispatchMixCommand(w, events.MixVolumesChanged, events.TableEvent[events.MixVolumePatch]{
		TableID: id.String(),
		Data:    body,
	})
}

func (s *Server) dispatchMixCommand(w http.ResponseWriter, event string, payload any) {
	if err := s.hub.SendEvent(event, payload); err != nil {
		s.logger.Error("failed to dispatch mixer command", "event", event, "error", err)
		http.Error(w, "Unable to dispatch mixer command", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func decodeCommand[T any](w http.ResponseWriter, r *http.Request, out *T) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024*1024))
	decoder.DisallowUnknownFields()
	var body *T
	if err := decoder.Decode(&body); err != nil || body == nil || !errors.Is(decoder.Decode(new(any)), io.EOF) {
		http.Error(w, "Invalid command body", http.StatusBadRequest)
		return false
	}
	*out = *body
	return true
}
