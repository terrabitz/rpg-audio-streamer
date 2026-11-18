package livemixer

import (
	"encoding/json"
	"errors"
	"sync"

	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
	"github.com/terrabitz/rpg-audio-streamer/internal/events"
)

type Controller interface {
	Update(tableID string, payload json.RawMessage) error
	Sync(tableID string, payload json.RawMessage) error
	Volumes(tableID string, payload json.RawMessage) error
	CloseTable(tableID string)
	RemoveTrack(tableID, trackID string)
}

type eventListener struct {
	bus   events.Bus
	mixer Controller
}

// Subscribe connects an independently owned mixer to the application's event bus.
func Subscribe(bus events.Bus, mixer Controller) func() {
	listener := &eventListener{bus: bus, mixer: mixer}
	stops := []func(){
		events.OnClientConnected(bus, listener.handleClientConnected),
		events.OnPlaybackChanged(bus, listener.handlePlaybackChanged),
		events.OnMixChanged(bus, listener.handleMixChanged),
		events.OnMixVolumesChanged(bus, listener.handleMixVolumesChanged),
		events.OnTrackRemoved(bus, listener.handleTrackRemoved),
		events.OnTableDeleted(bus, listener.handleTableDeleted),
		events.OnTableSessionEnded(bus, listener.handleTableSessionEnded),
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			for _, stop := range stops {
				stop()
			}
		})
	}
}

func (l *eventListener) handleClientConnected(_ events.Empty, metadata events.Metadata) error {
	if metadata.Role != auth.RoleGM {
		return nil
	}
	return l.bus.SendEvent(events.MixerReady, events.ClientEvent[struct{}]{
		TableID: metadata.TableID, ClientID: metadata.ClientID,
	})
}

func (l *eventListener) handlePlaybackChanged(changes events.PlaybackChanges[events.PlaybackPatch], metadata events.Metadata) error {
	for _, patch := range changes.Tracks {
		if err := applyMixer(metadata.TableID, patch, l.mixer.Update); err != nil {
			return l.result(metadata, events.TrackPlaybackChanged, err)
		}
	}
	return l.result(metadata, events.TrackPlaybackChanged, nil)
}

func (l *eventListener) handleMixChanged(payload events.MixSnapshot, metadata events.Metadata) error {
	return l.result(metadata, events.MixChanged, applyMixer(metadata.TableID, payload, l.mixer.Sync))
}

func (l *eventListener) handleMixVolumesChanged(payload events.MixVolumePatch, metadata events.Metadata) error {
	return l.result(metadata, events.MixVolumesChanged, applyMixer(metadata.TableID, payload, l.mixer.Volumes))
}

func (l *eventListener) handleTrackRemoved(track events.TrackReference, metadata events.Metadata) error {
	l.mixer.RemoveTrack(metadata.TableID, track.FileID)
	return nil
}

func (l *eventListener) handleTableDeleted(_ events.Empty, metadata events.Metadata) error {
	l.mixer.CloseTable(metadata.TableID)
	return nil
}

func (l *eventListener) handleTableSessionEnded(_ events.Empty, metadata events.Metadata) error {
	l.mixer.CloseTable(metadata.TableID)
	return nil
}

func applyMixer(tableID string, payload any, apply func(string, json.RawMessage) error) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return apply(tableID, raw)
}

func (l *eventListener) result(metadata events.Metadata, event string, applyErr error) error {
	if metadata.ClientID == "" {
		return applyErr
	}
	if applyErr != nil {
		err := l.bus.SendEvent(events.MixerFailed, events.ClientEvent[events.MixerFailure]{
			TableID: metadata.TableID, ClientID: metadata.ClientID,
			Data: events.MixerFailure{
				Event: event, Message: "Server mixing failed. Check the server logs and reconnect to retry.",
			},
		})
		return errors.Join(applyErr, err)
	}
	return l.bus.SendEvent(events.MixerApplied, events.ClientEvent[events.MixerResult]{
		TableID: metadata.TableID, ClientID: metadata.ClientID, Data: events.MixerResult{Event: event},
	})
}
