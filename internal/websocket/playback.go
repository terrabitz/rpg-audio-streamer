package websocket

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/events"
	"github.com/terrabitz/rpg-audio-streamer/internal/server"
)

type PlaybackStore interface {
	GetTrackByID(context.Context, uuid.UUID) (server.Track, error)
	GetTrackTypeByID(context.Context, uuid.UUID) (server.TrackType, error)
	HasTableTrack(context.Context, uuid.UUID, uuid.UUID) (bool, error)
	GetTableSettings(context.Context, uuid.UUID) (server.TableSettings, error)
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
	Playback        server.PlaybackSettings `json:"playback"`
	PositionVersion uint64                  `json:"positionVersion"`
	SeekVersion     uint64                  `json:"seekVersion"`
	SeekTarget      *float64                `json:"seekTarget"`
	SeekLanding     *float64                `json:"seekLanding"`
	typeID          uuid.UUID
	updatedAt       time.Time
}

type PlaybackPatch = events.PlaybackPatch

func (h *Hub) handleTrackUpdated(fileID string, _ *Client) {
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	id, err := uuid.Parse(fileID)
	if err != nil {
		h.logger.Error("invalid updated track ID", "error", err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// Read after taking the playback lock so concurrent saves cannot broadcast stale metadata.
	track, err := h.store.GetTrackByID(ctx, id)
	if err != nil {
		h.logger.Error("failed to read updated track", "error", err)
		return
	}
	if err := h.publishEvent(events.TrackUpdated, track, events.Metadata{Global: true}); err != nil {
		h.logger.Error("failed to announce track update", "error", err)
	}
	for tableID, tracks := range h.playback {
		state, ok := tracks[fileID]
		if !ok {
			continue
		}
		settings, err := h.tableSettings(tableID)
		if err != nil {
			h.logger.Error("failed to synchronize updated track", "tableID", tableID, "error", err)
			continue
		}
		state = state.at(time.Now())
		state.Name = track.Name
		state.Playback = track.Playback
		state.CurrentTime = math.Max(state.CurrentTime, track.Playback.StartTime)
		state.clearInvalidSeek()
		tracks[fileID] = state
		h.sendState(tableID, settings, "")
		h.notify(events.PlaybackChanged, events.PlaybackChanges[PlaybackPatch]{
			Tracks: []PlaybackPatch{{FileID: fileID, Playback: &track.Playback}},
		}, events.Metadata{TableID: tableID})
	}
}

func (h *Hub) refreshPlayback() {
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	connected := make(map[string]bool)
	h.clientsMu.RLock()
	for client := range h.clients {
		connected[client.Token.TableID] = true
	}
	h.clientsMu.RUnlock()
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
		settings, err := h.tableSettings(tableID)
		if err != nil {
			h.logger.Error("failed to refresh playback", "error", err, "tableID", tableID)
			continue
		}
		h.sendState(tableID, settings, "")
	}
}

func (h *Hub) handleTableSettingsChanged(event events.TableEvent[server.TableSettingsPatch], _ *Client) {
	tableID, patch := event.TableID, event.Data
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	// Concurrent HTTP patches can finish broadcasting in a different order.
	settings, err := h.tableSettings(tableID)
	if err != nil {
		h.logger.Error("failed to synchronize table settings", "error", err, "tableID", tableID)
		return
	}
	changes := make([]PlaybackPatch, 0, len(patch.TrackVolumes))
	for id := range patch.TrackVolumes {
		if track, ok := h.playback[tableID][id]; ok {
			track.Volume = settings.TrackVolumes[id]
			h.playback[tableID][id] = track
			if track.IsPlaying {
				changes = append(changes, PlaybackPatch{FileID: id, Volume: &track.Volume})
			}
		}
	}
	h.sendState(tableID, settings, "")
	if len(changes) > 0 {
		h.notify(events.PlaybackChanged, events.PlaybackChanges[PlaybackPatch]{Tracks: changes}, events.Metadata{TableID: tableID})
	}
	h.notify(events.TableSettingsChanged, settings, events.Metadata{TableID: tableID})
}

func (h *Hub) updatePlayback(patch PlaybackPatch, c *Client) {
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	track, trackType, settings, err := h.resolvePlayback(c.Token.TableID, patch.FileID)
	if err != nil {
		h.playbackError(c, err)
		return
	}
	now := time.Now()
	states := h.playback[c.Token.TableID]
	if states == nil {
		states = make(map[string]playingTrack)
	}
	state, ok := states[patch.FileID]
	if ok {
		state = state.at(now)
	} else {
		volume := 100.0
		if saved, ok := settings.TrackVolumes[patch.FileID]; ok {
			volume = saved
		}
		state = playingTrack{FileID: patch.FileID, Volume: volume, Playback: track.Playback,
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
		h.playbackError(c, err)
		return
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
	h.playback[c.Token.TableID] = states
	h.sendState(c.Token.TableID, settings, "")
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
	h.notify(events.PlaybackChanged, events.PlaybackChanges[PlaybackPatch]{Tracks: changes}, c.metadata())
	h.notify(events.TrackPlaybackChanged, patch, c.metadata())
}

func (h *Hub) resolvePlayback(tableID, fileID string) (server.Track, server.TrackType, server.TableSettings, error) {
	var track server.Track
	var trackType server.TrackType
	var settings server.TableSettings
	table, err := uuid.Parse(tableID)
	if err != nil {
		return track, trackType, settings, err
	}
	id, err := uuid.Parse(fileID)
	if err != nil {
		return track, trackType, settings, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	associated, err := h.store.HasTableTrack(ctx, table, id)
	if err != nil {
		return track, trackType, settings, err
	}
	if !associated {
		return track, trackType, settings, fmt.Errorf("track is not associated with this table")
	}
	track, err = h.store.GetTrackByID(ctx, id)
	if err != nil {
		return track, trackType, settings, err
	}
	if !track.Ready() {
		return track, trackType, settings, fmt.Errorf("track is not ready for playback")
	}
	trackType, err = h.store.GetTrackTypeByID(ctx, track.TypeID)
	if err != nil {
		return track, trackType, settings, err
	}
	settings, err = h.store.GetTableSettings(ctx, table)
	return track, trackType, settings, err
}

func (h *Hub) tableSettings(tableID string) (server.TableSettings, error) {
	id, err := uuid.Parse(tableID)
	if err != nil {
		return server.TableSettings{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return h.store.GetTableSettings(ctx, id)
}

func (h *Hub) sendState(tableID string, settings server.TableSettings, target string) {
	now := time.Now()
	tracks := make([]events.PlaybackTrack, 0, len(h.playback[tableID]))
	for id, track := range h.playback[tableID] {
		track = track.at(now)
		h.playback[tableID][id] = track
		tracks = append(tracks, track.snapshot())
	}
	sort.Slice(tracks, func(i, j int) bool { return tracks[i].FileID < tracks[j].FileID })
	snapshot := Snapshot{Tracks: tracks, Settings: &settings}
	if err := h.publishEvent(events.PlaybackStateChanged, snapshot, events.Metadata{TableID: tableID, TargetClientID: target}); err != nil {
		h.logger.Error("failed to send playback state", "error", err)
	}
}

func (t playingTrack) snapshot() events.PlaybackTrack {
	return events.PlaybackTrack{
		FileID: t.FileID, Name: t.Name, IsPlaying: t.IsPlaying, Volume: t.Volume,
		IsRepeating: t.IsRepeating, CurrentTime: t.CurrentTime, Duration: t.Duration,
		TrackType: t.TrackType, Playback: t.Playback, PositionVersion: t.PositionVersion,
		SeekVersion: t.SeekVersion, SeekTarget: t.SeekTarget, SeekLanding: t.SeekLanding,
	}
}

func (h *Hub) playbackError(c *Client, err error) {
	h.logger.Warn("playback event failed", "error", err, "tableID", c.Token.TableID)
	if sendErr := c.sendEvent(events.PlaybackFailed, Failure{Message: "Unable to synchronize playback. Reload the table and try again."}); sendErr != nil {
		h.logger.Error("failed to send playback error", "error", sendErr)
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
