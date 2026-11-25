package library

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
)

type Import struct {
	URL    string
	Name   string
	TypeID uuid.UUID
	// TableID optionally adds the new track to a table; uuid.Nil keeps it library-only.
	TableID uuid.UUID
}

// Import queues a remote video's audio for download and processing into a new track.
func (s *Service) Import(ctx context.Context, viewer domain.Viewer, request Import) (domain.Track, error) {
	if err := requireGM(viewer); err != nil {
		return domain.Track{}, err
	}
	if err := s.videos.ValidateURL(request.URL); err != nil {
		return domain.Track{}, err
	}
	name, err := parseTrackName(request.Name)
	if err != nil {
		return domain.Track{}, err
	}
	if err := s.requireTrackType(ctx, request.TypeID); err != nil {
		return domain.Track{}, err
	}
	if err := s.requireTable(ctx, request.TableID); err != nil {
		return domain.Track{}, err
	}
	track, err := s.audio.Import(ctx, name, request.TypeID, request.TableID, request.URL)
	if err != nil {
		return domain.Track{}, queueError(err)
	}
	return track, nil
}

func parseTrackName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" || utf8.RuneCountInString(name) > 200 {
		return "", domain.ErrInvalidTrackName
	}
	return name, nil
}
