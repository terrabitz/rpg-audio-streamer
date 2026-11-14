package discord

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

func startWorker(ctx context.Context, cfg Config) (*worker, error) {
	cmd := exec.CommandContext(ctx, "node", cfg.Worker)
	cmd.WaitDelay = 2 * time.Second
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
	cmd.Env = workerEnv(cfg)
	input, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("create Discord PCM pipe: %w", err)
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		input.Close()
		return nil, fmt.Errorf("create Discord status pipe: %w", err)
	}
	// Only structured, controlled worker errors are returned; third-party logs may contain credentials.
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		input.Close()
		return nil, fmt.Errorf("start Discord voice worker: %w", err)
	}
	done := make(chan struct{})
	process := &worker{input: input, done: done}
	ready := make(chan error, 1)
	go func() {
		scanner := bufio.NewScanner(output)
		first := true
		for scanner.Scan() {
			var event struct {
				State   string `json:"state"`
				Message string `json:"message"`
			}
			if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
				process.err = fmt.Errorf("invalid Discord worker status")
				break
			}
			if event.State == "error" {
				process.err = fmt.Errorf("Discord voice failed: %s", event.Message)
				break
			}
			if first && event.State == "connected" {
				ready <- nil
				first = false
			}
		}
		if scanner.Err() != nil {
			process.err = fmt.Errorf("Discord worker status stream failed")
		}
		if process.err != nil {
			_ = cmd.Process.Kill()
		}
		if err := cmd.Wait(); err != nil && process.err == nil {
			process.err = fmt.Errorf("Discord voice worker exited; verify dependencies, configuration, and network access")
		}
		if first {
			if process.err == nil {
				process.err = fmt.Errorf("Discord voice worker exited before connecting")
			}
			ready <- process.err
		}
		close(done)
	}()
	timeout := time.NewTimer(20 * time.Second)
	defer timeout.Stop()
	select {
	case err := <-ready:
		return process, err
	case <-ctx.Done():
		return process, ctx.Err()
	case <-timeout.C:
		return process, fmt.Errorf("Discord voice connection timed out")
	}
}

func workerEnv(cfg Config) []string {
	env := make([]string, 0, len(os.Environ())+3)
	for _, value := range os.Environ() {
		name, _, _ := strings.Cut(value, "=")
		if strings.HasPrefix(name, "DISCORD_") || strings.HasPrefix(name, "INTEGRATION_TEST_DISCORD_") {
			continue
		}
		env = append(env, value)
	}
	return append(env,
		"DISCORD_BOT_TOKEN="+cfg.Token,
		"DISCORD_GUILD_ID="+cfg.GuildID,
		"DISCORD_CHANNEL_ID="+cfg.ChannelID,
	)
}
