package server

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"net/http"
	"os/exec"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
	"github.com/terrabitz/rpg-audio-streamer/internal/livemixer"
	"github.com/terrabitz/rpg-audio-streamer/internal/mixer"
)

const mixerSampleRate = 48000

func (s *Server) handleMixerStatus(w http.ResponseWriter, r *http.Request, token *auth.Token) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.mixer == nil {
		http.Error(w, "Experimental mixer is disabled", http.StatusNotFound)
		return
	}
	id, ok := s.tableID(w, r, token)
	if !ok {
		return
	}
	if _, err := s.store.GetTable(r.Context(), id); err != nil {
		s.tableError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	respondJSON(w, http.StatusOK, s.mixer.Status(id.String()))
}

// RunMixer is separate from Handler so integration servers can own its cancellation.
func (s *Server) RunMixer(ctx context.Context) {
	if s.mixer != nil {
		s.mixer.Run(ctx)
	}
}

func (s *Server) mixerSource(ctx context.Context, tableID, fileID string) (livemixer.Clip, error) {
	table, err := uuid.Parse(tableID)
	if err != nil {
		return livemixer.Clip{}, fmt.Errorf("invalid table ID: %w", err)
	}
	file, err := uuid.Parse(fileID)
	if err != nil {
		return livemixer.Clip{}, fmt.Errorf("invalid track ID: %w", err)
	}
	associated, err := s.store.HasTableTrack(ctx, table, file)
	if err != nil {
		return livemixer.Clip{}, err
	}
	if !associated {
		return livemixer.Clip{}, fmt.Errorf("track is not associated with table")
	}
	track, err := s.store.GetTrackByID(ctx, file)
	if err != nil {
		return livemixer.Clip{}, err
	}
	trackType, err := s.store.GetTrackTypeByID(ctx, track.TypeID)
	if err != nil {
		return livemixer.Clip{}, err
	}
	frames, err := decodeMixerPCM(ctx, filepath.Join(track.Path, "index.m3u8"))
	if err != nil {
		return livemixer.Clip{}, err
	}
	return livemixer.Clip{
		Frames: frames, Type: trackType.Name, Repeating: trackType.IsRepeating,
		Playback: livemixer.Playback{
			StartTime: track.Playback.StartTime, EndTime: track.Playback.EndTime, LoopBackTime: track.Playback.LoopBackTime,
		},
	}, nil
}

func decodeMixerPCM(ctx context.Context, path string) ([]mixer.Frame, error) {
	var stdout, stderr limitedBuffer
	stdout.limit = 64 << 20
	stderr.limit = 64 << 10
	cmd := exec.CommandContext(ctx, "ffmpeg", "-v", "error", "-i", path,
		"-vn", "-ac", "2", "-ar", "48000", "-f", "f32le", "-")
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("decode mixer PCM: %w: %s", err, stderr.String())
	}
	if stdout.Len() == 0 || stdout.Len()%8 != 0 {
		return nil, fmt.Errorf("decoder returned empty or incomplete stereo PCM")
	}
	data := stdout.Bytes()
	frames := make([]mixer.Frame, len(data)/8)
	for i := range frames {
		left := float64(math.Float32frombits(binary.LittleEndian.Uint32(data[i*8:])))
		right := float64(math.Float32frombits(binary.LittleEndian.Uint32(data[i*8+4:])))
		if math.IsNaN(left) || math.IsInf(left, 0) || math.IsNaN(right) || math.IsInf(right, 0) {
			return nil, fmt.Errorf("decoder returned non-finite PCM")
		}
		// AAC decoding can overshoot unity; normalize to the core's PCM range.
		frames[i] = mixer.Frame{Left: max(-1, min(1, left)), Right: max(-1, min(1, right))}
	}
	return frames, nil
}

type limitedBuffer struct {
	bytes.Buffer
	limit int
}

func (b *limitedBuffer) Write(data []byte) (int, error) {
	if len(data) > b.limit-b.Len() {
		return 0, fmt.Errorf("decoder output exceeds %d byte limit", b.limit)
	}
	return b.Buffer.Write(data)
}
