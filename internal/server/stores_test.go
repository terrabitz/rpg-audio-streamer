package server

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
)

type MockStore struct {
	tracks     map[uuid.UUID]Track
	trackTypes map[uuid.UUID]TrackType
	tables     map[uuid.UUID]Table
}

func NewMockStore(t *testing.T) *MockStore {
	t.Helper()

	store := &MockStore{
		tracks:     make(map[uuid.UUID]Track),
		trackTypes: make(map[uuid.UUID]TrackType),
		tables:     make(map[uuid.UUID]Table),
	}

	// Add default track types
	ambianceID := uuid.MustParse("1EC000A2-A7C9-11EE-A0E5-0242AC120002")
	musicID := uuid.MustParse("1EC000A2-A7C9-11EE-A0E5-0242AC120003")
	oneShotID := uuid.MustParse("1EC000A2-A7C9-11EE-A0E5-0242AC120004")

	store.trackTypes[ambianceID] = TrackType{
		ID:                    ambianceID,
		Name:                  "Ambiance",
		IsRepeating:           true,
		AllowSimultaneousPlay: true,
	}
	store.trackTypes[musicID] = TrackType{
		ID:                    musicID,
		Name:                  "Music",
		IsRepeating:           true,
		AllowSimultaneousPlay: false,
	}
	store.trackTypes[oneShotID] = TrackType{
		ID:                    oneShotID,
		Name:                  "One-Shot",
		IsRepeating:           false,
		AllowSimultaneousPlay: true,
	}

	store.tables[uuid.MustParse("2EC000A2-A7C9-11EE-A0E5-0242AC120005")] = Table{
		ID:         uuid.MustParse("2EC000A2-A7C9-11EE-A0E5-0242AC120005"),
		Name:       "Test Table",
		InviteCode: "TEST1234",
		CreatedAt:  time.Now(),
	}

	return store
}

func (m *MockStore) SaveTrack(ctx context.Context, track *Track) error {
	m.tracks[track.ID] = *track
	return nil
}

func (m *MockStore) GetTracks(ctx context.Context) ([]Track, error) {
	var result []Track
	for _, t := range m.tracks {
		result = append(result, t)
	}
	return result, nil
}

func (m *MockStore) GetTrackByID(ctx context.Context, trackID uuid.UUID) (Track, error) {
	track, ok := m.tracks[trackID]
	if !ok {
		return Track{}, fmt.Errorf("track not found")
	}
	return track, nil
}

func (m *MockStore) DeleteTrack(ctx context.Context, trackID uuid.UUID) error {
	if _, ok := m.tracks[trackID]; !ok {
		return fmt.Errorf("track not found")
	}
	delete(m.tracks, trackID)
	return nil
}

func (m *MockStore) UpdateTrack(ctx context.Context, trackID uuid.UUID, update UpdateTrackRequest) (Track, error) {
	track, ok := m.tracks[trackID]
	if !ok {
		return Track{}, fmt.Errorf("track not found")
	}

	if update.Name != nil {
		track.Name = *update.Name
	}

	if update.TypeID != nil {
		track.TypeID = *update.TypeID
	}

	m.tracks[trackID] = track
	return track, nil
}

func (m *MockStore) GetTrackTypes(ctx context.Context) ([]TrackType, error) {
	var result []TrackType
	for _, t := range m.trackTypes {
		result = append(result, t)
	}
	return result, nil
}

func (m *MockStore) GetTrackTypeByID(ctx context.Context, id uuid.UUID) (TrackType, error) {
	trackType, ok := m.trackTypes[id]
	if !ok {
		return TrackType{}, fmt.Errorf("track type not found")
	}
	return trackType, nil
}

func (m *MockStore) GetTables(ctx context.Context) ([]Table, error) {
	var result []Table
	for _, t := range m.tables {
		result = append(result, t)
	}
	return result, nil
}

func (m *MockStore) GetTableByInviteCode(ctx context.Context, inviteCode string) (Table, error) {
	for _, table := range m.tables {
		if table.InviteCode == inviteCode {
			return table, nil
		}
	}
	return Table{}, fmt.Errorf("table not found")
}
