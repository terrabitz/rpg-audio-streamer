package server

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
)

type MockTrackStore struct {
	tracks map[uuid.UUID]Track
}

func (m *MockTrackStore) SaveTrack(ctx context.Context, track *Track) error {
	m.tracks[track.ID] = *track
	return nil
}

func (m *MockTrackStore) GetTracks(ctx context.Context) ([]Track, error) {
	var result []Track
	for _, t := range m.tracks {
		result = append(result, t)
	}
	return result, nil
}

func (m *MockTrackStore) GetTrackByID(ctx context.Context, trackID uuid.UUID) (Track, error) {
	track, ok := m.tracks[trackID]
	if !ok {
		return Track{}, fmt.Errorf("track not found")
	}
	return track, nil
}

func (m *MockTrackStore) DeleteTrack(ctx context.Context, trackID uuid.UUID) error {
	if _, ok := m.tracks[trackID]; !ok {
		return fmt.Errorf("track not found")
	}
	delete(m.tracks, trackID)
	return nil
}

func NewMockTrackStore(t *testing.T) *MockTrackStore {
	t.Helper()

	return &MockTrackStore{
		tracks: make(map[uuid.UUID]Track),
	}
}
