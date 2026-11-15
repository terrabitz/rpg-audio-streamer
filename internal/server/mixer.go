package server

import (
	"context"
	"fmt"
	"math"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/livemixer"
)

const mixerSampleRate = 48000

// RunMixer is separate from Handler so integration servers can own its cancellation.
func (s *Server) RunMixer(ctx context.Context) {
	if s.mixer != nil {
		s.mixer.Run(ctx)
	}
}

func (s *Server) mixerSource(ctx context.Context, tableID, fileID string) (livemixer.Clip, error) {
	table, err := uuid.Parse(tableID)
	if err != nil {
		return livemixer.Clip{}, fmt.Errorf("invalid table ID: %w", err)
	}
	file, err := uuid.Parse(fileID)
	if err != nil {
		return livemixer.Clip{}, fmt.Errorf("invalid track ID: %w", err)
	}
	associated, err := s.store.HasTableTrack(ctx, table, file)
	if err != nil {
		return livemixer.Clip{}, err
	}
	if !associated {
		return livemixer.Clip{}, fmt.Errorf("track is not associated with table")
	}
	track, err := s.store.GetTrackByID(ctx, file)
	if err != nil {
		return livemixer.Clip{}, err
	}
	trackType, err := s.store.GetTrackTypeByID(ctx, track.TypeID)
	if err != nil {
		return livemixer.Clip{}, err
	}
	playlist := filepath.Join(track.Path, "index.m3u8")
	segments, duration, err := readHLSPlaylist(playlist)
	if err != nil {
		return livemixer.Clip{}, fmt.Errorf("inspect mixer audio: %w", err)
	}
	length := int(math.Round(duration * mixerSampleRate))
	source, err := newWindowedPCMSource(mixerSampleRate, length, segments)
	if err != nil {
		return livemixer.Clip{}, err
	}
	return livemixer.Clip{
		Source: source, Type: trackType.Name, Repeating: trackType.IsRepeating,
		Playback: livemixer.Playback{
			StartTime: track.Playback.StartTime, EndTime: track.Playback.EndTime, LoopBackTime: track.Playback.LoopBackTime,
		},
	}, nil
}
