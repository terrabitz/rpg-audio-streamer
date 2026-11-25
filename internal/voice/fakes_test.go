package voice

import (
	"context"
	"errors"
	"io"
	"log/slog"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
	"github.com/terrabitz/rpg-audio-streamer/internal/events"
)

var (
	errStore   = errors.New("store failed")
	errDiscord = errors.New("discord gateway closed")
)

type fakeDiscord struct {
	connectFailures []error
	connected       map[string]domain.Destination
	disconnected    []string
	disconnectFail  error
	resolveFail     error
	setup           domain.DiscordSetup
	channels        []domain.DiscordChoice
	listFail        error
}

func newFakeDiscord() *fakeDiscord {
	return &fakeDiscord{connected: make(map[string]domain.Destination)}
}

func (d *fakeDiscord) Connect(_ context.Context, tableID string, destination domain.Destination) error {
	if len(d.connectFailures) > 0 {
		err := d.connectFailures[0]
		d.connectFailures = d.connectFailures[1:]
		return err
	}
	d.connected[tableID] = destination
	return nil
}

func (d *fakeDiscord) Disconnect(_ context.Context, tableID string) error {
	d.disconnected = append(d.disconnected, tableID)
	delete(d.connected, tableID)
	return d.disconnectFail
}

func (d *fakeDiscord) Resolve(_ context.Context, destination domain.Destination) (domain.Destination, error) {
	destination.GuildName = "Guild " + destination.GuildID
	destination.ChannelName = "Channel " + destination.ChannelID
	return destination, d.resolveFail
}

func (d *fakeDiscord) Status(tableID string) domain.DiscordStatus {
	destination, ok := d.connected[tableID]
	if !ok {
		return domain.DiscordStatus{
			Enabled: true,
			State:   "disconnected",
		}
	}
	return domain.DiscordStatus{
		Enabled:     true,
		State:       "connected",
		Destination: &destination,
	}
}

func (d *fakeDiscord) Setup(context.Context) (domain.DiscordSetup, error) {
	return d.setup, d.listFail
}

func (d *fakeDiscord) Channels(context.Context, string) ([]domain.DiscordChoice, error) {
	return d.channels, d.listFail
}

type fakeStore struct {
	destinations map[uuid.UUID]domain.Destination
	types        []domain.TrackType
	saveFail     error
}

func (s *fakeStore) GetTableDiscord(_ context.Context, id uuid.UUID) (*domain.Destination, error) {
	destination, ok := s.destinations[id]
	if !ok {
		return nil, nil
	}
	return &destination, nil
}

func (s *fakeStore) SaveTableDiscord(_ context.Context, id uuid.UUID, destination domain.Destination) error {
	if s.saveFail != nil {
		return s.saveFail
	}
	s.destinations[id] = destination
	return nil
}

func (s *fakeStore) GetTrackTypes(context.Context) ([]domain.TrackType, error) {
	return s.types, nil
}

type fakeTables struct {
	table domain.Table
}

func (t *fakeTables) Resolve(_ context.Context, viewer domain.Viewer, externalID string) (domain.Table, error) {
	if externalID != t.table.ExternalID {
		return domain.Table{}, domain.ErrNotFound
	}
	if !viewer.CanAccessTable(t.table.ID) {
		return domain.Table{}, domain.Forbidden("Forbidden")
	}
	return t.table, nil
}

type fakePlayback struct {
	snapshot events.Snapshot
}

func (p *fakePlayback) Snapshot(context.Context, uuid.UUID) (events.Snapshot, error) {
	return p.snapshot, nil
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
	service  *Service
	discord  *fakeDiscord
	store    *fakeStore
	playback *fakePlayback
	events   *fakeEvents
	table    domain.Table
}

func newFixture() *fixture {
	table := domain.Table{
		ID:         uuid.New(),
		ExternalID: "table-link",
	}
	f := &fixture{
		discord: newFakeDiscord(),
		store: &fakeStore{
			destinations: make(map[uuid.UUID]domain.Destination),
			types: []domain.TrackType{
				{Name: "Music"},
				{Name: "Ambiance"},
			},
		},
		playback: &fakePlayback{},
		events:   &fakeEvents{},
		table:    table,
	}
	f.service = New(f.discord, f.store, &fakeTables{table: table}, f.playback, f.events, discardLogger())
	return f
}

// unconfigured builds the service the way main does when Discord streaming is off.
func (f *fixture) unconfigured() *Service {
	return New(nil, f.store, &fakeTables{table: f.table}, f.playback, f.events, discardLogger())
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
