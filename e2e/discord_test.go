package e2e

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDiscordStreamsServerMix(t *testing.T) {
	if os.Getenv("E2E_DISCORD") != "1" {
		t.Skip("set E2E_DISCORD=1 to run against two bots in an isolated Discord voice channel")
	}
	root := repoRoot(t)
	config, err := loadLiveConfig(filepath.Join(root, ".env"), os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}
	ctx := t.Context()
	app := startApp(t, root, config)
	gm := app.login(t)
	low := gm.upload(t, "Discord Low", 440)
	high := gm.upload(t, "Discord High", 880)
	mix := gm.mixer(t)

	gm.connectDiscord(t)
	t.Cleanup(func() { gm.disconnectDiscord(t) })
	// The listener joins second so the streamer is the existing DAVE member
	// that commits; see joinerSession.
	listener, err := startListener(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(listener.Close)
	waitFor(t, "streamer joins the voice channel", listener, func(a Audio) bool { return a.Present })

	mix.sync(t, trackPatch{FileID: low, IsPlaying: ptr(true), IsRepeating: ptr(true), CurrentTime: ptr(0.0), Volume: ptr(100.0)})
	mix.sync(t, trackPatch{FileID: high, IsPlaying: ptr(true), IsRepeating: ptr(true), CurrentTime: ptr(0.0), Volume: ptr(100.0)})
	waitFor(t, "both tones arrive", listener, func(a Audio) bool { return a.Low > 0.05 && a.High > 0.05 })

	mix.sync(t, trackPatch{FileID: low, Volume: ptr(0.0)})
	mutedAt := listener.Latest().Frames
	waitFor(t, "muted tone disappears", listener, func(a Audio) bool {
		return a.Frames > mutedAt+sampleRate && a.Low < 0.003 && a.High > 0.05
	})

	mix.sync(t, trackPatch{FileID: high, IsPlaying: ptr(false)})
	pausedAt := listener.Latest().Frames
	waitFor(t, "paused mix goes silent", listener, func(a Audio) bool {
		silent := a.Frames > pausedAt+sampleRate && a.RMS < 0.003
		stopped := time.Since(a.Last) > 1500*time.Millisecond
		return a.Present && (silent || stopped)
	})

	mix.sync(t, trackPatch{FileID: high, IsPlaying: ptr(true)})
	resumedAt := listener.Latest().Frames
	waitFor(t, "resumed tone returns", listener, func(a Audio) bool {
		return a.Frames > resumedAt+sampleRate && a.High > 0.05
	})

	gm.disconnectDiscord(t)
	waitFor(t, "streamer leaves the voice channel", listener, func(a Audio) bool { return !a.Present })

	listener.Close()
	if err := mix.conn.Close(); err != nil {
		t.Fatal(err)
	}
	gm.waitDiscord(t, "disconnected")
	returnedMix := gm.mixer(t)
	gm.waitDiscord(t, "connected")
	returnedListener, err := startListener(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(returnedListener.Close)
	returnedMix.sync(t, trackPatch{FileID: high, IsPlaying: ptr(true), IsRepeating: ptr(true), CurrentTime: ptr(0.0), Volume: ptr(100.0)})
	waitFor(t, "saved destination reconnects and carries audio", returnedListener, func(a Audio) bool {
		return a.Present && a.High > 0.05
	})
	if err := returnedMix.conn.Close(); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "closing the last GM socket disconnects Discord", returnedListener, func(a Audio) bool { return !a.Present })
	gm.waitDiscord(t, "disconnected")
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
