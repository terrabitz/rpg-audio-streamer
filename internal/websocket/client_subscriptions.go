package websocket

import (
	"errors"

	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
	"github.com/terrabitz/rpg-audio-streamer/internal/events"
)

func (c *Client) subscribe() {
	c.subscriptions = []func(){
		events.OnPlaybackStateChanged(c.hub, c.handlePlaybackStateChanged),
		events.OnTrackRemoved(c.hub, c.handleTrackRemoved),
		events.OnTableDeleted(c.hub, c.handleTableDeleted),
	}
	if c.Token.Role == auth.RoleGM {
		c.subscriptions = append(c.subscriptions,
			events.OnTrackStatusChanged(c.hub, c.handleTrackStatusChanged),
			events.OnTrackUpdated(c.hub, c.handleTrackUpdated),
			events.OnTagUpdated(c.hub, c.handleTagUpdated),
			events.OnTagDeleted(c.hub, c.handleTagDeleted),
			events.OnDiscordStatusChanged(c.hub, c.handleDiscordStatusChanged),
			events.OnMixerReady(c.hub, c.handleMixerReady),
			events.OnMixerApplied(c.hub, c.handleMixerApplied),
			events.OnMixerFailed(c.hub, c.handleMixerFailed),
		)
	}
}

func (c *Client) handlePlaybackStateChanged(payload events.Snapshot, metadata events.Metadata) error {
	if !c.accepts(metadata) {
		return nil
	}
	if c.Token.Role == auth.RolePlayer {
		payload = payload.ForPlayer()
	}
	return c.deliver(events.PlaybackStateChanged, payload, metadata)
}

func (c *Client) handleTrackRemoved(payload events.TrackReference, metadata events.Metadata) error {
	return c.deliver(events.TrackRemoved, payload, metadata)
}

func (c *Client) handleTableDeleted(payload events.Empty, metadata events.Metadata) error {
	return c.deliver(events.TableDeleted, payload, metadata)
}

func (c *Client) handleTrackStatusChanged(payload events.TrackStatusUpdate, metadata events.Metadata) error {
	return c.deliver(events.TrackStatusChanged, payload, metadata)
}

func (c *Client) handleTrackUpdated(payload domain.Track, metadata events.Metadata) error {
	return c.deliver(events.TrackUpdated, payload, metadata)
}

func (c *Client) handleTagUpdated(payload domain.Tag, metadata events.Metadata) error {
	return c.deliver(events.TagUpdated, payload, metadata)
}

func (c *Client) handleTagDeleted(payload events.TagReference, metadata events.Metadata) error {
	return c.deliver(events.TagDeleted, payload, metadata)
}

func (c *Client) handleDiscordStatusChanged(payload domain.DiscordStatus, metadata events.Metadata) error {
	return c.deliver(events.DiscordStatusChanged, payload, metadata)
}

func (c *Client) handleMixerReady(payload events.Empty, metadata events.Metadata) error {
	return c.deliver(events.MixerReady, payload, metadata)
}

func (c *Client) handleMixerApplied(payload events.MixerResult, metadata events.Metadata) error {
	return c.deliver(events.MixerApplied, payload, metadata)
}

func (c *Client) handleMixerFailed(payload events.MixerFailure, metadata events.Metadata) error {
	return c.deliver(events.MixerFailed, payload, metadata)
}

func (c *Client) deliver(event string, payload any, metadata events.Metadata) error {
	if !c.accepts(metadata) {
		return nil
	}
	message, err := newMessage(event, payload)
	if err != nil {
		return err
	}
	if err := c.Send(message); err != nil {
		if errors.Is(err, ErrClientQueueFull) {
			c.hub.dropClient(c)
		}
		return err
	}
	return nil
}

func (c *Client) accepts(metadata events.Metadata) bool {
	return (metadata.Global || metadata.TableID == c.Token.TableID) &&
		(metadata.TargetClientID == "" || metadata.TargetClientID == c.ID)
}
