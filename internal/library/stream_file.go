package library

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
)

var (
	ErrStreamFailed       = domain.Internal("Failed to stream file")
	ErrInvalidStreamPath  = domain.Invalid("Invalid stream path")
	ErrInvalidTrackPath   = domain.Invalid("Invalid track path")
	ErrInvalidStreamTrack = domain.Invalid("Invalid track ID")
)

// StreamFile finds the file on disk for a track's HLS playlist or segment.
//
// The GM may stream any ready track. Players stream through their table's
// link and only see tracks on that table.
func (s *Service) StreamFile(ctx context.Context, viewer domain.Viewer, tableExternalID, relativePath string) (string, error) {
	if err := s.authorizeStream(ctx, viewer, tableExternalID, relativePath); err != nil {
		return "", err
	}
	filePath, trackID, err := s.resolveTrackFile(relativePath)
	if err != nil {
		return "", err
	}
	track, err := s.store.GetTrackByID(ctx, trackID)
	if err != nil {
		return "", err
	}
	if !track.Ready() {
		return "", ErrTrackNotReady
	}
	return filePath, nil
}

func (s *Service) authorizeStream(ctx context.Context, viewer domain.Viewer, tableExternalID, relativePath string) error {
	if tableExternalID == "" {
		return requireGM(viewer)
	}
	table, err := s.tables.Resolve(ctx, viewer, tableExternalID)
	if err != nil {
		return err
	}
	if viewer.IsGM() {
		return nil
	}
	trackID, err := uuid.Parse(strings.Split(relativePath, "/")[0])
	if err != nil {
		return ErrInvalidStreamTrack
	}
	onTable, err := s.store.HasTableTrack(ctx, table.ID, trackID)
	if err != nil {
		return err
	}
	if !onTable {
		return ErrTrackNotFound
	}
	return nil
}

// resolveTrackFile keeps the path inside the upload directory and finds the
// track that owns it from the first path segment.
func (s *Service) resolveTrackFile(relativePath string) (string, uuid.UUID, error) {
	filePath := filepath.Join(s.uploadDir, relativePath)
	uploadPath, err := filepath.Abs(s.uploadDir)
	if err != nil {
		return "", uuid.Nil, ErrStreamFailed.WithCause(err)
	}
	absolutePath, err := filepath.Abs(filePath)
	if err != nil {
		return "", uuid.Nil, ErrStreamFailed.WithCause(err)
	}
	relativeToUpload, err := filepath.Rel(uploadPath, absolutePath)
	if err != nil || relativeToUpload == ".." || strings.HasPrefix(relativeToUpload, ".."+string(filepath.Separator)) {
		return "", uuid.Nil, ErrInvalidStreamPath
	}
	trackID, err := uuid.Parse(strings.Split(relativeToUpload, string(filepath.Separator))[0])
	if err != nil {
		return "", uuid.Nil, ErrInvalidTrackPath
	}
	return filePath, trackID, nil
}
