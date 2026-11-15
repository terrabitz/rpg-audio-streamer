package discord

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestLiveVoiceConnection(t *testing.T) {
	if os.Getenv("DISCORD_LIVE_INTEGRATION") != "1" {
		t.Skip("set DISCORD_LIVE_INTEGRATION=1 to test a real Discord voice connection")
	}
	cfg := Config{
		Token:     os.Getenv("INTEGRATION_TEST_DISCORD_BOT_STREAMER_TOKEN"),
		GuildID:   os.Getenv("INTEGRATION_TEST_DISCORD_SERVER_ID"),
		ChannelID: os.Getenv("INTEGRATION_TEST_DISCORD_VOICE_CHANNEL_ID"),
	}
	if cfg.Token == "" || cfg.GuildID == "" || cfg.ChannelID == "" {
		t.Fatal("live Discord integration requires streamer token, server ID, and voice channel ID")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	conn, err := startVoice(ctx, cfg)
	if err != nil {
		t.Fatalf("connect Discord voice: %v", err)
	}
	cancel()
	select {
	case <-conn.done:
	case <-time.After(10 * time.Second):
		t.Fatal("Discord voice connection did not shut down")
	}
}
