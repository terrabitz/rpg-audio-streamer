package events

import (
	"encoding/json"
	"fmt"

	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
)

type Listener[T any] func(T, Metadata) error

func OnPlaybackStateChanged(bus Bus, handler Listener[Snapshot]) func() {
	return subscribe(bus, PlaybackStateChanged, handler)
}

func OnTrackRemoved(bus Bus, handler Listener[TrackReference]) func() {
	return subscribe(bus, TrackRemoved, handler)
}

func OnTableDeleted(bus Bus, handler Listener[Empty]) func() {
	return subscribe(bus, TableDeleted, handler)
}

func OnMixChanged(bus Bus, handler Listener[MixSnapshot]) func() {
	return subscribe(bus, MixChanged, handler)
}

func OnMixVolumesChanged(bus Bus, handler Listener[MixVolumePatch]) func() {
	return subscribe(bus, MixVolumesChanged, handler)
}

func OnMixerReady(bus Bus, handler Listener[Empty]) func() {
	return subscribe(bus, MixerReady, handler)
}

func OnMixerApplied(bus Bus, handler Listener[MixerResult]) func() {
	return subscribe(bus, MixerApplied, handler)
}

func OnMixerFailed(bus Bus, handler Listener[MixerFailure]) func() {
	return subscribe(bus, MixerFailed, handler)
}

func OnTrackStatusChanged(bus Bus, handler Listener[TrackStatusUpdate]) func() {
	return subscribe(bus, TrackStatusChanged, handler)
}

func OnTrackUpdated(bus Bus, handler Listener[domain.Track]) func() {
	return subscribe(bus, TrackUpdated, handler)
}

func OnDiscordStatusChanged(bus Bus, handler Listener[domain.DiscordStatus]) func() {
	return subscribe(bus, DiscordStatusChanged, handler)
}

func OnTableSettingsChanged(bus Bus, handler Listener[domain.TableSettings]) func() {
	return subscribe(bus, TableSettingsChanged, handler)
}

func OnClientConnected(bus Bus, handler Listener[Empty]) func() {
	return subscribe(bus, ClientConnected, handler)
}

func OnTableSessionEnded(bus Bus, handler Listener[Empty]) func() {
	return subscribe(bus, TableSessionEnded, handler)
}

func OnPlaybackChanged(bus Bus, handler Listener[PlaybackChanges[PlaybackPatch]]) func() {
	return subscribe(bus, PlaybackChanged, handler)
}

func subscribe[T any](bus Bus, event string, handler Listener[T]) func() {
	return bus.On(event, func(raw json.RawMessage, metadata Metadata) error {
		var payload T
		if err := json.Unmarshal(raw, &payload); err != nil {
			return fmt.Errorf("decode %s: %w", event, err)
		}
		return handler(payload, metadata)
	})
}
