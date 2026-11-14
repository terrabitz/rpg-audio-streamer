package e2e

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/disgoorg/snowflake/v2"
)

type liveConfig struct {
	GuildID       snowflake.ID
	ChannelID     snowflake.ID
	StreamerToken string
	StreamerID    snowflake.ID
	ListenerToken string
}

// loadLiveConfig reads the Discord test credentials from envPath, letting the
// process environment override the file.
func loadLiveConfig(envPath string, lookupEnv func(string) (string, bool)) (liveConfig, error) {
	file, err := readEnvFile(envPath)
	if err != nil {
		return liveConfig{}, err
	}
	value := func(name string) string {
		if v, ok := lookupEnv(name); ok && v != "" {
			return v
		}
		return file[name]
	}

	names := []string{
		"INTEGRATION_TEST_DISCORD_SERVER_ID",
		"INTEGRATION_TEST_DISCORD_VOICE_CHANNEL_ID",
		"INTEGRATION_TEST_DISCORD_BOT_STREAMER_TOKEN",
		"INTEGRATION_TEST_DISCORD_BOT_STREAMER_CLIENT_ID",
		"INTEGRATION_TEST_DISCORD_BOT_LISTENER_TOKEN",
	}
	var missing []string
	for _, name := range names {
		if value(name) == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return liveConfig{}, fmt.Errorf("missing Discord configuration: %s", strings.Join(missing, ", "))
	}

	var invalid []string
	id := func(name string) snowflake.ID {
		parsed, err := snowflake.Parse(value(name))
		if err != nil || parsed == 0 {
			invalid = append(invalid, name)
		}
		return parsed
	}
	config := liveConfig{
		GuildID:       id("INTEGRATION_TEST_DISCORD_SERVER_ID"),
		ChannelID:     id("INTEGRATION_TEST_DISCORD_VOICE_CHANNEL_ID"),
		StreamerToken: value("INTEGRATION_TEST_DISCORD_BOT_STREAMER_TOKEN"),
		StreamerID:    id("INTEGRATION_TEST_DISCORD_BOT_STREAMER_CLIENT_ID"),
		ListenerToken: value("INTEGRATION_TEST_DISCORD_BOT_LISTENER_TOKEN"),
	}
	if len(invalid) > 0 {
		return liveConfig{}, fmt.Errorf("invalid Discord IDs: %s", strings.Join(invalid, ", "))
	}
	if config.StreamerToken == config.ListenerToken {
		return liveConfig{}, errors.New("discord E2E requires distinct streamer and listener bot tokens")
	}
	return config, nil
}

func readEnvFile(path string) (map[string]string, error) {
	values := map[string]string{}
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return values, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, raw, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		values[strings.TrimSpace(name)] = unquote(strings.TrimSpace(raw))
	}
	return values, scanner.Err()
}

func unquote(value string) string {
	if len(value) >= 2 && value[0] == '\'' && value[len(value)-1] == '\'' {
		return value[1 : len(value)-1]
	}
	if unquoted, err := strconv.Unquote(value); err == nil && strings.HasPrefix(value, `"`) {
		return unquoted
	}
	return value
}
