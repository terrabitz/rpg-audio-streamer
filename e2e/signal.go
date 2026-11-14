// Package e2e verifies the running application against real external services.
package e2e

import (
	"encoding/binary"
	"math"
)

const sampleRate = 48000

// Signal summarizes one window of received stereo audio by the amplitudes of
// the two test tones and the overall loudness.
type Signal struct {
	Low  float64
	High float64
	RMS  float64
}

func analyze(pcm []byte) Signal {
	frames := len(pcm) / 4
	if frames == 0 {
		return Signal{}
	}
	var sum float64
	frequencies := [2]float64{440, 880}
	var real, imaginary [2]float64
	for i := range frames {
		left := int16(binary.LittleEndian.Uint16(pcm[i*4:]))
		right := int16(binary.LittleEndian.Uint16(pcm[i*4+2:]))
		sample := (float64(left) + float64(right)) / 65536
		sum += sample * sample
		for j, frequency := range frequencies {
			angle := 2 * math.Pi * frequency * float64(i) / sampleRate
			real[j] += sample * math.Cos(angle)
			imaginary[j] += sample * math.Sin(angle)
		}
	}
	return Signal{
		Low:  2 * math.Hypot(real[0], imaginary[0]) / float64(frames),
		High: 2 * math.Hypot(real[1], imaginary[1]) / float64(frames),
		RMS:  math.Sqrt(sum / float64(frames)),
	}
}
