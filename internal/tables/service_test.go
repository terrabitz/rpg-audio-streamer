package tables

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
	"github.com/terrabitz/rpg-audio-streamer/internal/events"
)

var gm = domain.GMViewer()

func TestResolveLimitsPlayersToTheirOwnTable(t *testing.T) {
	f := newFixture()
	ctx := context.Background()

	if table, err := f.service.Resolve(ctx, gm, f.other.ExternalID); err != nil || table.ID != f.other.ID {
		t.Fatalf("GM resolve: %+v %v", table, err)
	}
	if table, err := f.service.Resolve(ctx, f.player(), f.table.ExternalID); err != nil || table.ID != f.table.ID {
		t.Fatalf("player resolve: %+v %v", table, err)
	}
	if _, err := f.service.Resolve(ctx, f.player(), f.other.ExternalID); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("player resolved another table: %v", err)
	}
	if _, err := f.service.Resolve(ctx, gm, "missing"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing table: %v", err)
	}
}

func TestGMOnlyUseCasesRejectPlayers(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	player := f.player()
	link := f.table.ExternalID
	trackID := uuid.New()
	attempts := map[string]func() error{
		"List": func() error { _, err := f.service.List(ctx, player); return err },
		"Create": func() error {
			_, err := f.service.Create(ctx, player, "Name")
			return err
		},
		"Rename": func() error {
			_, err := f.service.Rename(ctx, player, link, "Name")
			return err
		},
		"Delete":     func() error { return f.service.Delete(ctx, player, link) },
		"RotateLink": func() error { _, err := f.service.RotateLink(ctx, player, link); return err },
		"Settings":   func() error { _, err := f.service.Settings(ctx, player, link); return err },
		"UpdateSettings": func() error {
			_, err := f.service.UpdateSettings(ctx, player, link, domain.TableSettingsPatch{})
			return err
		},
		"AddTrack":    func() error { return f.service.AddTrack(ctx, player, link, trackID) },
		"RemoveTrack": func() error { return f.service.RemoveTrack(ctx, player, link, trackID) },
		"UpdatePlayback": func() error {
			return f.service.UpdatePlayback(ctx, player, link, trackID, events.PlaybackPatch{})
		},
		"ReplaceMix": func() error {
			return f.service.ReplaceMix(ctx, player, link, events.MixSnapshot{Tracks: []events.PlaybackPatch{}})
		},
		"PatchMix": func() error {
			return f.service.PatchMix(ctx, player, link, events.MixVolumePatch{TypeVolumes: map[string]float64{}})
		},
	}
	for name, attempt := range attempts {
		if err := attempt(); !errors.Is(err, ErrForbidden) {
			t.Errorf("%s by player: %v", name, err)
		}
	}
	if len(f.events.sent) != 0 || len(f.access.revoked) != 0 || len(f.playback.updated) != 0 {
		t.Fatalf("rejected use cases had side effects: %+v %+v %+v", f.events.sent, f.access.revoked, f.playback.updated)
	}
}

func TestListReturnsAllTables(t *testing.T) {
	f := newFixture()
	tables, err := f.service.List(context.Background(), gm)
	if err != nil || len(tables) != 2 {
		t.Fatalf("list: %+v %v", tables, err)
	}
}

func TestCreateStoresTrimmedName(t *testing.T) {
	f := newFixture()
	table, err := f.service.Create(context.Background(), gm, "  Campaign  ")
	if err != nil {
		t.Fatal(err)
	}
	if table.ID == uuid.Nil || table.Name != "Campaign" || f.store.tables[table.ID].Name != "Campaign" {
		t.Fatalf("created table: %+v", table)
	}
	for _, name := range []string{"", "   ", strings.Repeat("x", 201)} {
		if _, err := f.service.Create(context.Background(), gm, name); !errors.Is(err, domain.ErrInvalidTableName) {
			t.Errorf("create %q: %v", name, err)
		}
	}
}

func TestGetAllowsGMAndTablePlayer(t *testing.T) {
	f := newFixture()
	for _, viewer := range []domain.Viewer{gm, f.player()} {
		table, err := f.service.Get(context.Background(), viewer, f.table.ExternalID)
		if err != nil || table != f.table {
			t.Fatalf("get as %+v: %+v %v", viewer, table, err)
		}
	}
}

func TestRenameValidatesName(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	if _, err := f.service.Rename(ctx, gm, f.table.ExternalID, " "); !errors.Is(err, domain.ErrInvalidTableName) {
		t.Fatalf("blank rename: %v", err)
	}
	table, err := f.service.Rename(ctx, gm, f.table.ExternalID, " Saturday ")
	if err != nil || table.Name != "Saturday" {
		t.Fatalf("rename: %+v %v", table, err)
	}
}

func TestDeleteRemovesTableAndNotifiesClients(t *testing.T) {
	f := newFixture()
	if err := f.service.Delete(context.Background(), gm, f.table.ExternalID); err != nil {
		t.Fatal(err)
	}
	if _, ok := f.store.tables[f.table.ID]; ok {
		t.Fatal("table was not deleted")
	}
	if len(f.events.sent) != 1 || f.events.sent[0].name != events.TableDeleted || f.events.sent[0].payload != f.table.ID.String() {
		t.Fatalf("events: %+v", f.events.sent)
	}
}

func TestDeleteReportsLiveSyncFailureAfterSaving(t *testing.T) {
	f := newFixture()
	f.events.fail = errSend
	err := f.service.Delete(context.Background(), gm, f.table.ExternalID)
	if !errors.Is(err, domain.ErrLiveSyncFailed) || !errors.Is(err, errSend) {
		t.Fatalf("delete: %v", err)
	}
	if _, ok := f.store.tables[f.table.ID]; ok {
		t.Fatal("table was not deleted before the event failed")
	}
}

func TestRotateLinkRevokesOldPlayerAccess(t *testing.T) {
	f := newFixture()
	table, err := f.service.RotateLink(context.Background(), gm, f.table.ExternalID)
	if err != nil || table.ExternalID != "rotated" {
		t.Fatalf("rotate: %+v %v", table, err)
	}
	if len(f.access.revoked) != 1 || f.access.revoked[0] != f.table.ID.String() {
		t.Fatalf("revoked: %+v", f.access.revoked)
	}
}

func TestRotateLinkKeepsAccessWhenStoreFails(t *testing.T) {
	f := newFixture()
	f.store.fail = errStore
	if _, err := f.service.RotateLink(context.Background(), gm, f.table.ExternalID); !errors.Is(err, errStore) {
		t.Fatalf("rotate: %v", err)
	}
	if len(f.access.revoked) != 0 {
		t.Fatalf("revoked after failure: %+v", f.access.revoked)
	}
}

func TestSettingsReturnsStoredSettings(t *testing.T) {
	f := newFixture()
	f.store.settings[f.table.ID] = domain.TableSettings{MasterVolume: 80}
	settings, err := f.service.Settings(context.Background(), gm, f.table.ExternalID)
	if err != nil || settings.MasterVolume != 80 {
		t.Fatalf("settings: %+v %v", settings, err)
	}
}

func TestUpdateSettingsSavesValidPatchAndNotifiesClients(t *testing.T) {
	f := newFixture()
	volume := 120.0
	patch := domain.TableSettingsPatch{MasterVolume: &volume}
	settings, err := f.service.UpdateSettings(context.Background(), gm, f.table.ExternalID, patch)
	if err != nil || settings.MasterVolume != 120 {
		t.Fatalf("update: %+v %v", settings, err)
	}
	want := events.TableEvent[domain.TableSettingsPatch]{
		TableID: f.table.ID.String(),
		Data:    patch,
	}
	if len(f.events.sent) != 1 || f.events.sent[0].name != events.TableSettingsChanged {
		t.Fatalf("events: %+v", f.events.sent)
	}
	if got := f.events.sent[0].payload.(events.TableEvent[domain.TableSettingsPatch]); got.TableID != want.TableID || *got.Data.MasterVolume != volume {
		t.Fatalf("payload: %+v", got)
	}
}

func TestUpdateSettingsRejectsInvalidPatch(t *testing.T) {
	f := newFixture()
	volume := 250.0
	_, err := f.service.UpdateSettings(context.Background(), gm, f.table.ExternalID, domain.TableSettingsPatch{MasterVolume: &volume})
	if !errors.Is(err, domain.ErrInvalidTableSettings) {
		t.Fatalf("update: %v", err)
	}
	if f.store.settings[f.table.ID].MasterVolume != 0 || len(f.events.sent) != 0 {
		t.Fatal("invalid settings were applied")
	}
}

func TestTracksHideProcessingErrorsFromPlayers(t *testing.T) {
	f := newFixture()
	f.store.tableTracks[f.table.ID] = []domain.Track{{ID: uuid.New(), ProcessingError: "ffmpeg exited"}}
	gmTracks, err := f.service.Tracks(context.Background(), gm, f.table.ExternalID)
	if err != nil || gmTracks[0].ProcessingError == "" {
		t.Fatalf("GM tracks: %+v %v", gmTracks, err)
	}
	playerTracks, err := f.service.Tracks(context.Background(), f.player(), f.table.ExternalID)
	if err != nil || playerTracks[0].ProcessingError != "" {
		t.Fatalf("player tracks: %+v %v", playerTracks, err)
	}
	if _, err := f.service.Tracks(context.Background(), f.player(), f.other.ExternalID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("other table tracks: %v", err)
	}
}

func TestAddTrackAssociatesTrackWithTable(t *testing.T) {
	f := newFixture()
	trackID := uuid.New()
	if err := f.service.AddTrack(context.Background(), gm, f.table.ExternalID, trackID); err != nil {
		t.Fatal(err)
	}
	if tracks := f.store.tableTracks[f.table.ID]; len(tracks) != 1 || tracks[0].ID != trackID {
		t.Fatalf("table tracks: %+v", tracks)
	}
}

func TestRemoveTrackStopsItForTableClients(t *testing.T) {
	f := newFixture()
	trackID := uuid.New()
	f.store.tableTracks[f.table.ID] = []domain.Track{{ID: trackID}}
	if err := f.service.RemoveTrack(context.Background(), gm, f.table.ExternalID, trackID); err != nil {
		t.Fatal(err)
	}
	if len(f.store.tableTracks[f.table.ID]) != 0 {
		t.Fatal("track was not removed")
	}
	payload := f.events.sent[0].payload.(events.TableEvent[events.TrackReference])
	if f.events.sent[0].name != events.TrackRemoved || payload.TableID != f.table.ID.String() || payload.Data.FileID != trackID.String() {
		t.Fatalf("event: %+v", f.events.sent[0])
	}
}

func TestRemoveTrackDoesNotNotifyWhenStoreFails(t *testing.T) {
	f := newFixture()
	f.store.fail = errStore
	if err := f.service.RemoveTrack(context.Background(), gm, f.table.ExternalID, uuid.New()); !errors.Is(err, errStore) {
		t.Fatalf("remove: %v", err)
	}
	if len(f.events.sent) != 0 {
		t.Fatalf("events: %+v", f.events.sent)
	}
}

func TestPlaybackAppliesMasterVolumeForPlayers(t *testing.T) {
	f := newFixture()
	f.playback.snapshot = events.Snapshot{
		Tracks:   []events.PlaybackTrack{{FileID: "music", Volume: 80}},
		Settings: &domain.TableSettings{MasterVolume: 50},
	}
	snapshot, err := f.service.Playback(context.Background(), gm, f.table.ExternalID)
	if err != nil || snapshot.Settings == nil || snapshot.Tracks[0].Volume != 80 {
		t.Fatalf("GM playback: %+v %v", snapshot, err)
	}
	snapshot, err = f.service.Playback(context.Background(), f.player(), f.table.ExternalID)
	if err != nil || snapshot.Settings != nil || snapshot.Tracks[0].Volume != 40 {
		t.Fatalf("player playback: %+v %v", snapshot, err)
	}
}

func TestUpdatePlaybackForwardsPatch(t *testing.T) {
	f := newFixture()
	playing := true
	err := f.service.UpdatePlayback(context.Background(), gm, f.table.ExternalID, uuid.New(), events.PlaybackPatch{IsPlaying: &playing})
	if err != nil || len(f.playback.updated) != 1 || !*f.playback.updated[0].IsPlaying {
		t.Fatalf("update: %+v %v", f.playback.updated, err)
	}
}

func TestReplaceMixRequiresTracksAndDispatchesSnapshot(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	if err := f.service.ReplaceMix(ctx, gm, f.table.ExternalID, events.MixSnapshot{}); !errors.Is(err, ErrMixRequiresTracks) {
		t.Fatalf("missing tracks: %v", err)
	}
	mix := events.MixSnapshot{Tracks: []events.PlaybackPatch{{FileID: "music"}}}
	if err := f.service.ReplaceMix(ctx, gm, f.table.ExternalID, mix); err != nil {
		t.Fatal(err)
	}
	payload := f.events.sent[0].payload.(events.TableEvent[events.MixSnapshot])
	if f.events.sent[0].name != events.MixChanged || payload.TableID != f.table.ID.String() || payload.Data.Tracks[0].FileID != "music" {
		t.Fatalf("event: %+v", f.events.sent[0])
	}
}

func TestPatchMixRequiresVolumesAndDispatchesPatch(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	if err := f.service.PatchMix(ctx, gm, f.table.ExternalID, events.MixVolumePatch{}); !errors.Is(err, ErrMixRequiresVolumes) {
		t.Fatalf("missing volumes: %v", err)
	}
	patch := events.MixVolumePatch{TypeVolumes: map[string]float64{"Music": 40}}
	if err := f.service.PatchMix(ctx, gm, f.table.ExternalID, patch); err != nil {
		t.Fatal(err)
	}
	payload := f.events.sent[0].payload.(events.TableEvent[events.MixVolumePatch])
	if f.events.sent[0].name != events.MixVolumesChanged || payload.Data.TypeVolumes["Music"] != 40 {
		t.Fatalf("event: %+v", f.events.sent[0])
	}
}

func TestMixDispatchFailureIsUnavailable(t *testing.T) {
	f := newFixture()
	f.events.fail = errSend
	err := f.service.PatchMix(context.Background(), gm, f.table.ExternalID, events.MixVolumePatch{TypeVolumes: map[string]float64{}})
	if !errors.Is(err, ErrMixDispatchFailed) || !errors.Is(err, domain.ErrUnavailable) {
		t.Fatalf("patch: %v", err)
	}
}
