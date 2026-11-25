package library

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
)

var ErrWaveformFailed = domain.Internal("Failed to load waveform")

// Waveform returns a ready track's peak overview, generating and saving it on first use.
func (s *Service) Waveform(ctx context.Context, viewer domain.Viewer, trackID uuid.UUID) (domain.Waveform, error) {
	if err := requireGM(viewer); err != nil {
		return domain.Waveform{}, err
	}
	track, err := s.findTrack(ctx, trackID)
	if err != nil {
		return domain.Waveform{}, err
	}
	if !track.Ready() {
		return domain.Waveform{}, ErrTrackNotReady
	}
	waveform, err := s.loadWaveform(ctx, track)
	if err != nil {
		return domain.Waveform{}, ErrWaveformFailed.WithCause(err)
	}
	return waveform, nil
}

func (s *Service) loadWaveform(ctx context.Context, track domain.Track) (domain.Waveform, error) {
	waveform, err := s.store.GetTrackWaveform(ctx, track.ID)
	if !errors.Is(err, domain.ErrNotFound) {
		return waveform, err
	}
	waveform, err = s.audio.GenerateWaveform(ctx, track)
	if err != nil {
		return domain.Waveform{}, err
	}
	if err := s.store.SaveTrackWaveform(ctx, track.ID, waveform); err != nil {
		return domain.Waveform{}, err
	}
	return waveform, nil
}
