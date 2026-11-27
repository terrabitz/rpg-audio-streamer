package server

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/discord"
	"github.com/terrabitz/rpg-audio-streamer/internal/musicprocessing"
)

type MockTrackStore struct {
	TableStore
	TagStore
	tracks       map[uuid.UUID]Track
	waveforms    map[uuid.UUID]musicprocessing.Waveform
	trackTypes   map[uuid.UUID]TrackType
	destinations map[uuid.UUID]discord.Destination
	tracksMu     sync.RWMutex
}

func (m *MockTrackStore) GetTableDiscord(_ context.Context, id uuid.UUID) (*discord.Destination, error) {
	if target, ok := m.destinations[id]; ok {
		return &target, nil
	}
	return nil, nil
}

func (m *MockTrackStore) SaveTableDiscord(_ context.Context, id uuid.UUID, target discord.Destination) error {
	if m.destinations == nil {
		m.destinations = make(map[uuid.UUID]discord.Destination)
	}
	m.destinations[id] = target
	return nil
}
func (m *MockTrackStore) GetTable(ctx context.Context, id uuid.UUID) (Table, error) {
	if id != DefaultTableID {
		return Table{}, sql.ErrNoRows
	}
	return Table{ID: id, ExternalID: testDefaultExternalID, Name: "My Table"}, nil
}

func (m *MockTrackStore) GetTableByExternalID(ctx context.Context, externalID string) (Table, error) {
	if externalID != testDefaultExternalID {
		return Table{}, sql.ErrNoRows
	}
	return m.GetTable(ctx, DefaultTableID)
}

func (m *MockTrackStore) RotateTableExternalID(ctx context.Context, id uuid.UUID) (Table, error) {
	table, err := m.GetTable(ctx, id)
	if err != nil {
		return Table{}, err
	}
	table.ExternalID = "bmV3LWRlZmF1bHQtZXh0ZXJuYWwtaWQ"
	return table, nil
}

const testDefaultExternalID = "ZGVmYXVsdC10YWJsZS1leHRlcm5hbC1pZA"

func (m *MockTrackStore) CountTrackTables(ctx context.Context, id uuid.UUID) (int64, error) {
	m.tracksMu.RLock()
	defer m.tracksMu.RUnlock()
	if _, ok := m.tracks[id]; !ok {
		return 0, sql.ErrNoRows
	}
	return 0, nil
}

func (m *MockTrackStore) SaveTrack(ctx context.Context, track *Track) error {
	m.tracksMu.Lock()
	defer m.tracksMu.Unlock()
	m.tracks[track.ID] = *track
	return nil
}

func (m *MockTrackStore) SetTrackProcessing(ctx context.Context, id uuid.UUID, status, message string) error {
	m.tracksMu.Lock()
	defer m.tracksMu.Unlock()
	track, ok := m.tracks[id]
	if !ok || track.Status != "processing" {
		return sql.ErrNoRows
	}
	track.Status, track.ProcessingError = status, message
	m.tracks[id] = track
	return nil
}

func (m *MockTrackStore) GetTrackWaveform(_ context.Context, id uuid.UUID) (musicprocessing.Waveform, error) {
	m.tracksMu.RLock()
	defer m.tracksMu.RUnlock()
	waveform, ok := m.waveforms[id]
	if !ok {
		return musicprocessing.Waveform{}, sql.ErrNoRows
	}
	return waveform, nil
}

func (m *MockTrackStore) SaveTrackWaveform(_ context.Context, id uuid.UUID, waveform musicprocessing.Waveform) error {
	m.tracksMu.Lock()
	defer m.tracksMu.Unlock()
	if _, ok := m.tracks[id]; !ok {
		return sql.ErrNoRows
	}
	if m.waveforms == nil {
		m.waveforms = make(map[uuid.UUID]musicprocessing.Waveform)
	}
	m.waveforms[id] = waveform
	return nil
}

func (m *MockTrackStore) GetTracks(ctx context.Context) ([]Track, error) {
	m.tracksMu.RLock()
	defer m.tracksMu.RUnlock()
	var result []Track
	for _, t := range m.tracks {
		result = append(result, t)
	}
	return result, nil
}

func (m *MockTrackStore) GetTrackByID(ctx context.Context, trackID uuid.UUID) (Track, error) {
	m.tracksMu.RLock()
	defer m.tracksMu.RUnlock()
	track, ok := m.tracks[trackID]
	if !ok {
		return Track{}, sql.ErrNoRows
	}
	return track, nil
}

func (m *MockTrackStore) DeleteTrack(ctx context.Context, trackID uuid.UUID) error {
	m.tracksMu.Lock()
	defer m.tracksMu.Unlock()
	if _, ok := m.tracks[trackID]; !ok {
		return fmt.Errorf("track not found")
	}
	delete(m.tracks, trackID)
	return nil
}

func (m *MockTrackStore) DeleteTrackIfTableCount(ctx context.Context, trackID uuid.UUID, expectedCount int64) (bool, error) {
	count, err := m.CountTrackTables(ctx, trackID)
	if err != nil || count != expectedCount {
		return false, err
	}
	return true, m.DeleteTrack(ctx, trackID)
}

func (m *MockTrackStore) UpdateTrack(ctx context.Context, trackID uuid.UUID, update UpdateTrackRequest) (Track, error) {
	m.tracksMu.Lock()
	defer m.tracksMu.Unlock()
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
	if update.Volume != nil {
		track.Volume = *update.Volume
	}

	m.tracks[trackID] = track
	return track, nil
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

	return store
}
