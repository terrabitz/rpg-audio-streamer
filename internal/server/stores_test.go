package server

import (
	"context"
	"fmt"
	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/discord"
)

type MockTrackStore struct {
	testStore
	tracks       map[uuid.UUID]domain.Track
	waveforms    map[uuid.UUID]domain.Waveform
	trackTypes   map[uuid.UUID]domain.TrackType
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
func (m *MockTrackStore) GetTable(ctx context.Context, id uuid.UUID) (domain.Table, error) {
	if id != domain.DefaultTableID {
		return domain.Table{}, domain.ErrNotFound
	}
	return domain.Table{ID: id, ExternalID: testDefaultExternalID, Name: "My Table"}, nil
}

func (m *MockTrackStore) GetTableByExternalID(ctx context.Context, externalID string) (domain.Table, error) {
	if externalID != testDefaultExternalID {
		return domain.Table{}, domain.ErrNotFound
	}
	return m.GetTable(ctx, domain.DefaultTableID)
}

func (m *MockTrackStore) RotateTableExternalID(ctx context.Context, id uuid.UUID) (domain.Table, error) {
	table, err := m.GetTable(ctx, id)
	if err != nil {
		return domain.Table{}, err
	}
	table.ExternalID = "bmV3LWRlZmF1bHQtZXh0ZXJuYWwtaWQ"
	return table, nil
}

const testDefaultExternalID = "ZGVmYXVsdC10YWJsZS1leHRlcm5hbC1pZA"

func (m *MockTrackStore) CountTrackTables(ctx context.Context, id uuid.UUID) (int64, error) {
	m.tracksMu.RLock()
	defer m.tracksMu.RUnlock()
	if _, ok := m.tracks[id]; !ok {
		return 0, domain.ErrNotFound
	}
	return 0, nil
}

func (m *MockTrackStore) SaveTrack(ctx context.Context, track *domain.Track) error {
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
		return domain.ErrNotFound
	}
	track.Status, track.ProcessingError = status, message
	m.tracks[id] = track
	return nil
}

func (m *MockTrackStore) GetTrackWaveform(_ context.Context, id uuid.UUID) (domain.Waveform, error) {
	m.tracksMu.RLock()
	defer m.tracksMu.RUnlock()
	waveform, ok := m.waveforms[id]
	if !ok {
		return domain.Waveform{}, domain.ErrNotFound
	}
	return waveform, nil
}

func (m *MockTrackStore) SaveTrackWaveform(_ context.Context, id uuid.UUID, waveform domain.Waveform) error {
	m.tracksMu.Lock()
	defer m.tracksMu.Unlock()
	if _, ok := m.tracks[id]; !ok {
		return domain.ErrNotFound
	}
	if m.waveforms == nil {
		m.waveforms = make(map[uuid.UUID]domain.Waveform)
	}
	m.waveforms[id] = waveform
	return nil
}

func (m *MockTrackStore) GetTracks(ctx context.Context) ([]domain.Track, error) {
	m.tracksMu.RLock()
	defer m.tracksMu.RUnlock()
	var result []domain.Track
	for _, t := range m.tracks {
		result = append(result, t)
	}
	return result, nil
}

func (m *MockTrackStore) GetTrackByID(ctx context.Context, trackID uuid.UUID) (domain.Track, error) {
	m.tracksMu.RLock()
	defer m.tracksMu.RUnlock()
	track, ok := m.tracks[trackID]
	if !ok {
		return domain.Track{}, domain.ErrNotFound
	}
	return track, nil
}

func (m *MockTrackStore) DeleteTrack(ctx context.Context, trackID uuid.UUID) error {
	m.tracksMu.Lock()
	defer m.tracksMu.Unlock()
	if _, ok := m.tracks[trackID]; !ok {
		return fmt.Errorf("track not found: %w", domain.ErrNotFound)
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

func (m *MockTrackStore) UpdateTrack(ctx context.Context, trackID uuid.UUID, update domain.TrackUpdate) (domain.Track, error) {
	m.tracksMu.Lock()
	defer m.tracksMu.Unlock()
	track, ok := m.tracks[trackID]
	if !ok {
		return domain.Track{}, fmt.Errorf("track not found: %w", domain.ErrNotFound)
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

func (m *MockTrackStore) GetTrackTypes(ctx context.Context) ([]domain.TrackType, error) {
	var result []domain.TrackType
	for _, t := range m.trackTypes {
		result = append(result, t)
	}
	return result, nil
}

func (m *MockTrackStore) GetTrackTypeByID(ctx context.Context, id uuid.UUID) (domain.TrackType, error) {
	trackType, ok := m.trackTypes[id]
	if !ok {
		return domain.TrackType{}, fmt.Errorf("track type not found: %w", domain.ErrNotFound)
	}
	return trackType, nil
}

func NewMockTrackStore(t *testing.T) *MockTrackStore {
	t.Helper()

	store := &MockTrackStore{
		tracks:     make(map[uuid.UUID]domain.Track),
		trackTypes: make(map[uuid.UUID]domain.TrackType),
	}

	// Add default track types
	ambianceID := uuid.MustParse("1EC000A2-A7C9-11EE-A0E5-0242AC120002")
	musicID := uuid.MustParse("1EC000A2-A7C9-11EE-A0E5-0242AC120003")
	oneShotID := uuid.MustParse("1EC000A2-A7C9-11EE-A0E5-0242AC120004")

	store.trackTypes[ambianceID] = domain.TrackType{
		ID:                    ambianceID,
		Name:                  "Ambiance",
		IsRepeating:           true,
		AllowSimultaneousPlay: true,
	}
	store.trackTypes[musicID] = domain.TrackType{
		ID:                    musicID,
		Name:                  "Music",
		IsRepeating:           true,
		AllowSimultaneousPlay: false,
	}
	store.trackTypes[oneShotID] = domain.TrackType{
		ID:                    oneShotID,
		Name:                  "One-Shot",
		IsRepeating:           false,
		AllowSimultaneousPlay: true,
	}

	return store
}
