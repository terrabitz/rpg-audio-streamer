package library

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

func TestGMOnlyUseCasesRejectPlayers(t *testing.T) {
	f := newFixture(t.TempDir())
	ctx := context.Background()
	player := f.player()
	id := f.track.ID
	attempts := map[string]func() error{
		"ListTracks": func() error { _, err := f.service.ListTracks(ctx, player); return err },
		"TrackUsage": func() error { _, err := f.service.TrackUsage(ctx, player, id); return err },
		"Upload": func() error {
			_, err := f.service.Upload(ctx, player, Upload{TypeID: f.music.ID, Audio: strings.NewReader("")})
			return err
		},
		"Import": func() error {
			_, err := f.service.Import(ctx, player, Import{Name: "Song", TypeID: f.music.ID})
			return err
		},
		"Scan": func() error { _, err := f.service.Scan(ctx, player, "url"); return err },
		"UpdateTrack": func() error {
			_, err := f.service.UpdateTrack(ctx, player, id, domain.TrackUpdate{})
			return err
		},
		"DeleteTrack": func() error { return f.service.DeleteTrack(ctx, player, id, nil) },
		"Waveform":    func() error { _, err := f.service.Waveform(ctx, player, id); return err },
		"StreamFile without table": func() error {
			_, err := f.service.StreamFile(ctx, player, "", id.String()+"/index.m3u8")
			return err
		},
	}
	for name, attempt := range attempts {
		if err := attempt(); !errors.Is(err, ErrForbidden) {
			t.Errorf("%s by player: %v", name, err)
		}
	}
	if len(f.audio.queued) != 0 || len(f.audio.removed) != 0 || len(f.events.sent) != 0 {
		t.Fatal("rejected use cases had side effects")
	}
}

func TestListTracksHidesStoreFailure(t *testing.T) {
	f := newFixture(t.TempDir())
	tracks, err := f.service.ListTracks(context.Background(), gm)
	if err != nil || len(tracks) != 1 {
		t.Fatalf("list: %+v %v", tracks, err)
	}
	f.store.fail = errStore
	if _, err := f.service.ListTracks(context.Background(), gm); !errors.Is(err, ErrListTracksFailed) || err.Error() != "Failed to list files" {
		t.Fatalf("list failure: %v", err)
	}
}

func TestTrackUsageCountsTables(t *testing.T) {
	f := newFixture(t.TempDir())
	f.store.tableTracks[f.table.ID] = []uuid.UUID{f.track.ID}
	count, err := f.service.TrackUsage(context.Background(), gm, f.track.ID)
	if err != nil || count != 1 {
		t.Fatalf("usage: %d %v", count, err)
	}
}

func TestUploadQueuesAudioForProcessing(t *testing.T) {
	f := newFixture(t.TempDir())
	track, err := f.service.Upload(context.Background(), gm, Upload{
		Name:    "Battle",
		TypeID:  f.music.ID,
		TableID: f.table.ID,
		Audio:   strings.NewReader("audio bytes"),
	})
	if err != nil || track.Name != "Battle" {
		t.Fatalf("upload: %+v %v", track, err)
	}
	want := queuedTrack{
		name:    "Battle",
		typeID:  f.music.ID,
		tableID: f.table.ID,
		source:  "audio bytes",
	}
	if len(f.audio.queued) != 1 || f.audio.queued[0] != want {
		t.Fatalf("queued: %+v", f.audio.queued)
	}
}

func TestUploadRejectsUnknownTableAndType(t *testing.T) {
	f := newFixture(t.TempDir())
	ctx := context.Background()
	_, err := f.service.Upload(ctx, gm, Upload{TypeID: f.music.ID, TableID: uuid.New(), Audio: strings.NewReader("")})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unknown table: %v", err)
	}
	_, err = f.service.Upload(ctx, gm, Upload{TypeID: uuid.New(), Audio: strings.NewReader("")})
	if !errors.Is(err, ErrInvalidTrackType) || !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("unknown type: %v", err)
	}
	if len(f.audio.queued) != 0 {
		t.Fatalf("queued: %+v", f.audio.queued)
	}
}

func TestUploadQueueFailureIsUnavailable(t *testing.T) {
	f := newFixture(t.TempDir())
	f.audio.fail = errAudio
	_, err := f.service.Upload(context.Background(), gm, Upload{TypeID: f.music.ID, Audio: strings.NewReader("")})
	if !errors.Is(err, ErrQueueFailed) || !errors.Is(err, errAudio) {
		t.Fatalf("upload: %v", err)
	}
}

func TestImportQueuesTrimmedName(t *testing.T) {
	f := newFixture(t.TempDir())
	_, err := f.service.Import(context.Background(), gm, Import{
		URL:    "https://youtu.be/abc",
		Name:   "  Theme  ",
		TypeID: f.music.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(f.audio.queued) != 1 || f.audio.queued[0].name != "Theme" || f.audio.queued[0].source != "https://youtu.be/abc" {
		t.Fatalf("queued: %+v", f.audio.queued)
	}
}

func TestImportValidatesRequestBeforeQueueing(t *testing.T) {
	invalidURL := domain.Invalid("Enter a YouTube video link")
	tests := []struct {
		name    string
		videos  fakeVideos
		request func(*fixture) Import
		want    error
	}{
		{
			name:   "invalid URL",
			videos: fakeVideos{invalid: invalidURL},
			request: func(f *fixture) Import {
				return Import{Name: "Song", TypeID: f.music.ID}
			},
			want: invalidURL,
		},
		{
			name: "blank name",
			request: func(f *fixture) Import {
				return Import{Name: "  ", TypeID: f.music.ID}
			},
			want: domain.ErrInvalidTrackName,
		},
		{
			name: "long name",
			request: func(f *fixture) Import {
				return Import{Name: strings.Repeat("é", 201), TypeID: f.music.ID}
			},
			want: domain.ErrInvalidTrackName,
		},
		{
			name: "unknown type",
			request: func(*fixture) Import {
				return Import{Name: "Song", TypeID: uuid.New()}
			},
			want: ErrInvalidTrackType,
		},
		{
			name: "unknown table",
			request: func(f *fixture) Import {
				return Import{Name: "Song", TypeID: f.music.ID, TableID: uuid.New()}
			},
			want: domain.ErrNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t.TempDir())
			*f.videos = tt.videos
			_, err := f.service.Import(context.Background(), gm, tt.request(f))
			if !errors.Is(err, tt.want) {
				t.Fatalf("import: %v, want %v", err, tt.want)
			}
			if len(f.audio.queued) != 0 {
				t.Fatalf("queued: %+v", f.audio.queued)
			}
		})
	}
}

func TestImportKeepsUserFacingQueueErrors(t *testing.T) {
	f := newFixture(t.TempDir())
	unavailable := domain.Unavailable("YouTube import requires yt-dlp on the server")
	f.audio.fail = unavailable
	_, err := f.service.Import(context.Background(), gm, Import{Name: "Song", TypeID: f.music.ID})
	if !errors.Is(err, unavailable) || errors.Is(err, ErrQueueFailed) {
		t.Fatalf("import: %v", err)
	}
}

func TestScanClassifiesFailures(t *testing.T) {
	tooLong := domain.Unsupported("Video is too long")
	tests := []struct {
		name string
		fail error
		want error
	}{
		{name: "user-facing", fail: tooLong, want: tooLong},
		{name: "deadline", fail: context.DeadlineExceeded, want: ErrScanTimedOut},
		{name: "canceled", fail: context.Canceled, want: ErrScanTimedOut},
		{name: "unknown", fail: errors.New("yt-dlp exited 1"), want: ErrScanFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t.TempDir())
			f.videos.fail = tt.fail
			if _, err := f.service.Scan(context.Background(), gm, "url"); !errors.Is(err, tt.want) {
				t.Fatalf("scan: %v", err)
			}
		})
	}
}

func TestScanReturnsMetadata(t *testing.T) {
	f := newFixture(t.TempDir())
	f.videos.metadata = domain.VideoMetadata{Name: "Theme", Duration: 60}
	metadata, err := f.service.Scan(context.Background(), gm, "url")
	if err != nil || metadata.Name != "Theme" {
		t.Fatalf("scan: %+v %v", metadata, err)
	}
}

func TestUpdateTrackSavesAndNotifiesClients(t *testing.T) {
	f := newFixture(t.TempDir())
	name := "Renamed"
	track, err := f.service.UpdateTrack(context.Background(), gm, f.track.ID, domain.TrackUpdate{Name: &name})
	if err != nil || track.Name != name {
		t.Fatalf("update: %+v %v", track, err)
	}
	if event := f.lastEvent(); event.name != events.TrackUpdated || event.payload != f.track.ID.String() {
		t.Fatalf("event: %+v", event)
	}
}

func TestUpdateTrackClassifiesFailures(t *testing.T) {
	f := newFixture(t.TempDir())
	ctx := context.Background()
	volume := 101.0
	if _, err := f.service.UpdateTrack(ctx, gm, f.track.ID, domain.TrackUpdate{Volume: &volume}); !errors.Is(err, domain.ErrInvalidTrackVolume) {
		t.Fatalf("invalid volume: %v", err)
	}
	if _, err := f.service.UpdateTrack(ctx, gm, uuid.New(), domain.TrackUpdate{}); !errors.Is(err, ErrTrackNotFound) {
		t.Fatalf("missing track: %v", err)
	}
	f.store.fail = errStore
	if _, err := f.service.UpdateTrack(ctx, gm, f.track.ID, domain.TrackUpdate{}); !errors.Is(err, ErrUpdateTrackFailed) {
		t.Fatalf("store failure: %v", err)
	}
	if len(f.events.sent) != 0 {
		t.Fatalf("events: %+v", f.events.sent)
	}
}

func TestDeleteTrackRemovesRecordFilesAndNotifiesClients(t *testing.T) {
	f := newFixture(t.TempDir())
	if err := f.service.DeleteTrack(context.Background(), gm, f.track.ID, nil); err != nil {
		t.Fatal(err)
	}
	if _, ok := f.store.tracks[f.track.ID]; ok {
		t.Fatal("track record was not deleted")
	}
	if len(f.audio.removed) != 1 || f.audio.removed[0].ID != f.track.ID {
		t.Fatalf("removed: %+v", f.audio.removed)
	}
	event := f.lastEvent()
	payload := event.payload.(events.TableEvent[events.TrackReference])
	if event.name != events.TrackRemoved || payload.TableID != "" || payload.Data.FileID != f.track.ID.String() {
		t.Fatalf("event: %+v", event)
	}
}

func TestDeleteTrackChecksExpectedTableCount(t *testing.T) {
	f := newFixture(t.TempDir())
	f.store.tableTracks[f.table.ID] = []uuid.UUID{f.track.ID}
	stale := int64(0)
	if err := f.service.DeleteTrack(context.Background(), gm, f.track.ID, &stale); !errors.Is(err, ErrTableUsageChanged) {
		t.Fatalf("stale count: %v", err)
	}
	if _, ok := f.store.tracks[f.track.ID]; !ok || len(f.audio.removed) != 0 {
		t.Fatal("track was deleted despite changed usage")
	}
	current := int64(1)
	if err := f.service.DeleteTrack(context.Background(), gm, f.track.ID, &current); err != nil {
		t.Fatalf("current count: %v", err)
	}
}

func TestDeleteTrackRefusesMissingOrProcessingTracks(t *testing.T) {
	f := newFixture(t.TempDir())
	if err := f.service.DeleteTrack(context.Background(), gm, uuid.New(), nil); !errors.Is(err, ErrTrackNotFound) {
		t.Fatalf("missing: %v", err)
	}
	processing := f.track
	processing.Status = "processing"
	f.store.tracks[processing.ID] = processing
	if err := f.service.DeleteTrack(context.Background(), gm, processing.ID, nil); !errors.Is(err, ErrTrackProcessing) {
		t.Fatalf("processing: %v", err)
	}
}

func TestDeleteTrackReportsFailures(t *testing.T) {
	f := newFixture(t.TempDir())
	f.store.fail = errStore
	if err := f.service.DeleteTrack(context.Background(), gm, f.track.ID, nil); !errors.Is(err, ErrDeleteRecordFailed) {
		t.Fatalf("record failure: %v", err)
	}
	f.store.fail = nil
	f.audio.fail = errAudio
	if err := f.service.DeleteTrack(context.Background(), gm, f.track.ID, nil); !errors.Is(err, ErrDeleteFilesFailed) {
		t.Fatalf("file failure: %v", err)
	}
	if f.lastEvent().name != events.TrackRemoved {
		t.Fatal("clients were not told the record is gone")
	}
}

func TestStreamFileResolvesTrackFile(t *testing.T) {
	f := newFixture(t.TempDir())
	f.store.tableTracks[f.table.ID] = []uuid.UUID{f.track.ID}
	segment := f.track.ID.String() + "/segment-1.ts"
	want := f.uploadDir + "/" + segment
	for name, request := range map[string]struct {
		viewer domain.Viewer
		link   string
	}{
		"GM library":   {viewer: gm},
		"GM table":     {viewer: gm, link: f.table.ExternalID},
		"player table": {viewer: f.player(), link: f.table.ExternalID},
	} {
		path, err := f.service.StreamFile(context.Background(), request.viewer, request.link, segment)
		if err != nil || path != want {
			t.Errorf("%s: %q %v", name, path, err)
		}
	}
}

func TestStreamFileRejectsUnsafeRequests(t *testing.T) {
	f := newFixture(t.TempDir())
	offTable := uuid.New()
	processing := domain.Track{ID: uuid.New(), Status: "processing"}
	f.store.tracks[processing.ID] = processing
	f.store.tableTracks[f.table.ID] = []uuid.UUID{processing.ID}
	tests := []struct {
		name   string
		viewer domain.Viewer
		link   string
		path   string
		want   error
	}{
		{name: "unknown table", viewer: gm, link: "missing", path: f.track.ID.String(), want: domain.ErrNotFound},
		{name: "other table", viewer: domain.PlayerViewer(uuid.New()), link: f.table.ExternalID, path: f.track.ID.String(), want: domain.ErrForbidden},
		{name: "player bad track ID", viewer: f.player(), link: f.table.ExternalID, path: "not-a-track/index.m3u8", want: ErrInvalidStreamTrack},
		{name: "player track off table", viewer: f.player(), link: f.table.ExternalID, path: offTable.String() + "/index.m3u8", want: ErrTrackNotFound},
		{name: "path escape", viewer: gm, path: "../secret", want: ErrInvalidStreamPath},
		{name: "not a track", viewer: gm, path: "notes.txt", want: ErrInvalidTrackPath},
		{name: "unknown track", viewer: gm, path: offTable.String() + "/index.m3u8", want: domain.ErrNotFound},
		{name: "processing track", viewer: f.player(), link: f.table.ExternalID, path: processing.ID.String() + "/index.m3u8", want: ErrTrackNotReady},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := f.service.StreamFile(context.Background(), tt.viewer, tt.link, tt.path); !errors.Is(err, tt.want) {
				t.Fatalf("stream: %v, want %v", err, tt.want)
			}
		})
	}
}

func TestWaveformGeneratesOnceThenUsesStoredCopy(t *testing.T) {
	f := newFixture(t.TempDir())
	f.audio.waveform = domain.Waveform{Peaks: []float64{0.5, 1}}
	for range 2 {
		waveform, err := f.service.Waveform(context.Background(), gm, f.track.ID)
		if err != nil || len(waveform.Peaks) != 2 {
			t.Fatalf("waveform: %+v %v", waveform, err)
		}
	}
	if f.audio.generated != 1 {
		t.Fatalf("generated %d times", f.audio.generated)
	}
}

func TestWaveformClassifiesFailures(t *testing.T) {
	f := newFixture(t.TempDir())
	ctx := context.Background()
	if _, err := f.service.Waveform(ctx, gm, uuid.New()); !errors.Is(err, ErrTrackNotFound) {
		t.Fatalf("missing: %v", err)
	}
	processing := domain.Track{ID: uuid.New(), Status: "processing"}
	f.store.tracks[processing.ID] = processing
	if _, err := f.service.Waveform(ctx, gm, processing.ID); !errors.Is(err, ErrTrackNotReady) {
		t.Fatalf("processing: %v", err)
	}
	f.audio.fail = errAudio
	if _, err := f.service.Waveform(ctx, gm, f.track.ID); !errors.Is(err, ErrWaveformFailed) || !errors.Is(err, errAudio) {
		t.Fatalf("generate failure: %v", err)
	}
	f.audio.fail = nil
	f.store.fail = errStore
	if _, err := f.service.Waveform(ctx, gm, f.track.ID); !errors.Is(err, ErrWaveformFailed) || !errors.Is(err, errStore) {
		t.Fatalf("save failure: %v", err)
	}
}

func TestTrackTypesHidesStoreFailure(t *testing.T) {
	f := newFixture(t.TempDir())
	types, err := f.service.TrackTypes(context.Background())
	if err != nil || len(types) != 1 || types[0].Name != "Music" {
		t.Fatalf("types: %+v %v", types, err)
	}
	f.store.fail = errStore
	if _, err := f.service.TrackTypes(context.Background()); !errors.Is(err, ErrTrackTypesFailed) {
		t.Fatalf("failure: %v", err)
	}
}

func TestLiveSyncFailureIsReportedAfterSaving(t *testing.T) {
	f := newFixture(t.TempDir())
	f.events.fail = errSend
	name := "Renamed"
	_, err := f.service.UpdateTrack(context.Background(), gm, f.track.ID, domain.TrackUpdate{Name: &name})
	if !errors.Is(err, domain.ErrLiveSyncFailed) || f.store.tracks[f.track.ID].Name != name {
		t.Fatalf("update: %v", err)
	}
}
