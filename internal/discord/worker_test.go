package discord

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestWorkerHandshakeFailureAndCancellationUseRealChildProcesses(t *testing.T) {
	for _, channel := range []string{"1", "2", "3"} {
		t.Run(channel, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			process, err := startWorker(ctx, Config{
				Token: "public-test-token", GuildID: "1", ChannelID: channel,
				Worker: "testdata/worker.mjs",
			})
			if (err == nil) != (channel == "1") {
				t.Fatalf("handshake error: %v", err)
			}
			cancel()
			if process != nil {
				process.input.Close()
				select {
				case <-process.done:
				case <-time.After(3 * time.Second):
					t.Fatal("Discord worker survived cancellation")
				}
			}
		})
	}
}

func TestWorkerEnvironmentDoesNotLeakListenerOrOAuthCredentials(t *testing.T) {
	t.Setenv("INTEGRATION_TEST_DISCORD_BOT_LISTENER_TOKEN", "test-listener")
	t.Setenv("DISCORD_CLIENT_SECRET", "test-secret")
	env := workerEnv(Config{Token: "test-streamer", GuildID: "1", ChannelID: "2"})
	for _, entry := range env {
		if strings.Contains(entry, "test-listener") || strings.Contains(entry, "test-secret") {
			t.Fatal("unrelated Discord credentials reached the voice worker")
		}
	}
}
