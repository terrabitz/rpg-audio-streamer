package playback

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
	"github.com/terrabitz/rpg-audio-streamer/internal/events"
)

type Store interface {
	GetTrackByID(context.Context, uuid.UUID) (domain.Track, error)
	GetTrackTypeByID(context.Context, uuid.UUID) (domain.TrackType, error)
	HasTableTrack(context.Context, uuid.UUID, uuid.UUID) (bool, error)
	GetTableSettings(context.Context, uuid.UUID) (domain.TableSettings, error)
}

type Publisher interface {
	Publish(event string, payload any, metadata events.Metadata) error
}

type Service struct {
	stateMu  sync.Mutex
	playback map[string]map[string]playingTrack
	revision uint64
	store    Store
	output   Publisher
	logger   *slog.Logger
}

var (
	ErrTrackNotAssociated = domain.NotFound("track is not associated with this table")
	ErrTrackNotReady      = domain.Conflict("track is not ready for playback")
	ErrInvalidPatch       = domain.Invalid("invalid playback patch")
)

func New(logger *slog.Logger, store Store, output Publisher) *Service {
	return &Service{
		playback: make(map[string]map[string]playingTrack),
		store:    store,
		output:   output,
		logger:   logger,
	}
}

type playingTrack struct {
	FileID          string                  `json:"fileID"`
	Name            string                  `json:"name"`
	IsPlaying       bool                    `json:"isPlaying"`
	Volume          float64                 `json:"volume"`
	IsRepeating     bool                    `json:"isRepeating"`
	CurrentTime     float64                 `json:"currentTime"`
	Duration        float64                 `json:"duration"`
	TrackType       string                  `json:"trackType"`
	Playback        domain.PlaybackSettings `json:"playback"`
	PositionVersion uint64                  `json:"positionVersion"`
	SeekVersion     uint64                  `json:"seekVersion"`
	SeekTarget      *float64                `json:"seekTarget"`
	SeekLanding     *float64                `json:"seekLanding"`
	typeID          uuid.UUID
	updatedAt       time.Time
}

type PlaybackPatch = events.PlaybackPatch

func (h *Service) TrackUpdated(ctx context.Context, fileID string) error {
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	id, err := uuid.Parse(fileID)
	if err != nil {
		return err
	}
	// Read after taking the playback lock so concurrent saves cannot broadcast stale metadata.
	track, err := h.store.GetTrackByID(ctx, id)
	if err != nil {
		return err
	}
	h.publish(events.TrackUpdated, track, events.Metadata{Global: true})
	for tableID, tracks := range h.playback {
		state, ok := tracks[fileID]
		if !ok {
			continue
		}
		settings, err := h.tableSettings(ctx, tableID)
		if err != nil {
			h.logger.Error("failed to synchronize updated track", "tableID", tableID, "error", err)
			continue
		}
		state = state.at(time.Now())
		state.Name = track.Name
		state.Playback = track.Playback
		state.Volume = track.Volume
		state.CurrentTime = math.Max(state.CurrentTime, track.Playback.StartTime)
		state.clearInvalidSeek()
		tracks[fileID] = state
		h.sendState(tableID, settings)
		h.publish(events.PlaybackChanged, events.PlaybackChanges[PlaybackPatch]{
			Tracks: []PlaybackPatch{{FileID: fileID, Playback: &track.Playback, Volume: &state.Volume}},
		}, events.Metadata{TableID: tableID})
	}
	return nil
}

func (h *Service) Refresh(ctx context.Context, connected map[string]bool) {
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	for tableID, tracks := range h.playback {
		if !connected[tableID] {
			continue
		}
		active := false
		for _, track := range tracks {
			active = active || track.IsPlaying
		}
		if !active {
			continue
		}
		settings, err := h.tableSettings(ctx, tableID)
		if err != nil {
			h.logger.Error("failed to refresh playback", "error", err, "tableID", tableID)
			continue
		}
		h.sendState(tableID, settings)
	}
}

func (h *Service) SettingsChanged(ctx context.Context, tableID string) error {
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	// Concurrent HTTP patches can finish broadcasting in a different order.
	settings, err := h.tableSettings(ctx, tableID)
	if err != nil {
		return err
	}
	h.sendState(tableID, settings)
	h.publish(events.TableSettingsChanged, settings, events.Metadata{TableID: tableID})
	return nil
}

func (h *Service) DeleteTable(tableID string) {
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	delete(h.playback, tableID)
}

func (h *Service) RemoveTrack(tableID, trackID string) {
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	if tableID != "" {
		delete(h.playback[tableID], trackID)
		return
	}
	for _, tracks := range h.playback {
		delete(tracks, trackID)
	}
}

func (h *Service) Snapshot(ctx context.Context, tableID uuid.UUID) (events.Snapshot, error) {
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	settings, err := h.store.GetTableSettings(ctx, tableID)
	if err != nil {
		return events.Snapshot{}, err
	}
	return h.snapshot(tableID.String(), settings, false), nil
}

func (h *Service) Update(ctx context.Context, tableID, trackID uuid.UUID, patch PlaybackPatch) error {
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	track, trackType, settings, err := h.resolvePlayback(ctx, tableID, trackID)
	if err != nil {
		return err
	}
	patch.FileID = trackID.String()
	now := time.Now()
	states := h.playback[tableID.String()]
	if states == nil {
		states = make(map[string]playingTrack)
	}
	state, ok := states[patch.FileID]
	if ok {
		state = state.at(now)
	} else {
		state = playingTrack{FileID: patch.FileID, Volume: track.Volume, Playback: track.Playback,
			CurrentTime: track.Playback.StartTime}
	}
	state.Name, state.TrackType, state.typeID = track.Name, trackType.Name, track.TypeID
	outgoing, wasPlaying := state.CurrentTime, state.IsPlaying
	state.IsRepeating = trackType.IsRepeating
	if patch.Duration != nil {
		state.Duration = *patch.Duration
	}
	if patch.Playback != nil {
		state.Playback = *patch.Playback
	}
	if patch.CurrentTime != nil {
		state.CurrentTime = *patch.CurrentTime
		state.SeekVersion++
	}
	if patch.Volume != nil {
		state.Volume = *patch.Volume
	}
	if patch.IsPlaying != nil {
		state.IsPlaying = *patch.IsPlaying
		if !state.IsPlaying && patch.CurrentTime == nil {
			state.SeekTarget = nil
			state.SeekLanding = nil
		}
	}
	if patch.CurrentTime != nil || patch.IsPlaying != nil {
		state.PositionVersion++
	}
	if err := state.validate(); err != nil {
		return ErrInvalidPatch.Detail(err.Error())
	}
	end := state.end()
	if patch.CurrentTime == nil {
		state.clearInvalidSeek()
	}
	state.CurrentTime = math.Max(state.Playback.StartTime, state.CurrentTime)
	if patch.CurrentTime != nil {
		target := state.CurrentTime
		if end > 0 {
			target = math.Min(target, math.Max(state.Playback.StartTime, end-2))
		}
		state.SeekTarget = &target
		state.CurrentTime = target
		landing := target
		if wasPlaying && state.IsPlaying {
			duration := math.Max(0, math.Min(2, math.Min(end-outgoing, end-state.Playback.StartTime)))
			if duration > 0 {
				if target-state.Playback.StartTime <= 2 {
					state.CurrentTime = state.Playback.StartTime
				} else {
					state.CurrentTime = target - duration
				}
				landing = state.CurrentTime + duration
			}
		}
		state.SeekLanding = &landing
	}
	if end > 0 {
		state.CurrentTime = math.Min(state.CurrentTime, end)
		if state.IsPlaying && state.CurrentTime >= end {
			state.CurrentTime = state.Playback.StartTime
			if state.IsRepeating && state.Playback.LoopBackTime != nil {
				state.CurrentTime = *state.Playback.LoopBackTime
			}
		}
	}
	state.updatedAt = now
	var stopped []string
	if state.IsPlaying && !trackType.AllowSimultaneousPlay {
		for id, other := range states {
			if id != state.FileID && other.typeID == state.typeID && other.IsPlaying {
				other = other.at(now)
				other.IsPlaying = false
				states[id] = other
				stopped = append(stopped, id)
			}
		}
	}
	states[state.FileID] = state
	h.playback[tableID.String()] = states
	h.sendState(tableID.String(), settings)
	changes := make([]PlaybackPatch, 0, len(stopped)+1)
	paused := false
	for _, id := range stopped {
		changes = append(changes, PlaybackPatch{FileID: id, IsPlaying: &paused})
	}
	if patch.IsPlaying != nil && *patch.IsPlaying {
		patch.Volume = &state.Volume
		patch.Playback = &state.Playback
	}
	changes = append(changes, patch)
	h.publish(events.PlaybackChanged, events.PlaybackChanges[PlaybackPatch]{Tracks: changes}, events.Metadata{TableID: tableID.String()})
	return nil
}

func (h *Service) resolvePlayback(ctx context.Context, tableID, trackID uuid.UUID) (domain.Track, domain.TrackType, domain.TableSettings, error) {
	var track domain.Track
	var trackType domain.TrackType
	var settings domain.TableSettings
	associated, err := h.store.HasTableTrack(ctx, tableID, trackID)
	if err != nil {
		return track, trackType, settings, err
	}
	if !associated {
		return track, trackType, settings, ErrTrackNotAssociated
	}
	track, err = h.store.GetTrackByID(ctx, trackID)
	if err != nil {
		return track, trackType, settings, err
	}
	if !track.Ready() {
		return track, trackType, settings, ErrTrackNotReady
	}
	trackType, err = h.store.GetTrackTypeByID(ctx, track.TypeID)
	if err != nil {
		return track, trackType, settings, err
	}
	settings, err = h.store.GetTableSettings(ctx, tableID)
	return track, trackType, settings, err
}

func (h *Service) tableSettings(ctx context.Context, tableID string) (domain.TableSettings, error) {
	id, err := uuid.Parse(tableID)
	if err != nil {
		return domain.TableSettings{}, err
	}
	return h.store.GetTableSettings(ctx, id)
}

func (h *Service) sendState(tableID string, settings domain.TableSettings) {
	h.publish(events.PlaybackStateChanged, h.snapshot(tableID, settings, true), events.Metadata{TableID: tableID})
}

func (h *Service) publish(event string, payload any, metadata events.Metadata) {
	if err := h.output.Publish(event, payload, metadata); err != nil {
		h.logger.Error("failed to publish playback event", "event", event, "tableID", metadata.TableID, "error", err)
	}
}

func (h *Service) snapshot(tableID string, settings domain.TableSettings, advance bool) events.Snapshot {
	now := time.Now()
	tracks := make([]events.PlaybackTrack, 0, len(h.playback[tableID]))
	for id, track := range h.playback[tableID] {
		track = track.at(now)
		if advance {
			h.playback[tableID][id] = track
		}
		tracks = append(tracks, track.snapshot())
	}
	sort.Slice(tracks, func(i, j int) bool { return tracks[i].FileID < tracks[j].FileID })
	h.revision++
	return events.Snapshot{Revision: h.revision, Tracks: tracks, Settings: &settings}
}

func (t playingTrack) snapshot() events.PlaybackTrack {
	return events.PlaybackTrack{
		FileID: t.FileID, Name: t.Name, IsPlaying: t.IsPlaying, Volume: t.Volume,
		IsRepeating: t.IsRepeating, CurrentTime: t.CurrentTime, Duration: t.Duration,
		TrackType: t.TrackType, Playback: t.Playback, PositionVersion: t.PositionVersion,
		SeekVersion: t.SeekVersion, SeekTarget: t.SeekTarget, SeekLanding: t.SeekLanding,
	}
}

func (t playingTrack) validate() error {
	if t.Volume < 0 || t.Volume > 100 || t.CurrentTime < 0 || t.Duration < 0 {
		return fmt.Errorf("invalid playback volume or time")
	}
	if err := t.Playback.Validate(); err != nil {
		return err
	}
	if t.Duration > 0 && (t.Playback.StartTime >= t.Duration || t.end() > t.Duration) {
		return fmt.Errorf("playback region is outside the track duration")
	}
	if t.end() > 0 && t.Playback.LoopBackTime != nil && *t.Playback.LoopBackTime >= t.end() {
		return fmt.Errorf("loop-back point is outside the track duration")
	}
	return nil
}

func (t playingTrack) at(now time.Time) playingTrack {
	if t.IsPlaying {
		t.CurrentTime += now.Sub(t.updatedAt).Seconds()
		if t.SeekLanding != nil && t.CurrentTime >= *t.SeekLanding {
			t.SeekTarget = nil
			t.SeekLanding = nil
		}
		end := t.end()
		if end > 0 && t.CurrentTime >= end {
			if !t.IsRepeating {
				t.IsPlaying = false
				t.CurrentTime = t.Playback.StartTime
			} else {
				loop := t.Playback.StartTime
				if t.Playback.LoopBackTime != nil {
					loop = *t.Playback.LoopBackTime
				}
				fade := math.Min(2, math.Min((end-loop)/3, end-t.Playback.StartTime))
				period := end - loop - fade
				t.CurrentTime = loop + fade + math.Mod(t.CurrentTime-end, period)
			}
		}
	}
	t.updatedAt = now
	return t
}

func (t *playingTrack) clearInvalidSeek() {
	end := t.end()
	if t.SeekTarget != nil && (*t.SeekTarget < t.Playback.StartTime ||
		end > 0 && (*t.SeekTarget > math.Max(t.Playback.StartTime, end-2) ||
			t.SeekLanding != nil && *t.SeekLanding > end)) {
		t.SeekTarget, t.SeekLanding = nil, nil
	}
}

func (t playingTrack) end() float64 {
	if t.Playback.EndTime != nil {
		return *t.Playback.EndTime
	}
	return t.Duration
}
