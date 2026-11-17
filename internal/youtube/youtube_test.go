package youtube

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCanonicalURL(t *testing.T) {
	for _, value := range []string{
		"https://www.youtube.com/watch?v=aaaaaaaaaaa&list=PL123",
		" http://youtu.be/aaaaaaaaaaa?t=42 ",
		"https://youtube.com/shorts/aaaaaaaaaaa",
		"https://m.youtube.com/watch?v=aaaaaaaaaaa",
		"https://music.youtube.com/watch?v=aaaaaaaaaaa",
		"https://www.youtube.com/embed/aaaaaaaaaaa",
		"https://www.youtube.com/live/aaaaaaaaaaa",
	} {
		if got, ok := CanonicalURL(value); !ok || got != "https://www.youtube.com/watch?v=aaaaaaaaaaa" {
			t.Errorf("CanonicalURL(%q) = %q, %v", value, got, ok)
		}
	}
	for _, value := range []string{
		"", "--exec=bad", "file:///etc/passwd", "https://127.0.0.1/watch?v=aaaaaaaaaaa",
		"https://youtube.com.evil.example/watch?v=aaaaaaaaaaa",
		"https://youtube.com@evil.example/watch?v=aaaaaaaaaaa",
		"https://user@youtube.com/watch?v=aaaaaaaaaaa",
		"https://youtube.com:8080/watch?v=aaaaaaaaaaa",
		"https://youtube.com/playlist?list=PL123",
		"https://youtu.be/aaaaaaaaaaa/extra", "https://youtube.com/redirect?q=https://example.com",
		"https://youtube.com/watch?v=bad", "ftp://youtube.com/watch?v=aaaaaaaaaaa",
	} {
		if got, ok := CanonicalURL(value); ok {
			t.Errorf("accepted unsafe or non-video URL %q as %q", value, got)
		}
	}
}

func TestResolve(t *testing.T) {
	client := testClient()
	t.Setenv("PATH", t.TempDir())
	if _, err := client.Resolve("https://youtube.com/playlist?list=PL123"); !errors.Is(err, ErrInvalidURL) {
		t.Fatalf("playlist resolve error = %v", err)
	}
	if _, err := client.Resolve("https://youtu.be/aaaaaaaaaaa"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("missing downloader error = %v", err)
	}
	fixtureDownloader(t)
	source, err := client.Resolve("https://youtu.be/aaaaaaaaaaa")
	if err != nil || source != "https://www.youtube.com/watch?v=aaaaaaaaaaa" {
		t.Fatalf("resolved source = %q, %v", source, err)
	}
}

func TestScanUsesRequestContext(t *testing.T) {
	client := testClient()
	started := fixtureDownloader(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	finished := make(chan error, 1)
	go func() {
		_, err := client.Scan(ctx, "https://youtu.be/ddddddddddd")
		finished <- err
	}()
	waitForFile(t, started)
	cancel()
	select {
	case err := <-finished:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("scan ignored request cancellation: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("scan did not stop after request cancellation")
	}
}

func testClient() *Client {
	return New(slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func fixtureDownloader(t *testing.T) string {
	t.Helper()
	script, err := os.ReadFile("testdata/yt-dlp")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "yt-dlp"), script, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	started := filepath.Join(t.TempDir(), "started")
	t.Setenv("IMPORT_ARGS", started)
	return started
}

func waitForFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("downloader did not start")
}
