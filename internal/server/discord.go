package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
	"github.com/terrabitz/rpg-audio-streamer/internal/discord"
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
	if s.discordMix != nil && (r.Method == http.MethodPost || r.Method == http.MethodDelete) {
		s.discordMix.cancelRestore(id.String())
	}
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
			err = s.discord.Connect(ctx, id.String(), *target)
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
