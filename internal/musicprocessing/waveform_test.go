package musicprocessing

import (
	"slices"
	"testing"
)

func TestWaveformFromSamples(t *testing.T) {
	got := waveformFromSamples([]float32{0, 0.1, -0.5, 0.25, 0, 0}, 3)
	if want := []float64{0.2, 1, 0}; !slices.Equal(got, want) {
		t.Fatalf("peaks = %v; want %v", got, want)
	}
	if got := waveformFromSamples([]float32{0, 0}, 4); !slices.Equal(got, []float64{0, 0, 0, 0}) {
		t.Fatalf("silent peaks = %v", got)
	}
	if got := waveformFromSamples(nil, 2); !slices.Equal(got, []float64{0, 0}) {
		t.Fatalf("empty peaks = %v", got)
	}
}
