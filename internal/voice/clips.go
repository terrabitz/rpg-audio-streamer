package voice

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
	"github.com/terrabitz/rpg-audio-streamer/internal/livemixer"
	"github.com/terrabitz/rpg-audio-streamer/internal/mixer"
)

var (
	errTrackNotOnTable = errors.New("track is not associated with table")
	errTrackNotReady   = errors.New("track is not ready for playback")
)

type ClipStore interface {
	HasTableTrack(ctx context.Context, tableID, trackID uuid.UUID) (bool, error)
	GetTrackByID(context.Context, uuid.UUID) (domain.Track, error)
	GetTrackTypeByID(context.Context, uuid.UUID) (domain.TrackType, error)
}

// Decoder opens a processed track directory as PCM audio for the mixer.
type Decoder func(trackDir string) (mixer.Source, error)

// Clips loads table tracks into the Discord live mixer.
type Clips struct {
	store  ClipStore
	decode Decoder
}

func NewClips(store ClipStore, decode Decoder) *Clips {
	return &Clips{
		store:  store,
		decode: decode,
	}
}

// Load opens one of a table's ready tracks with the playback rules of its type.
func (c *Clips) Load(ctx context.Context, tableID, fileID string) (livemixer.Clip, error) {
	track, err := c.tableTrack(ctx, tableID, fileID)
	if err != nil {
		return livemixer.Clip{}, err
	}
	trackType, err := c.store.GetTrackTypeByID(ctx, track.TypeID)
	if err != nil {
		return livemixer.Clip{}, err
	}
	source, err := c.decode(track.Path)
	if err != nil {
		return livemixer.Clip{}, fmt.Errorf("decode mixer audio: %w", err)
	}
	return livemixer.Clip{
		Source:    source,
		Type:      trackType.Name,
		Repeating: trackType.IsRepeating,
		Playback: livemixer.Playback{
			StartTime:    track.Playback.StartTime,
			EndTime:      track.Playback.EndTime,
			LoopBackTime: track.Playback.LoopBackTime,
		},
	}, nil
}

func (c *Clips) tableTrack(ctx context.Context, rawTableID, rawFileID string) (domain.Track, error) {
	tableID, err := uuid.Parse(rawTableID)
	if err != nil {
		return domain.Track{}, fmt.Errorf("invalid table ID: %w", err)
	}
	trackID, err := uuid.Parse(rawFileID)
	if err != nil {
		return domain.Track{}, fmt.Errorf("invalid track ID: %w", err)
	}
	onTable, err := c.store.HasTableTrack(ctx, tableID, trackID)
	if err != nil {
		return domain.Track{}, err
	}
	if !onTable {
		return domain.Track{}, errTrackNotOnTable
	}
	track, err := c.store.GetTrackByID(ctx, trackID)
	if err != nil {
		return domain.Track{}, err
	}
	if !track.Ready() {
		return domain.Track{}, errTrackNotReady
	}
	return track, nil
}
