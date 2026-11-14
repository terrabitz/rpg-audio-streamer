package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadLiveConfigMergesEnvFileAndEnvironment(t *testing.T) {
	file := writeEnv(t, `# private test server
INTEGRATION_TEST_DISCORD_SERVER_ID=11
INTEGRATION_TEST_DISCORD_VOICE_CHANNEL_ID="12"
INTEGRATION_TEST_DISCORD_BOT_STREAMER_TOKEN='file-streamer'
INTEGRATION_TEST_DISCORD_BOT_STREAMER_CLIENT_ID = 13

INTEGRATION_TEST_DISCORD_BOT_LISTENER_TOKEN=file-listener
`)
	config, err := loadLiveConfig(file, lookup(map[string]string{
		"INTEGRATION_TEST_DISCORD_BOT_LISTENER_TOKEN": "env-listener",
	}))
	if err != nil {
		t.Fatal(err)
	}
	want := liveConfig{GuildID: 11, ChannelID: 12, StreamerToken: "file-streamer", StreamerID: 13, ListenerToken: "env-listener"}
	if config != want {
		t.Fatalf("config = %+v", config)
	}
}

func TestLoadLiveConfigRejectsIncompleteOrSharedCredentials(t *testing.T) {
	missing := filepath.Join(t.TempDir(), ".env")
	_, err := loadLiveConfig(missing, lookup(map[string]string{"INTEGRATION_TEST_DISCORD_BOT_STREAMER_TOKEN": "secret-value"}))
	if err == nil || !strings.Contains(err.Error(), "INTEGRATION_TEST_DISCORD_BOT_LISTENER_TOKEN") {
		t.Fatalf("missing values error = %v", err)
	}
	if strings.Contains(err.Error(), "secret-value") {
		t.Fatal("error exposed a credential")
	}

	shared := lookup(map[string]string{
		"INTEGRATION_TEST_DISCORD_SERVER_ID":              "1",
		"INTEGRATION_TEST_DISCORD_VOICE_CHANNEL_ID":       "2",
		"INTEGRATION_TEST_DISCORD_BOT_STREAMER_CLIENT_ID": "3",
		"INTEGRATION_TEST_DISCORD_BOT_STREAMER_TOKEN":     "same",
		"INTEGRATION_TEST_DISCORD_BOT_LISTENER_TOKEN":     "same",
	})
	if _, err := loadLiveConfig(missing, shared); err == nil || !strings.Contains(err.Error(), "distinct") {
		t.Fatalf("shared token error = %v", err)
	}

	invalid := lookup(map[string]string{
		"INTEGRATION_TEST_DISCORD_SERVER_ID":              "server",
		"INTEGRATION_TEST_DISCORD_VOICE_CHANNEL_ID":       "2",
		"INTEGRATION_TEST_DISCORD_BOT_STREAMER_CLIENT_ID": "3",
		"INTEGRATION_TEST_DISCORD_BOT_STREAMER_TOKEN":     "streamer",
		"INTEGRATION_TEST_DISCORD_BOT_LISTENER_TOKEN":     "listener",
	})
	if _, err := loadLiveConfig(missing, invalid); err == nil || !strings.Contains(err.Error(), "INTEGRATION_TEST_DISCORD_SERVER_ID") {
		t.Fatalf("invalid ID error = %v", err)
	}
}

func writeEnv(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func lookup(values map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		value, ok := values[name]
		return value, ok
	}
}
