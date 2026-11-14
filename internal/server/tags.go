package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
)

var ErrTagConflict = errors.New("a tag with this name already exists")
var ErrInvalidTag = errors.New("unknown or duplicate tag")
var tagColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

type Tag struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Color string    `json:"color"`
}

type TagStore interface {
	GetTags(context.Context) ([]Tag, error)
	CreateTag(context.Context, Tag) (Tag, error)
	UpdateTag(context.Context, Tag) (Tag, error)
	DeleteTag(context.Context, uuid.UUID) error
}

func (s *Server) handleTags(w http.ResponseWriter, r *http.Request, _ *auth.Token) {
	switch r.Method {
	case http.MethodGet:
		tags, err := s.store.GetTags(r.Context())
		if err != nil {
			s.tagError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, tags)
	case http.MethodPost:
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
		respondJSON(w, http.StatusCreated, created)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleTag(w http.ResponseWriter, r *http.Request, _ *auth.Token) {
	id, err := uuid.Parse(r.PathValue("tagID"))
	if err != nil {
		http.Error(w, "Invalid tag ID", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodPut:
		tag, ok := readTag(w, r)
		if !ok {
			return
		}
		tag.ID = id
		updated, err := s.store.UpdateTag(r.Context(), tag)
		if err != nil {
			s.tagError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, updated)
	case http.MethodDelete:
		if err := s.store.DeleteTag(r.Context(), id); err != nil {
			s.tagError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func readTag(w http.ResponseWriter, r *http.Request) (Tag, bool) {
	var tag Tag
	if err := json.NewDecoder(r.Body).Decode(&tag); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return tag, false
	}
	tag.Name = strings.TrimSpace(tag.Name)
	if utf8.RuneCountInString(tag.Name) < 1 || utf8.RuneCountInString(tag.Name) > 50 || !tagColorPattern.MatchString(tag.Color) {
		http.Error(w, "Tags require a name of 1-50 characters and a #RRGGBB color", http.StatusBadRequest)
		return tag, false
	}
	return tag, true
}

func (s *Server) tagError(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrTagConflict) {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	s.tableError(w, err)
}
