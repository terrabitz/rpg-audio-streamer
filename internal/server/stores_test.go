package server

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
)

type MockTrackStore struct {
	tracks     map[uuid.UUID]Track
	trackTypes map[uuid.UUID]TrackType
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

func (m *MockTrackStore) GetTrackTypes(ctx context.Context) ([]TrackType, error) {
	var result []TrackType
	for _, t := range m.trackTypes {
		result = append(result, t)
	}
	return result, nil
}

func (m *MockTrackStore) GetTrackTypeByID(ctx context.Context, id uuid.UUID) (TrackType, error) {
	trackType, ok := m.trackTypes[id]
	if !ok {
		return TrackType{}, fmt.Errorf("track type not found")
	}
	return trackType, nil
}

func NewMockTrackStore(t *testing.T) *MockTrackStore {
	t.Helper()

	store := &MockTrackStore{
		tracks:     make(map[uuid.UUID]Track),
		trackTypes: make(map[uuid.UUID]TrackType),
	}

	// Add default track types
	ambianceID := uuid.MustParse("1EC000A2-A7C9-11EE-A0E5-0242AC120002")
	musicID := uuid.MustParse("1EC000A2-A7C9-11EE-A0E5-0242AC120003")
	oneShotID := uuid.MustParse("1EC000A2-A7C9-11EE-A0E5-0242AC120004")

	store.trackTypes[ambianceID] = TrackType{
		ID:                    ambianceID,
		Name:                  "Ambiance",
		IsRepeating:          true,
		AllowSimultaneousPlay: true,
	}
	store.trackTypes[musicID] = TrackType{
		ID:                    musicID,
		Name:                  "Music",
		IsRepeating:          true,
		AllowSimultaneousPlay: false,
	}
	store.trackTypes[oneShotID] = TrackType{
		ID:                    oneShotID,
		Name:                  "One-Shot",
		IsRepeating:          false,
		AllowSimultaneousPlay: true,
	}

	return store
}
