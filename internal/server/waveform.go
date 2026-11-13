package server

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
)

const (
	waveformFile       = "waveform.json"
	waveformPeaks      = 400
	waveformSampleRate = 8000
)

type Waveform struct {
	Peaks []float64 `json:"peaks"`
}

func (s *Server) handleFileWaveform(w http.ResponseWriter, r *http.Request, token *auth.Token) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	trackID, err := uuid.Parse(r.PathValue("trackID"))
	if err != nil {
		http.Error(w, "Invalid track ID", http.StatusBadRequest)
		return
	}
	track, err := s.store.GetTrackByID(r.Context(), trackID)
	if err != nil {
		http.Error(w, "Track not found", http.StatusNotFound)
		return
	}
	waveform, err := loadWaveform(track.Path)
	if err != nil {
		s.logger.Error("failed to load waveform", "error", err, "trackID", trackID)
		http.Error(w, "Failed to generate waveform", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=3600")
	respondJSON(w, http.StatusOK, waveform)
}

// loadWaveform reads the cached waveform for an HLS track directory, generating
// it from the stored audio for tracks uploaded before waveforms existed.
func loadWaveform(dir string) (Waveform, error) {
	var waveform Waveform
	data, err := os.ReadFile(filepath.Join(dir, waveformFile))
	if err == nil && json.Unmarshal(data, &waveform) == nil && len(waveform.Peaks) > 0 {
		return waveform, nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Waveform{}, err
	}
	return generateWaveform(dir)
}

func generateWaveform(dir string) (Waveform, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("ffmpeg", "-v", "error", "-i", filepath.Join(dir, "index.m3u8"),
		"-vn", "-ac", "1", "-ar", fmt.Sprint(waveformSampleRate), "-f", "f32le", "-")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return Waveform{}, fmt.Errorf("decode audio: %w: %s", err, stderr.String())
	}
	samples := make([]float32, stdout.Len()/4)
	if err := binary.Read(&stdout, binary.LittleEndian, samples); err != nil {
		return Waveform{}, fmt.Errorf("read samples: %w", err)
	}
	waveform := Waveform{Peaks: waveformFromSamples(samples, waveformPeaks)}

	data, err := json.Marshal(waveform)
	if err != nil {
		return Waveform{}, err
	}
	// Write through a temporary file so concurrent readers never see a partial cache.
	tmp, err := os.CreateTemp(dir, waveformFile+".*")
	if err != nil {
		return Waveform{}, err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return Waveform{}, err
	}
	if err := tmp.Close(); err != nil {
		return Waveform{}, err
	}
	if err := os.Rename(tmp.Name(), filepath.Join(dir, waveformFile)); err != nil {
		return Waveform{}, err
	}
	return waveform, nil
}

// waveformFromSamples returns the peak amplitude of each of count equal time
// buckets, scaled so the loudest bucket is 1.
func waveformFromSamples(samples []float32, count int) []float64 {
	peaks := make([]float64, count)
	if len(samples) == 0 {
		return peaks
	}
	loudest := 0.0
	for i := range peaks {
		from := i * len(samples) / count
		to := max((i+1)*len(samples)/count, from+1)
		for _, sample := range samples[from:min(to, len(samples))] {
			peaks[i] = max(peaks[i], math.Abs(float64(sample)))
		}
		loudest = max(loudest, peaks[i])
	}
	if loudest == 0 {
		return peaks
	}
	for i := range peaks {
		peaks[i] = math.Round(peaks[i]/loudest*1000) / 1000
	}
	return peaks
}
