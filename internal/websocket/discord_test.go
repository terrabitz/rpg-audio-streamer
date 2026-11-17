package websocket

import (
	"encoding/json"
	"testing"

	"github.com/terrabitz/rpg-audio-streamer/internal/discord"
	"github.com/terrabitz/rpg-audio-streamer/internal/events"
)

func TestDiscordStatusReachesOnlyTableGMs(t *testing.T) {
	hub, clients := testHub()
	for id, client := range clients {
		client.Token.TableID = "friday"
		if id == "otherGM" || id == "otherPlayer" {
			client.Token.TableID = "sunday"
		}
	}
	sendAndDrain(t, hub, events.DiscordStatusChanged, events.TableEvent[discord.Status]{TableID: "friday", Data: discord.Status{Enabled: true, State: "connecting"}})
	<-clients["gm"].send
	sendAndDrain(t, hub, events.DiscordStatusChanged, events.TableEvent[discord.Status]{TableID: "friday", Data: discord.Status{
		Enabled: true, State: "error", Error: "voice lost",
		Destination: &discord.Destination{GuildID: "1", ChannelID: "2"},
	}})
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
			if message.Event != events.DiscordStatusChanged || status.State != "error" ||
				status.Error != "voice lost" || status.ChannelID != "2" {
				t.Fatalf("wrong status event: %s", raw)
			}
		default:
			t.Fatal("table GM did not receive status")
		}
	}
	drainEvents(hub)
	if len(clients["gm"].send) != 0 {
		t.Fatal("pending updates were not drained")
	}
}
