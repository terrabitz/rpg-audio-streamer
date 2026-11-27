package domain

import (
	"errors"
	"reflect"
	"testing"

	"github.com/google/uuid"
)

func TestParseTrackEdit(t *testing.T) {
	track, tag := uuid.New(), uuid.New()
	for _, tc := range []struct {
		name         string
		trackIDs     []uuid.UUID
		typeID       *uuid.UUID
		addTagIDs    []uuid.UUID
		removeTagIDs []uuid.UUID
		want         error
	}{
		{
			name:      "no tracks",
			addTagIDs: []uuid.UUID{tag},
			want:      ErrNoTracksToEdit,
		},
		{
			name:     "no changes",
			trackIDs: []uuid.UUID{track},
			want:     ErrEmptyTrackEdit,
		},
		{
			name:         "conflicting tag",
			trackIDs:     []uuid.UUID{track},
			addTagIDs:    []uuid.UUID{tag},
			removeTagIDs: []uuid.UUID{tag},
			want:         ErrConflictingTagEdit,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseTrackEdit(tc.trackIDs, tc.typeID, tc.addTagIDs, tc.removeTagIDs); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v; want %v", err, tc.want)
			}
		})
	}
	edit, err := ParseTrackEdit([]uuid.UUID{track, track}, nil, []uuid.UUID{tag, tag}, nil)
	want := TrackEdit{
		TrackIDs:     []uuid.UUID{track},
		AddTagIDs:    []uuid.UUID{tag},
		RemoveTagIDs: []uuid.UUID{},
	}
	if err != nil || !reflect.DeepEqual(edit, want) {
		t.Fatalf("edit = %+v, %v; want %+v", edit, err, want)
	}
}
