package tables

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
	"github.com/terrabitz/rpg-audio-streamer/internal/events"
)

var (
	errStore = errors.New("store failed")
	errSend  = errors.New("send failed")
)

type fakeStore struct {
	tables      map[uuid.UUID]domain.Table
	settings    map[uuid.UUID]domain.TableSettings
	tableTracks map[uuid.UUID][]domain.Track
	fail        error
}

func newFakeStore(tables ...domain.Table) *fakeStore {
	store := &fakeStore{
		tables:      make(map[uuid.UUID]domain.Table),
		settings:    make(map[uuid.UUID]domain.TableSettings),
		tableTracks: make(map[uuid.UUID][]domain.Track),
	}
	for _, table := range tables {
		store.tables[table.ID] = table
	}
	return store
}

func (s *fakeStore) GetTables(context.Context) ([]domain.Table, error) {
	tables := make([]domain.Table, 0, len(s.tables))
	for _, table := range s.tables {
		tables = append(tables, table)
	}
	return tables, s.fail
}

func (s *fakeStore) GetTable(_ context.Context, id uuid.UUID) (domain.Table, error) {
	table, ok := s.tables[id]
	if !ok {
		return domain.Table{}, domain.ErrNotFound
	}
	return table, nil
}

func (s *fakeStore) GetTableByExternalID(_ context.Context, externalID string) (domain.Table, error) {
	for _, table := range s.tables {
		if table.ExternalID == externalID {
			return table, nil
		}
	}
	return domain.Table{}, fmt.Errorf("lookup %q: %w", externalID, domain.ErrNotFound)
}

func (s *fakeStore) CreateTable(_ context.Context, table domain.Table) (domain.Table, error) {
	if s.fail != nil {
		return domain.Table{}, s.fail
	}
	table.ExternalID = "link-" + table.ID.String()
	s.tables[table.ID] = table
	return table, nil
}

func (s *fakeStore) UpdateTable(_ context.Context, id uuid.UUID, name string) (domain.Table, error) {
	table := s.tables[id]
	table.Name = name
	s.tables[id] = table
	return table, s.fail
}

func (s *fakeStore) RotateTableExternalID(_ context.Context, id uuid.UUID) (domain.Table, error) {
	if s.fail != nil {
		return domain.Table{}, s.fail
	}
	table := s.tables[id]
	table.ExternalID = "rotated"
	s.tables[id] = table
	return table, nil
}

func (s *fakeStore) DeleteTable(_ context.Context, id uuid.UUID) error {
	if s.fail != nil {
		return s.fail
	}
	delete(s.tables, id)
	return nil
}

func (s *fakeStore) GetTableSettings(_ context.Context, id uuid.UUID) (domain.TableSettings, error) {
	return s.settings[id], s.fail
}

func (s *fakeStore) UpdateTableSettings(_ context.Context, id uuid.UUID, patch domain.TableSettingsPatch) (domain.TableSettings, error) {
	if s.fail != nil {
		return domain.TableSettings{}, s.fail
	}
	settings := s.settings[id]
	if patch.MasterVolume != nil {
		settings.MasterVolume = *patch.MasterVolume
	}
	s.settings[id] = settings
	return settings, nil
}

func (s *fakeStore) GetTableTracks(_ context.Context, id uuid.UUID) ([]domain.Track, error) {
	return append([]domain.Track(nil), s.tableTracks[id]...), s.fail
}

func (s *fakeStore) AddTableTrack(_ context.Context, tableID, trackID uuid.UUID) error {
	if s.fail != nil {
		return s.fail
	}
	s.tableTracks[tableID] = append(s.tableTracks[tableID], domain.Track{ID: trackID})
	return nil
}

func (s *fakeStore) RemoveTableTrack(_ context.Context, tableID, trackID uuid.UUID) error {
	if s.fail != nil {
		return s.fail
	}
	var kept []domain.Track
	for _, track := range s.tableTracks[tableID] {
		if track.ID != trackID {
			kept = append(kept, track)
		}
	}
	s.tableTracks[tableID] = kept
	return nil
}

type fakePlayback struct {
	snapshot events.Snapshot
	updated  []events.PlaybackPatch
	fail     error
}

func (p *fakePlayback) Snapshot(context.Context, uuid.UUID) (events.Snapshot, error) {
	return p.snapshot, p.fail
}

func (p *fakePlayback) Update(_ context.Context, _, _ uuid.UUID, patch events.PlaybackPatch) error {
	if p.fail != nil {
		return p.fail
	}
	p.updated = append(p.updated, patch)
	return nil
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

type fakeAccess struct {
	revoked []string
}

func (a *fakeAccess) RevokePlayerAccess(tableID string) {
	a.revoked = append(a.revoked, tableID)
}

type fixture struct {
	service  *Service
	store    *fakeStore
	playback *fakePlayback
	events   *fakeEvents
	access   *fakeAccess
	table    domain.Table
	other    domain.Table
}

func newFixture() *fixture {
	table := domain.Table{
		ID:         uuid.New(),
		ExternalID: "table-link",
		Name:       "Friday",
	}
	other := domain.Table{
		ID:         uuid.New(),
		ExternalID: "other-link",
		Name:       "Sunday",
	}
	f := &fixture{
		store:    newFakeStore(table, other),
		playback: &fakePlayback{},
		events:   &fakeEvents{},
		access:   &fakeAccess{},
		table:    table,
		other:    other,
	}
	f.service = New(f.store, f.playback, f.access, f.events)
	return f
}

func (f *fixture) player() domain.Viewer {
	return domain.PlayerViewer(f.table.ID)
}
