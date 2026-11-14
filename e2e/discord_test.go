package e2e

import (
	"log/slog"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestDiscordStreamsServerMix(t *testing.T) {
	if os.Getenv("E2E_DISCORD") != "1" {
		t.Skip("set E2E_DISCORD=1 to run against two bots in an isolated Discord voice channel")
	}
	root := repoRoot(t)
	config, err := loadProjectLiveConfig(root, os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}
	previousLogger := slog.Default()
	redactor := strings.NewReplacer(
		config.GuildID.String(), "[redacted]", config.ChannelID.String(), "[redacted]",
		config.StreamerID.String(), "[redacted]", config.StreamerToken, "[redacted]",
		config.ListenerToken, "[redacted]",
	)
	slog.SetDefault(slog.New(slog.NewTextHandler(redactingWriter{redactor}, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { slog.SetDefault(previousLogger) })
	ctx := t.Context()
	app := startApp(t, root, config)
	gm := app.login(t)
	low := gm.upload(t, "Discord Low", 440)
	mix := gm.mixer(t)

	gm.connectDiscord(t)
	t.Cleanup(func() { gm.disconnectDiscord(t) })
	// The listener joins second so the streamer is the existing DAVE member
	// that commits; see joinerSession.
	listener, err := startListener(ctx, config)
	if err != nil {
		t.Fatal(discordID.ReplaceAllString(redactor.Replace(err.Error()), "[redacted]"))
	}
	t.Cleanup(listener.Close)
	waitFor(t, "streamer joins the voice channel", listener, func(a Audio) bool { return a.Present })

	mix.sync(t, trackPatch{FileID: low, IsPlaying: ptr(true), IsRepeating: ptr(true), CurrentTime: ptr(0.0), Volume: ptr(100.0)})
	waitFor(t, "server mix tone arrives", listener, func(a Audio) bool { return a.Low > 0.05 })

	gm.disconnectDiscord(t)
	waitFor(t, "streamer leaves the voice channel", listener, func(a Audio) bool { return !a.Present })
}

type redactingWriter struct {
	replacer *strings.Replacer
}

var discordID = regexp.MustCompile(`\b[0-9]{17,20}\b`)

func (w redactingWriter) Write(data []byte) (int, error) {
	redacted := discordID.ReplaceAllString(w.replacer.Replace(string(data)), "[redacted]")
	_, err := os.Stderr.WriteString(redacted)
	if err != nil {
		return 0, err
	}
	return len(data), nil
}

func waitFor(t *testing.T, description string, l *listener, done func(Audio) bool) {
	t.Helper()
	deadline := time.Now().Add(45 * time.Second)
	for {
		audio := l.Latest()
		if audio.Err != nil {
			t.Fatalf("%s: listener failed: %v", description, audio.Err)
		}
		if done(audio) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s: timed out with %+v", description, audio)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func ptr[T any](value T) *T {
	return &value
}
