package youtube

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
)

const maxImportBytes = 200 << 20

var (
	ErrInvalidURL      = domain.Invalid("Enter a YouTube video link, not a playlist")
	ErrUnavailable     = domain.Unavailable("YouTube import requires yt-dlp on the server")
	ErrInvalidMetadata = domain.Upstream("Could not determine the video name and duration")
	ErrLiveStream      = domain.Unsupported("Live streams are not supported")
	ErrDurationLimit   = domain.Unsupported("Video exceeds the 3-hour limit")
	ErrSizeLimit       = domain.Unsupported("Audio exceeds the 200 MiB limit")
)

// Client retrieves single YouTube videos through the yt-dlp executable on PATH.
type Client struct {
	logger *slog.Logger
}

func New(logger *slog.Logger) *Client {
	return &Client{logger: logger}
}

func (c *Client) Scan(ctx context.Context, sourceURL string) (domain.VideoMetadata, error) {
	source, err := c.Resolve(sourceURL)
	if err != nil {
		return domain.VideoMetadata{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	cmd := videoCommand(ctx, "--skip-download", "--dump-single-json", "--", source)
	output := cappedOutput{limit: 1 << 20}
	var stderr cappedOutput
	cmd.Stdout, cmd.Stderr = &output, &stderr
	err = cmd.Run()
	if ctx.Err() != nil {
		return domain.VideoMetadata{}, ctx.Err()
	}
	if err != nil {
		c.logger.Warn("YouTube scan failed", "error", err, "stderr", stderr.buf.String())
		return domain.VideoMetadata{}, fmt.Errorf("retrieve video metadata: %w", err)
	}
	var info struct {
		Title          string  `json:"title"`
		Duration       float64 `json:"duration"`
		Filesize       *int64  `json:"filesize"`
		FilesizeApprox *int64  `json:"filesize_approx"`
		IsLive         bool    `json:"is_live"`
		LiveStatus     string  `json:"live_status"`
	}
	if err := json.Unmarshal(output.buf.Bytes(), &info); err != nil {
		c.logger.Warn("invalid YouTube metadata", "error", err)
		return domain.VideoMetadata{}, ErrInvalidMetadata
	}
	if info.IsLive || info.LiveStatus == "is_upcoming" || info.LiveStatus == "post_live" {
		return domain.VideoMetadata{}, ErrLiveStream
	}
	if strings.TrimSpace(info.Title) == "" || info.Duration <= 0 {
		return domain.VideoMetadata{}, ErrInvalidMetadata
	}
	if info.Duration > 3*time.Hour.Seconds() {
		return domain.VideoMetadata{}, ErrDurationLimit
	}
	size := info.Filesize
	estimated := false
	if size == nil || *size <= 0 {
		size, estimated = info.FilesizeApprox, true
	}
	if size != nil && *size > maxImportBytes {
		return domain.VideoMetadata{}, ErrSizeLimit
	}
	if size != nil && *size <= 0 {
		size = nil
	}
	return domain.VideoMetadata{URL: source, Name: info.Title, Duration: info.Duration,
		SizeBytes: size, SizeEstimated: estimated}, nil
}

// ValidateURL accepts only single-video links, without requiring yt-dlp.
func (c *Client) ValidateURL(sourceURL string) error {
	if _, ok := CanonicalURL(sourceURL); !ok {
		return ErrInvalidURL
	}
	return nil
}

// Resolve returns the canonical single-video URL once retrieval is possible.
func (c *Client) Resolve(sourceURL string) (string, error) {
	source, ok := CanonicalURL(sourceURL)
	if !ok {
		return "", ErrInvalidURL
	}
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		return "", ErrUnavailable
	}
	return source, nil
}

func (c *Client) Download(ctx context.Context, source, destination string) error {
	cmd := videoCommand(ctx, "--no-part", "--max-filesize", "200M",
		"--match-filter", "!is_live & duration <= 10800", "--output", destination, "--", source)
	output := cappedOutput{limit: 1 << 20}
	cmd.Stdout, cmd.Stderr = &output, &output
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			err = fmt.Errorf("%w: %w", ctx.Err(), err)
		}
		return downloadError(cmd, &output, err)
	}
	info, err := os.Stat(destination)
	if err != nil {
		return downloadError(cmd, &output, fmt.Errorf("YouTube audio unavailable or exceeds import limits: %w", err))
	}
	if !info.Mode().IsRegular() || info.Size() == 0 || info.Size() > maxImportBytes {
		return downloadError(cmd, &output, errors.New("YouTube audio is empty or exceeds the 200 MiB limit"))
	}
	return nil
}

func downloadError(cmd *exec.Cmd, output *cappedOutput, err error) error {
	args := make([]string, len(cmd.Args))
	for i, arg := range cmd.Args {
		args[i] = "'" + strings.ReplaceAll(arg, "'", "'\\''") + "'"
	}
	logs := output.buf.String()
	if output.truncated {
		logs += "\n[Output truncated after 1 MiB]"
	}
	return fmt.Errorf("Command:\n%s\n\nLogs:\n%s\n\nError:\n%w", strings.Join(args, " "), logs, err)
}

func videoCommand(ctx context.Context, args ...string) *exec.Cmd {
	base := []string{"--ignore-config", "--no-plugin-dirs", "--no-cache-dir", "--js-runtimes", "node",
		"--no-playlist", "--no-progress", "--socket-timeout", "30",
		"--retries", "2", "--format", "bestaudio/best"}
	cmd := exec.CommandContext(ctx, "yt-dlp", append(base, args...)...)
	cmd.WaitDelay = 2 * time.Second
	return cmd
}

var videoID = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

func CanonicalURL(value string) (string, bool) {
	u, err := url.Parse(strings.TrimSpace(value))
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.Port() != "" {
		return "", false
	}
	var id string
	switch strings.ToLower(u.Host) {
	case "youtu.be":
		id = strings.TrimPrefix(u.Path, "/")
	case "youtube.com", "www.youtube.com", "m.youtube.com", "music.youtube.com":
		switch {
		case u.Path == "/watch":
			id = u.Query().Get("v")
		case strings.HasPrefix(u.Path, "/shorts/"):
			id = strings.TrimPrefix(u.Path, "/shorts/")
		case strings.HasPrefix(u.Path, "/embed/"):
			id = strings.TrimPrefix(u.Path, "/embed/")
		case strings.HasPrefix(u.Path, "/live/"):
			id = strings.TrimPrefix(u.Path, "/live/")
		}
	}
	if !videoID.MatchString(id) {
		return "", false
	}
	return "https://www.youtube.com/watch?v=" + id, true
}

type cappedOutput struct {
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func (b *cappedOutput) Write(p []byte) (int, error) {
	n := len(p)
	limit := b.limit
	if limit == 0 {
		limit = 8 << 10
	}
	if len(p) > limit-b.buf.Len() {
		b.truncated = true
	}
	if remaining := limit - b.buf.Len(); remaining > 0 {
		if len(p) > remaining {
			p = p[:remaining]
		}
		_, _ = b.buf.Write(p)
	}
	return n, nil
}
