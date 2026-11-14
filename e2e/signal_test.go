package e2e

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestAnalyzeSeparatesTones(t *testing.T) {
	signal := analyze(tones(48000, 0.4, 0))
	if signal.Low < 0.35 || signal.High > 0.01 {
		t.Fatalf("low tone analyzed as %+v", signal)
	}
	signal = analyze(tones(48000, 0.2, 0.3))
	if math.Abs(signal.Low-0.2) > 0.01 || math.Abs(signal.High-0.3) > 0.01 {
		t.Fatalf("mixed tones analyzed as %+v", signal)
	}
	if signal.RMS < 0.2 {
		t.Fatalf("mixed tone RMS = %v", signal.RMS)
	}
}

func TestAnalyzeSilence(t *testing.T) {
	if signal := analyze(make([]byte, 48000*4)); signal != (Signal{}) {
		t.Fatalf("silence analyzed as %+v", signal)
	}
	if signal := analyze(nil); signal != (Signal{}) {
		t.Fatalf("empty PCM analyzed as %+v", signal)
	}
}

func tones(frames int, low, high float64) []byte {
	pcm := make([]byte, frames*4)
	for i := range frames {
		phase := 2 * math.Pi * float64(i) / 48000
		sample := int16((low*math.Sin(440*phase) + high*math.Sin(880*phase)) * 32767)
		binary.LittleEndian.PutUint16(pcm[i*4:], uint16(sample))
		binary.LittleEndian.PutUint16(pcm[i*4+2:], uint16(sample))
	}
	return pcm
}
