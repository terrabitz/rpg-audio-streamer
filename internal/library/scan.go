package library

import (
	"context"
	"errors"

	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
)

var (
	ErrScanTimedOut = domain.Timeout("Video scan timed out. Please try again.")
	ErrScanFailed   = domain.Upstream("Could not retrieve video metadata. Check that the video is public and available.")
)

// Scan previews a remote video so the GM can confirm it before importing.
func (s *Service) Scan(ctx context.Context, viewer domain.Viewer, url string) (domain.VideoMetadata, error) {
	if err := requireGM(viewer); err != nil {
		return domain.VideoMetadata{}, err
	}
	metadata, err := s.videos.Scan(ctx, url)
	switch {
	case err == nil:
		return metadata, nil
	case domain.IsUserFacing(err):
		return domain.VideoMetadata{}, err
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		return domain.VideoMetadata{}, ErrScanTimedOut.WithCause(err)
	default:
		return domain.VideoMetadata{}, ErrScanFailed.WithCause(err)
	}
}
