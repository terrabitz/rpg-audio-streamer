package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
	"github.com/terrabitz/rpg-audio-streamer/internal/events"
)

func (s *Server) getTags(w http.ResponseWriter, r *http.Request, _ *auth.Token) {
	tags, err := s.store.GetTags(r.Context())
	if err != nil {
		s.tagError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, tags)
}

func (s *Server) createTag(w http.ResponseWriter, r *http.Request, _ *auth.Token) {
	tag, ok := readTag(w, r)
	if !ok {
		return
	}
	tag.ID = uuid.New()
	created, err := s.store.CreateTag(r.Context(), tag)
	if err != nil {
		s.tagError(w, err)
		return
	}
	if !s.sendEvent(w, events.TagUpdated, created) {
		return
	}
	respondJSON(w, http.StatusCreated, created)
}

func (s *Server) updateTag(w http.ResponseWriter, r *http.Request, _ *auth.Token) {
	tagID, err := uuid.Parse(r.PathValue("tagID"))
	if err != nil {
		http.Error(w, "Invalid tag ID", http.StatusBadRequest)
		return
	}
	tag, ok := readTag(w, r)
	if !ok {
		return
	}
	tag.ID = tagID
	updated, err := s.store.UpdateTag(r.Context(), tag)
	if err != nil {
		s.tagError(w, err)
		return
	}
	if !s.sendEvent(w, events.TagUpdated, updated) {
		return
	}
	respondJSON(w, http.StatusOK, updated)
}

func (s *Server) editTracks(w http.ResponseWriter, r *http.Request, _ *auth.Token) {
	var body struct {
		TrackIDs     []uuid.UUID `json:"trackIDs"`
		TypeID       *uuid.UUID  `json:"typeID"`
		AddTagIDs    []uuid.UUID `json:"addTagIDs"`
		RemoveTagIDs []uuid.UUID `json:"removeTagIDs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	edit, err := domain.ParseTrackEdit(body.TrackIDs, body.TypeID, body.AddTagIDs, body.RemoveTagIDs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	tracks, err := s.store.EditTracks(r.Context(), edit)
	if err != nil {
		s.tagError(w, err)
		return
	}
	for _, track := range tracks {
		if !s.sendEvent(w, events.TrackUpdated, track.ID.String()) {
			return
		}
	}
	if !s.deleteUnusedTags(w, r.Context(), edit.RemoveTagIDs) {
		return
	}
	respondJSON(w, http.StatusOK, tracks)
}

// deleteUnusedTags removes candidate tags that no track uses any more.
func (s *Server) deleteUnusedTags(w http.ResponseWriter, ctx context.Context, tagIDs []uuid.UUID) bool {
	if len(tagIDs) == 0 {
		return true
	}
	deleted, err := s.store.DeleteUnusedTags(ctx, tagIDs)
	if err != nil {
		s.tagError(w, err)
		return false
	}
	for _, tagID := range deleted {
		if !s.sendEvent(w, events.TagDeleted, events.TagReference{TagID: tagID.String()}) {
			return false
		}
	}
	return true
}

func (s *Server) tagError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		http.Error(w, "Tag or track not found", http.StatusNotFound)
	case errors.Is(err, domain.ErrTagNameTaken):
		http.Error(w, err.Error(), http.StatusConflict)
	default:
		s.logger.Error("tag operation failed", "error", err)
		http.Error(w, "Tag operation failed", http.StatusInternalServerError)
	}
}

func readTag(w http.ResponseWriter, r *http.Request) (domain.Tag, bool) {
	var body struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return domain.Tag{}, false
	}
	name, err := domain.ParseTagName(body.Name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return domain.Tag{}, false
	}
	color, err := domain.ParseTagColor(body.Color)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return domain.Tag{}, false
	}
	return domain.Tag{
		Name:  name,
		Color: color,
	}, true
}
