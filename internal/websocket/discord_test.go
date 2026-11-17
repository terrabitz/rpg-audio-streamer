package websocket

import (
	"encoding/json"
	"testing"

	"github.com/terrabitz/rpg-audio-streamer/internal/discord"
)

func TestDiscordStatusReachesOnlyTableGMs(t *testing.T) {
	hub, clients := testHub()
	for id, client := range clients {
		client.Token.TableID = "friday"
		if id == "otherGM" || id == "otherPlayer" {
			client.Token.TableID = "sunday"
		}
	}
	hub.DiscordStatusChanged("friday", discord.Status{Enabled: true, State: "connecting"})
	hub.DiscordStatusChanged("friday", discord.Status{
		Enabled: true, State: "error", Error: "voice lost",
		Destination: &discord.Destination{GuildID: "1", ChannelID: "2"},
	})
	select {
	case <-hub.discordChanged:
		hub.broadcastDiscordStatuses()
	default:
		t.Fatal("status update did not wake the hub")
	}
	for id, client := range clients {
		if id != "gm" {
			if len(client.send) != 0 {
				t.Fatalf("Discord status leaked to %s", id)
			}
			continue
		}
		select {
		case raw := <-client.send:
			var message Message
			if err := json.Unmarshal(raw, &message); err != nil {
				t.Fatal(err)
			}
			var status discord.Status
			if err := json.Unmarshal(message.Payload, &status); err != nil {
				t.Fatal(err)
			}
			if message.Method != DiscordStatus || status.State != "error" ||
				status.Error != "voice lost" || status.ChannelID != "2" {
				t.Fatalf("wrong status event: %s", raw)
			}
		default:
			t.Fatal("table GM did not receive status")
		}
	}
	hub.broadcastDiscordStatuses()
	if len(clients["gm"].send) != 0 {
		t.Fatal("pending updates were not drained")
	}
}
