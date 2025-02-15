package server

import (
	"context"
	"testing"
)

type MockTrackStore struct {
	tracks []Track
}

func (m *MockTrackStore) SaveTrack(ctx context.Context, track *Track) error {
	m.tracks = append(m.tracks, *track)
	return nil
}

func (m *MockTrackStore) GetTracks() []Track {
	return m.tracks
}

func NewMockTrackStore(t *testing.T) *MockTrackStore {
	t.Helper()

	return &MockTrackStore{
		tracks: []Track{},
	}
}
