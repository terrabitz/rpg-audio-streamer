package library

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
	"github.com/terrabitz/rpg-audio-streamer/internal/events"
)

var (
	errStore = errors.New("store failed")
	errAudio = errors.New("audio failed")
	errSend  = errors.New("send failed")
)

type fakeStore struct {
	tracks      map[uuid.UUID]domain.Track
	types       map[uuid.UUID]domain.TrackType
	tables      map[uuid.UUID]domain.Table
	tableTracks map[uuid.UUID][]uuid.UUID
	waveforms   map[uuid.UUID]domain.Waveform
	fail        error
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		tracks:      make(map[uuid.UUID]domain.Track),
		types:       make(map[uuid.UUID]domain.TrackType),
		tables:      make(map[uuid.UUID]domain.Table),
		tableTracks: make(map[uuid.UUID][]uuid.UUID),
		waveforms:   make(map[uuid.UUID]domain.Waveform),
	}
}

func notFound(id uuid.UUID) error {
	return fmt.Errorf("%s: %w", id, domain.ErrNotFound)
}

func (s *fakeStore) GetTracks(context.Context) ([]domain.Track, error) {
	if s.fail != nil {
		return nil, s.fail
	}
	tracks := make([]domain.Track, 0, len(s.tracks))
	for _, track := range s.tracks {
		tracks = append(tracks, track)
	}
	return tracks, nil
}

func (s *fakeStore) GetTrackByID(_ context.Context, id uuid.UUID) (domain.Track, error) {
	track, ok := s.tracks[id]
	if !ok {
		return domain.Track{}, notFound(id)
	}
	return track, nil
}

func (s *fakeStore) UpdateTrack(_ context.Context, id uuid.UUID, update domain.TrackUpdate) (domain.Track, error) {
	if s.fail != nil {
		return domain.Track{}, s.fail
	}
	track, ok := s.tracks[id]
	if !ok {
		return domain.Track{}, notFound(id)
	}
	if update.Name != nil {
		track.Name = *update.Name
	}
	s.tracks[id] = track
	return track, nil
}

func (s *fakeStore) DeleteTrack(_ context.Context, id uuid.UUID) error {
	if s.fail != nil {
		return s.fail
	}
	delete(s.tracks, id)
	return nil
}

func (s *fakeStore) DeleteTrackIfTableCount(ctx context.Context, id uuid.UUID, expected int64) (bool, error) {
	count, err := s.CountTrackTables(ctx, id)
	if err != nil || count != expected {
		return false, err
	}
	return true, s.DeleteTrack(ctx, id)
}

func (s *fakeStore) CountTrackTables(_ context.Context, id uuid.UUID) (int64, error) {
	var count int64
	for _, trackIDs := range s.tableTracks {
		for _, trackID := range trackIDs {
			if trackID == id {
				count++
			}
		}
	}
	return count, nil
}

func (s *fakeStore) GetTrackTypes(context.Context) ([]domain.TrackType, error) {
	if s.fail != nil {
		return nil, s.fail
	}
	types := make([]domain.TrackType, 0, len(s.types))
	for _, trackType := range s.types {
		types = append(types, trackType)
	}
	return types, nil
}

func (s *fakeStore) GetTrackTypeByID(_ context.Context, id uuid.UUID) (domain.TrackType, error) {
	trackType, ok := s.types[id]
	if !ok {
		return domain.TrackType{}, notFound(id)
	}
	return trackType, nil
}

func (s *fakeStore) GetTable(_ context.Context, id uuid.UUID) (domain.Table, error) {
	table, ok := s.tables[id]
	if !ok {
		return domain.Table{}, notFound(id)
	}
	return table, nil
}

func (s *fakeStore) HasTableTrack(_ context.Context, tableID, trackID uuid.UUID) (bool, error) {
	for _, id := range s.tableTracks[tableID] {
		if id == trackID {
			return true, nil
		}
	}
	return false, nil
}

func (s *fakeStore) GetTrackWaveform(_ context.Context, id uuid.UUID) (domain.Waveform, error) {
	waveform, ok := s.waveforms[id]
	if !ok {
		return domain.Waveform{}, notFound(id)
	}
	return waveform, nil
}

func (s *fakeStore) SaveTrackWaveform(_ context.Context, id uuid.UUID, waveform domain.Waveform) error {
	if s.fail != nil {
		return s.fail
	}
	s.waveforms[id] = waveform
	return nil
}

type queuedTrack struct {
	name    string
	typeID  uuid.UUID
	tableID uuid.UUID
	source  string
}

type fakeAudio struct {
	queued    []queuedTrack
	removed   []domain.Track
	generated int
	waveform  domain.Waveform
	fail      error
}

func (a *fakeAudio) Upload(_ context.Context, name string, typeID, tableID uuid.UUID, input io.Reader) (domain.Track, error) {
	data, _ := io.ReadAll(input)
	return a.queue(name, typeID, tableID, string(data))
}

func (a *fakeAudio) Import(_ context.Context, name string, typeID, tableID uuid.UUID, sourceURL string) (domain.Track, error) {
	return a.queue(name, typeID, tableID, sourceURL)
}

func (a *fakeAudio) queue(name string, typeID, tableID uuid.UUID, source string) (domain.Track, error) {
	if a.fail != nil {
		return domain.Track{}, a.fail
	}
	a.queued = append(a.queued, queuedTrack{
		name:    name,
		typeID:  typeID,
		tableID: tableID,
		source:  source,
	})
	return domain.Track{
		ID:     uuid.New(),
		Name:   name,
		Status: "processing",
	}, nil
}

func (a *fakeAudio) Remove(track domain.Track) error {
	a.removed = append(a.removed, track)
	return a.fail
}

func (a *fakeAudio) GenerateWaveform(context.Context, domain.Track) (domain.Waveform, error) {
	a.generated++
	return a.waveform, a.fail
}

type fakeVideos struct {
	invalid  error
	metadata domain.VideoMetadata
	fail     error
}

func (v *fakeVideos) ValidateURL(string) error {
	return v.invalid
}

func (v *fakeVideos) Scan(context.Context, string) (domain.VideoMetadata, error) {
	return v.metadata, v.fail
}

// fakeTables mirrors tables.Service.Resolve: players may only use their own table.
type fakeTables struct {
	byLink map[string]domain.Table
}

func (t *fakeTables) Resolve(_ context.Context, viewer domain.Viewer, externalID string) (domain.Table, error) {
	table, ok := t.byLink[externalID]
	if !ok {
		return domain.Table{}, domain.ErrNotFound
	}
	if !viewer.CanAccessTable(table.ID) {
		return domain.Table{}, domain.Forbidden("Forbidden")
	}
	return table, nil
}

type recordedEvent struct {
	name    string
	payload any
}

type fakeEvents struct {
	sent []recordedEvent
	fail error
}

func (e *fakeEvents) SendEvent(name string, payload any) error {
	if e.fail != nil {
		return e.fail
	}
	e.sent = append(e.sent, recordedEvent{
		name:    name,
		payload: payload,
	})
	return nil
}

type fixture struct {
	service   *Service
	store     *fakeStore
	audio     *fakeAudio
	videos    *fakeVideos
	events    *fakeEvents
	uploadDir string
	table     domain.Table
	music     domain.TrackType
	track     domain.Track
}

func newFixture(uploadDir string) *fixture {
	table := domain.Table{
		ID:         uuid.New(),
		ExternalID: "table-link",
	}
	music := domain.TrackType{
		ID:   uuid.New(),
		Name: "Music",
	}
	trackID := uuid.New()
	track := domain.Track{
		ID:     trackID,
		Name:   "Tavern",
		Path:   uploadDir + "/" + trackID.String(),
		TypeID: music.ID,
		Status: "ready",
	}
	f := &fixture{
		store:     newFakeStore(),
		audio:     &fakeAudio{},
		videos:    &fakeVideos{},
		events:    &fakeEvents{},
		uploadDir: uploadDir,
		table:     table,
		music:     music,
		track:     track,
	}
	f.store.tables[table.ID] = table
	f.store.types[music.ID] = music
	f.store.tracks[track.ID] = track
	tables := &fakeTables{byLink: map[string]domain.Table{table.ExternalID: table}}
	f.service = New(f.store, f.audio, f.videos, tables, f.events, uploadDir)
	return f
}

func (f *fixture) player() domain.Viewer {
	return domain.PlayerViewer(f.table.ID)
}

func (f *fixture) lastEvent() recordedEvent {
	if len(f.events.sent) == 0 {
		return recordedEvent{}
	}
	return f.events.sent[len(f.events.sent)-1]
}

var _ events.Sender = (*fakeEvents)(nil)
