package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
	"github.com/terrabitz/rpg-audio-streamer/internal/discord"
	"github.com/terrabitz/rpg-audio-streamer/internal/events"
)

func (s *Server) handleDiscord(w http.ResponseWriter, r *http.Request, token *auth.Token) {
	id, ok := s.tableID(w, r, token)
	if !ok {
		return
	}
	if _, err := s.store.GetTable(r.Context(), id); err != nil {
		s.tableError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	if s.discord == nil {
		if r.Method == http.MethodGet {
			respondJSON(w, http.StatusOK, discord.Status{State: "disconnected"})
			return
		}
		http.Error(w, "Discord is not configured", http.StatusNotFound)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	var err error
	switch r.Method {
	case http.MethodGet:
	case http.MethodPost:
		var target *discord.Destination
		if r.ContentLength != 0 {
			var selection discord.Destination
			decoder := json.NewDecoder(io.LimitReader(r.Body, 64<<10))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&selection); err != nil {
				http.Error(w, "Select a Discord server and voice channel", http.StatusBadRequest)
				return
			}
			if err := decoder.Decode(new(any)); err != io.EOF {
				http.Error(w, "Invalid Discord selection", http.StatusBadRequest)
				return
			}
			if err := selection.Validate(); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			resolved, resolveErr := s.discord.Resolve(ctx, selection)
			if resolveErr != nil {
				s.logger.Warn("Discord destination validation failed", "error", resolveErr)
				http.Error(w, resolveErr.Error(), http.StatusBadGateway)
				return
			}
			target = &resolved
		} else {
			target, err = s.store.GetTableDiscord(ctx, id)
		}
		if err == nil && target == nil {
			http.Error(w, "Select a Discord server and voice channel first", http.StatusBadRequest)
			return
		}
		if err == nil {
			err = s.connectDiscord(ctx, id, *target)
		}
		if err == nil {
			err = s.store.SaveTableDiscord(ctx, id, *target)
			if err != nil {
				cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if disconnectErr := s.discord.Disconnect(cleanup, id.String()); disconnectErr != nil {
					s.logger.Error("Failed to disconnect unsaved Discord destination", "error", disconnectErr)
				}
				s.logger.Error("Failed to save Discord destination", "error", err)
				http.Error(w, "Unable to save Discord destination; connection cancelled", http.StatusInternalServerError)
				return
			}
		}
	case http.MethodDelete:
		err = s.discord.Disconnect(ctx, id.String())
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, discord.ErrNotReady) || errors.Is(err, discord.ErrInUse) {
			status = http.StatusConflict
		}
		s.logger.Warn("Discord command failed", "tableID", id, "error", err)
		http.Error(w, err.Error(), status)
		return
	}
	status := s.discord.Status(id.String())
	if status.Destination == nil {
		status.Destination, err = s.store.GetTableDiscord(ctx, id)
		if err != nil {
			s.tableError(w, err)
			return
		}
	}
	respondJSON(w, http.StatusOK, status)
}

func (s *Server) connectDiscord(ctx context.Context, tableID uuid.UUID, target discord.Destination) error {
	err := s.discord.Connect(ctx, tableID.String(), target)
	if !errors.Is(err, discord.ErrNotReady) || s.mixer == nil || s.playback == nil {
		return err
	}
	snapshot, err := s.playback.Snapshot(ctx, tableID)
	if err != nil {
		return err
	}
	mix, err := s.discordMixSnapshot(ctx, snapshot)
	if err != nil {
		return err
	}
	if err := s.hub.SendEvent(events.MixChanged, events.TableEvent[events.MixSnapshot]{
		TableID: tableID.String(),
		Data:    mix,
	}); err != nil {
		return err
	}
	return s.discord.Connect(ctx, tableID.String(), target)
}

func (s *Server) discordMixSnapshot(ctx context.Context, snapshot events.Snapshot) (events.MixSnapshot, error) {
	if snapshot.Settings == nil {
		return events.MixSnapshot{}, errors.New("playback snapshot is missing table settings")
	}
	types, err := s.store.GetTrackTypes(ctx)
	if err != nil {
		return events.MixSnapshot{}, err
	}
	settings := snapshot.Settings
	typeVolumes := make(map[string]float64, len(types))
	masterVolume := settings.MasterVolume / 100
	for _, trackType := range types {
		volume, ok := settings.TypeVolumes[trackType.Name]
		if !ok {
			volume = 100
		}
		typeVolumes[trackType.Name] = volume * masterVolume
	}
	tracks := make([]events.PlaybackPatch, 0, len(snapshot.Tracks))
	for _, track := range snapshot.Tracks {
		if !track.IsPlaying {
			continue
		}
		isPlaying, isRepeating := track.IsPlaying, track.IsRepeating
		volume, currentTime, duration := track.Volume, track.CurrentTime, track.Duration
		playback := track.Playback
		tracks = append(tracks, events.PlaybackPatch{
			FileID:      track.FileID,
			IsPlaying:   &isPlaying,
			IsRepeating: &isRepeating,
			Volume:      &volume,
			CurrentTime: &currentTime,
			Duration:    &duration,
			Playback:    &playback,
		})
	}
	return events.MixSnapshot{Tracks: tracks, TypeVolumes: typeVolumes}, nil
}

func (s *Server) handleDiscordSetup(w http.ResponseWriter, r *http.Request, token *auth.Token) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.discord == nil {
		http.Error(w, "Discord is not configured", http.StatusNotFound)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	if guildID := r.PathValue("guildID"); guildID != "" {
		channels, err := s.discord.Channels(ctx, guildID)
		if err != nil {
			s.logger.Warn("Discord channels request failed", "error", err)
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		respondJSON(w, http.StatusOK, channels)
		return
	}
	setup, err := s.discord.Setup(ctx)
	if err != nil {
		s.logger.Warn("Discord setup request failed", "error", err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	respondJSON(w, http.StatusOK, setup)
}
