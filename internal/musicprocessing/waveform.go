package musicprocessing

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"os/exec"
	"path/filepath"
)

const (
	waveformPeaks      = 400
	waveformSampleRate = 8000
)

type Waveform struct {
	Peaks []float64 `json:"peaks"`
}

func GenerateWaveform(ctx context.Context, dir string) (Waveform, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "ffmpeg", "-nostdin", "-v", "error", "-i", filepath.Join(dir, "index.m3u8"),
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
	return Waveform{Peaks: waveformFromSamples(samples, waveformPeaks)}, nil
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
