package server

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
	"github.com/terrabitz/rpg-audio-streamer/internal/events"
)

func (s *Server) getPlayback(w http.ResponseWriter, r *http.Request, token *auth.Token) {
	w.Header().Set("Cache-Control", "no-store")
	snapshot, err := s.tables.Playback(r.Context(), viewer(token), r.PathValue("tableID"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, snapshot)
}

func (s *Server) patchPlayback(w http.ResponseWriter, r *http.Request, token *auth.Token) {
	trackID, err := uuid.Parse(r.PathValue("trackID"))
	if err != nil {
		http.Error(w, "Invalid track ID", http.StatusBadRequest)
		return
	}
	var body struct {
		IsPlaying   *bool                    `json:"isPlaying"`
		Volume      *float64                 `json:"volume"`
		CurrentTime *float64                 `json:"currentTime"`
		Duration    *float64                 `json:"duration"`
		Playback    *domain.PlaybackSettings `json:"playback"`
	}
	if !decodeCommand(w, r, &body) {
		return
	}

	err = s.tables.UpdatePlayback(r.Context(), viewer(token), r.PathValue("tableID"), trackID, events.PlaybackPatch{
		IsPlaying:   body.IsPlaying,
		Volume:      body.Volume,
		CurrentTime: body.CurrentTime,
		Duration:    body.Duration,
		Playback:    body.Playback,
	})
	if err != nil {
		s.writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) putMix(w http.ResponseWriter, r *http.Request, token *auth.Token) {
	var mix events.MixSnapshot
	if !decodeCommand(w, r, &mix) {
		return
	}

	if err := s.tables.ReplaceMix(r.Context(), viewer(token), r.PathValue("tableID"), mix); err != nil {
		s.writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) patchMix(w http.ResponseWriter, r *http.Request, token *auth.Token) {
	var patch events.MixVolumePatch
	if !decodeCommand(w, r, &patch) {
		return
	}

	if err := s.tables.PatchMix(r.Context(), viewer(token), r.PathValue("tableID"), patch); err != nil {
		s.writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func decodeCommand[T any](w http.ResponseWriter, r *http.Request, out *T) bool {
	var body *T
	if err := decodeStrict(w, r, 1<<20, &body); err != nil || body == nil {
		http.Error(w, "Invalid command body", http.StatusBadRequest)
		return false
	}
	*out = *body
	return true
}
