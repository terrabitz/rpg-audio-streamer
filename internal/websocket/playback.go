package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
	"github.com/terrabitz/rpg-audio-streamer/internal/server"
)

type PlaybackStore interface {
	GetTrackByID(context.Context, uuid.UUID) (server.Track, error)
	GetTrackTypeByID(context.Context, uuid.UUID) (server.TrackType, error)
	HasTableTrack(context.Context, uuid.UUID, uuid.UUID) (bool, error)
	GetTableSettings(context.Context, uuid.UUID) (server.TableSettings, error)
}

type playingTrack struct {
	FileID      string                  `json:"fileID"`
	Name        string                  `json:"name"`
	IsPlaying   bool                    `json:"isPlaying"`
	Volume      float64                 `json:"volume"`
	IsRepeating bool                    `json:"isRepeating"`
	CurrentTime float64                 `json:"currentTime"`
	Duration    float64                 `json:"duration"`
	TrackType   string                  `json:"trackType"`
	Playback    server.PlaybackSettings `json:"playback"`
	typeID      uuid.UUID
	updatedAt   time.Time
}

type playbackPatch struct {
	FileID      string                   `json:"fileID"`
	IsPlaying   *bool                    `json:"isPlaying"`
	Volume      *float64                 `json:"volume"`
	CurrentTime *float64                 `json:"currentTime"`
	Duration    *float64                 `json:"duration"`
	Playback    *server.PlaybackSettings `json:"playback"`
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

func (h *Hub) SyncSettings(tableID string, patch server.TableSettingsPatch) {
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	// Concurrent HTTP patches can finish broadcasting in a different order.
	settings, err := h.tableSettings(tableID)
	if err != nil {
		h.logger.Error("failed to synchronize table settings", "error", err, "tableID", tableID)
		return
	}
	for id := range patch.TrackVolumes {
		if track, ok := h.playback[tableID][id]; ok {
			track.Volume = settings.TrackVolumes[id]
			h.playback[tableID][id] = track
		}
	}
	h.sendState(tableID, settings, "")
}

func (h *Hub) updatePlayback(payload json.RawMessage, c *Client) {
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	var patch playbackPatch
	if err := json.Unmarshal(payload, &patch); err != nil {
		h.playbackError(c, fmt.Errorf("invalid playback command: %w", err))
		return
	}
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
	state.IsRepeating = trackType.IsRepeating
	if patch.Duration != nil {
		state.Duration = *patch.Duration
	}
	if patch.Playback != nil {
		state.Playback = *patch.Playback
	}
	if patch.CurrentTime != nil {
		state.CurrentTime = *patch.CurrentTime
	}
	if patch.Volume != nil {
		state.Volume = *patch.Volume
	}
	if patch.IsPlaying != nil {
		state.IsPlaying = *patch.IsPlaying
	}
	if err := state.validate(); err != nil {
		h.playbackError(c, err)
		return
	}
	end := state.end()
	state.CurrentTime = math.Max(state.Playback.StartTime, state.CurrentTime)
	if end > 0 {
		state.CurrentTime = math.Min(state.CurrentTime, end)
		if state.IsPlaying && state.CurrentTime >= end {
			state.CurrentTime = state.Playback.StartTime
		}
	}
	state.updatedAt = now
	if state.IsPlaying && !trackType.AllowSimultaneousPlay {
		for id, other := range states {
			if id != state.FileID && other.typeID == state.typeID {
				other = other.at(now)
				other.IsPlaying = false
				states[id] = other
			}
		}
	}
	states[state.FileID] = state
	h.playback[c.Token.TableID] = states
	h.sendState(c.Token.TableID, settings, "")
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
	tracks := make([]playingTrack, 0, len(h.playback[tableID]))
	for id, track := range h.playback[tableID] {
		track = track.at(now)
		h.playback[tableID][id] = track
		tracks = append(tracks, track)
	}
	sort.Slice(tracks, func(i, j int) bool { return tracks[i].FileID < tracks[j].FileID })
	for _, role := range []auth.Role{auth.RoleGM, auth.RolePlayer} {
		outgoing := append([]playingTrack{}, tracks...)
		if role == auth.RolePlayer {
			for i := range outgoing {
				outgoing[i].Volume *= settings.MasterVolume / 100
			}
		}
		snapshot := struct {
			Tracks   []playingTrack        `json:"tracks"`
			Settings *server.TableSettings `json:"settings,omitempty"`
		}{Tracks: outgoing}
		if role == auth.RoleGM {
			snapshot.Settings = &settings
		}
		payload, err := json.Marshal(snapshot)
		if err != nil {
			h.logger.Error("failed to encode playback state", "error", err)
			return
		}
		options := []BroadcastOption{ToTable(tableID), func(c *Client) bool { return c.Token.Role == role }}
		if target != "" {
			options = append(options, ToClientID(target))
		}
		if err := h.Broadcast(Message{Method: "syncAll", Payload: payload}, options...); err != nil {
			h.logger.Error("failed to send playback state", "error", err)
		}
	}
}

func (h *Hub) playbackError(c *Client, err error) {
	h.logger.Warn("playback command failed", "error", err, "tableID", c.Token.TableID)
	payload, _ := json.Marshal(struct {
		Message string `json:"message"`
	}{Message: "Unable to synchronize playback. Reload the table and try again."})
	if sendErr := c.Send(Message{Method: "playbackError", Payload: payload}); sendErr != nil {
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

func (t playingTrack) end() float64 {
	if t.Playback.EndTime != nil {
		return *t.Playback.EndTime
	}
	return t.Duration
}
