package voice

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
	"github.com/terrabitz/rpg-audio-streamer/internal/mixer"
)

type fakeClipStore struct {
	onTable bool
	track   domain.Track
	types   map[uuid.UUID]domain.TrackType
}

func (s *fakeClipStore) HasTableTrack(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return s.onTable, nil
}

func (s *fakeClipStore) GetTrackByID(_ context.Context, id uuid.UUID) (domain.Track, error) {
	if id != s.track.ID {
		return domain.Track{}, domain.ErrNotFound
	}
	return s.track, nil
}

func (s *fakeClipStore) GetTrackTypeByID(_ context.Context, id uuid.UUID) (domain.TrackType, error) {
	trackType, ok := s.types[id]
	if !ok {
		return domain.TrackType{}, domain.ErrNotFound
	}
	return trackType, nil
}

type silence struct{}

func (silence) Len() int                        { return 0 }
func (silence) FrameAt(int) (mixer.Frame, bool) { return mixer.Frame{}, false }
func (silence) Err() error                      { return nil }
func (silence) Close() error                    { return nil }

func newClipStore() *fakeClipStore {
	ambiance := domain.TrackType{
		ID:          uuid.New(),
		Name:        "Ambiance",
		IsRepeating: true,
	}
	end := 30.0
	return &fakeClipStore{
		onTable: true,
		track: domain.Track{
			ID:     uuid.New(),
			Path:   "/uploads/rain",
			TypeID: ambiance.ID,
			Status: "ready",
			Playback: domain.PlaybackSettings{
				StartTime: 5,
				EndTime:   &end,
			},
		},
		types: map[uuid.UUID]domain.TrackType{ambiance.ID: ambiance},
	}
}

func TestClipsLoadDecodesTableTrackWithTypeRules(t *testing.T) {
	store := newClipStore()
	var decoded string
	clips := NewClips(store, func(dir string) (mixer.Source, error) {
		decoded = dir
		return silence{}, nil
	})
	clip, err := clips.Load(context.Background(), uuid.NewString(), store.track.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	if decoded != "/uploads/rain" || clip.Source == nil {
		t.Fatalf("decoded %q", decoded)
	}
	if clip.Type != "Ambiance" || !clip.Repeating || clip.Playback.StartTime != 5 || *clip.Playback.EndTime != 30 {
		t.Fatalf("clip: %+v", clip)
	}
}

func TestClipsLoadRejectsUnplayableTracks(t *testing.T) {
	errDecode := errors.New("ffmpeg failed")
	tests := []struct {
		name   string
		table  string
		file   func(*fakeClipStore) string
		modify func(*fakeClipStore)
		decode error
		want   error
	}{
		{name: "bad table ID", table: "table", want: nil},
		{name: "bad track ID", file: func(*fakeClipStore) string { return "track" }, want: nil},
		{name: "not on table", modify: func(s *fakeClipStore) { s.onTable = false }, want: errTrackNotOnTable},
		{name: "unknown track", file: func(*fakeClipStore) string { return uuid.NewString() }, want: domain.ErrNotFound},
		{name: "processing", modify: func(s *fakeClipStore) { s.track.Status = "processing" }, want: errTrackNotReady},
		{name: "unknown type", modify: func(s *fakeClipStore) { s.types = nil }, want: domain.ErrNotFound},
		{name: "decode failure", decode: errDecode, want: errDecode},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newClipStore()
			if tt.modify != nil {
				tt.modify(store)
			}
			tableID := uuid.NewString()
			if tt.table != "" {
				tableID = tt.table
			}
			fileID := store.track.ID.String()
			if tt.file != nil {
				fileID = tt.file(store)
			}
			clips := NewClips(store, func(string) (mixer.Source, error) { return silence{}, tt.decode })
			_, err := clips.Load(context.Background(), tableID, fileID)
			if err == nil || (tt.want != nil && !errors.Is(err, tt.want)) {
				t.Fatalf("load: %v, want %v", err, tt.want)
			}
		})
	}
}
