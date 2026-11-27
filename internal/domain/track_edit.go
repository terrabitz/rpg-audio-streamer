package domain

import (
	"errors"
	"slices"

	"github.com/google/uuid"
)

var (
	ErrNoTracksToEdit     = errors.New("select at least one track to edit")
	ErrEmptyTrackEdit     = errors.New("specify a track type or tags to change")
	ErrConflictingTagEdit = errors.New("a tag cannot be added and removed in the same edit")
)

// TrackEdit applies the same change to every listed track.
type TrackEdit struct {
	TrackIDs     []uuid.UUID
	TypeID       *uuid.UUID
	AddTagIDs    []uuid.UUID
	RemoveTagIDs []uuid.UUID
}

func ParseTrackEdit(trackIDs []uuid.UUID, typeID *uuid.UUID, addTagIDs, removeTagIDs []uuid.UUID) (TrackEdit, error) {
	edit := TrackEdit{
		TrackIDs:     uniqueIDs(trackIDs),
		TypeID:       typeID,
		AddTagIDs:    uniqueIDs(addTagIDs),
		RemoveTagIDs: uniqueIDs(removeTagIDs),
	}
	if len(edit.TrackIDs) == 0 {
		return TrackEdit{}, ErrNoTracksToEdit
	}
	if edit.TypeID == nil && len(edit.AddTagIDs) == 0 && len(edit.RemoveTagIDs) == 0 {
		return TrackEdit{}, ErrEmptyTrackEdit
	}
	for _, tagID := range edit.AddTagIDs {
		if slices.Contains(edit.RemoveTagIDs, tagID) {
			return TrackEdit{}, ErrConflictingTagEdit
		}
	}
	return edit, nil
}

func uniqueIDs(ids []uuid.UUID) []uuid.UUID {
	unique := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if !slices.Contains(unique, id) {
			unique = append(unique, id)
		}
	}
	return unique
}
