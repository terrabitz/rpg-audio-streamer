package voice

import (
	"context"
	"errors"

	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
	"github.com/terrabitz/rpg-audio-streamer/internal/events"
)

// Connect starts streaming a table to a Discord voice channel.
//
// A nil selection reconnects to the table's saved channel. A new selection is
// saved only after the bot has joined it, so a bad choice never replaces a
// working one.
func (s *Service) Connect(ctx context.Context, viewer domain.Viewer, externalID string, selection *domain.Destination) (domain.DiscordStatus, error) {
	table, err := s.resolveTable(ctx, viewer, externalID)
	if err != nil {
		return domain.DiscordStatus{}, err
	}
	if !s.configured() {
		return domain.DiscordStatus{}, ErrNotConfigured
	}
	ctx, cancel := context.WithTimeout(ctx, tableTimeout)
	defer cancel()

	destination, err := s.chooseDestination(ctx, table, selection)
	if err != nil {
		return domain.DiscordStatus{}, err
	}
	if err := s.join(ctx, table, destination); err != nil {
		return domain.DiscordStatus{}, upstreamError(err)
	}
	if err := s.store.SaveTableDiscord(ctx, table.ID, destination); err != nil {
		s.abandon(table)
		return domain.DiscordStatus{}, ErrSaveFailed.WithCause(err)
	}
	return s.currentStatus(ctx, table)
}

func (s *Service) chooseDestination(ctx context.Context, table domain.Table, selection *domain.Destination) (domain.Destination, error) {
	if selection == nil {
		return s.savedDestination(ctx, table)
	}
	if err := selection.Validate(); err != nil {
		return domain.Destination{}, domain.Invalid(err.Error())
	}
	resolved, err := s.discord.Resolve(ctx, *selection)
	if err != nil {
		return domain.Destination{}, upstreamError(err)
	}
	return resolved, nil
}

func (s *Service) savedDestination(ctx context.Context, table domain.Table) (domain.Destination, error) {
	saved, err := s.store.GetTableDiscord(ctx, table.ID)
	if err != nil {
		return domain.Destination{}, err
	}
	if saved == nil {
		return domain.Destination{}, ErrNoDestination
	}
	return *saved, nil
}

// join connects the bot, rebuilding the table's live mix first when the
// mixer has forgotten it, for example after a server restart.
func (s *Service) join(ctx context.Context, table domain.Table, destination domain.Destination) error {
	err := s.discord.Connect(ctx, table.ID.String(), destination)
	if !errors.Is(err, domain.ErrTableMixNotReady) {
		return err
	}
	if err := s.restoreMix(ctx, table); err != nil {
		return err
	}
	return s.discord.Connect(ctx, table.ID.String(), destination)
}

func (s *Service) restoreMix(ctx context.Context, table domain.Table) error {
	snapshot, err := s.playback.Snapshot(ctx, table.ID)
	if err != nil {
		return err
	}
	types, err := s.store.GetTrackTypes(ctx)
	if err != nil {
		return err
	}
	mix, err := liveMix(snapshot, types)
	if err != nil {
		return err
	}
	err = s.events.SendEvent(events.MixChanged, events.TableEvent[events.MixSnapshot]{
		TableID: table.ID.String(),
		Data:    mix,
	})
	if err != nil {
		return ErrMixRestoreFailed.WithCause(err)
	}
	return nil
}

// liveMix rebuilds the mixer's view of a table: only the tracks that are
// playing, with the GM's master volume folded into each type's volume.
func liveMix(snapshot events.Snapshot, types []domain.TrackType) (events.MixSnapshot, error) {
	if snapshot.Settings == nil {
		return events.MixSnapshot{}, ErrMissingSettings
	}
	settings := snapshot.Settings
	masterVolume := settings.MasterVolume / 100
	typeVolumes := make(map[string]float64, len(types))
	for _, trackType := range types {
		volume, ok := settings.TypeVolumes[trackType.Name]
		if !ok {
			volume = 100
		}
		typeVolumes[trackType.Name] = volume * masterVolume
	}
	tracks := make([]events.PlaybackPatch, 0, len(snapshot.Tracks))
	for _, track := range snapshot.Tracks {
		if track.IsPlaying {
			tracks = append(tracks, playingTrack(track))
		}
	}
	return events.MixSnapshot{
		Tracks:      tracks,
		TypeVolumes: typeVolumes,
	}, nil
}

func playingTrack(track events.PlaybackTrack) events.PlaybackPatch {
	return events.PlaybackPatch{
		FileID:      track.FileID,
		IsPlaying:   &track.IsPlaying,
		IsRepeating: &track.IsRepeating,
		Volume:      &track.Volume,
		CurrentTime: &track.CurrentTime,
		Duration:    &track.Duration,
		Playback:    &track.Playback,
	}
}

// abandon leaves a channel the GM never saw saved, even if the request was cancelled.
func (s *Service) abandon(table domain.Table) {
	ctx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
	defer cancel()
	if err := s.discord.Disconnect(ctx, table.ID.String()); err != nil {
		s.logger.Error("failed to disconnect unsaved Discord destination", "tableID", table.ID, "error", err)
	}
}
