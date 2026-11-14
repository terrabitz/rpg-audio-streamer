// Package mixer renders decoded stereo PCM without clocks, devices, or network I/O.
package mixer

import (
	"fmt"
	"math"
	"slices"
)

type Frame struct {
	Left, Right float64
}

type State struct {
	Position int
	Playing  bool
}

// Region uses absolute frame offsets; End is exclusive.
type Region struct {
	Start, End, LoopBack int
}

// Mixer must be owned by one goroutine, including control changes between renders.
type Mixer struct {
	sampleRate int
	tracks     []*track
}

type track struct {
	id        string
	samples   []Frame
	position  int
	playing   bool
	region    Region
	repeating bool
	overlap   *overlap
	gain      ramp
	envelope  ramp
}

type overlap struct {
	start, incoming int
}

type ramp struct {
	from, to       float64
	elapsed, total int
}

func New(sampleRate int) (*Mixer, error) {
	if sampleRate <= 0 || sampleRate > math.MaxInt/2 {
		return nil, fmt.Errorf("sample rate must be positive and allow a two-second frame count")
	}
	return &Mixer{sampleRate: sampleRate}, nil
}

// Add copies a clip at the mixer's sample rate and leaves it paused.
func (m *Mixer) Add(id string, samples []Frame) error {
	if id == "" || len(samples) == 0 {
		return fmt.Errorf("track requires an ID and PCM frames")
	}
	for _, t := range m.tracks {
		if t.id == id {
			return fmt.Errorf("track %q already exists", id)
		}
	}
	for _, frame := range samples {
		if !validSample(frame.Left) || !validSample(frame.Right) {
			return fmt.Errorf("track %q contains invalid PCM", id)
		}
	}
	m.tracks = append(m.tracks, &track{
		id: id, samples: slices.Clone(samples), region: Region{End: len(samples)},
		gain: ramp{to: 1},
	})
	return nil
}

func (m *Mixer) Play(id string) error {
	t, err := m.find(id)
	if err != nil {
		return err
	}
	t.setPlaying(true, m.sampleRate)
	return nil
}

func (m *Mixer) Pause(id string) error {
	t, err := m.find(id)
	if err != nil {
		return err
	}
	t.setPlaying(false, m.sampleRate)
	return nil
}

func (m *Mixer) Seek(id string, position int) error {
	t, err := m.find(id)
	if err != nil {
		return err
	}
	if position < t.region.Start || position > t.region.End {
		return fmt.Errorf("seek position must be within the playback region")
	}
	t.position = position
	t.overlap = nil
	return nil
}

func (m *Mixer) SetRegion(id string, region Region) error {
	t, err := m.find(id)
	if err != nil {
		return err
	}
	if region.Start < 0 || region.End <= region.Start || region.End > len(t.samples) ||
		region.LoopBack < region.Start || region.LoopBack >= region.End {
		return fmt.Errorf("invalid playback region for track %q", id)
	}
	if t.region == region {
		return nil
	}
	t.region = region
	t.overlap = nil
	if t.position < region.Start || t.position >= region.End {
		t.position = region.Start
	}
	return nil
}

func (m *Mixer) SetRepeating(id string, repeating bool) error {
	t, err := m.find(id)
	if err != nil {
		return err
	}
	if t.repeating == repeating {
		return nil
	}
	t.repeating = repeating
	t.overlap = nil
	return nil
}

func (m *Mixer) Remove(id string) error {
	t, err := m.find(id)
	if err != nil {
		return err
	}
	m.tracks = slices.DeleteFunc(m.tracks, func(candidate *track) bool { return candidate == t })
	return nil
}

// SetGain accepts the effective track * master * type gain in [0, 1].
func (m *Mixer) SetGain(id string, gain float64) error {
	t, err := m.find(id)
	if err != nil {
		return err
	}
	if !validSample(gain) || gain < 0 {
		return fmt.Errorf("gain must be finite and between zero and one")
	}
	if t.gain.to == gain {
		return nil
	}
	t.gain.retarget(gain, 2*m.sampleRate)
	return nil
}

func (m *Mixer) State(id string) (State, error) {
	t, err := m.find(id)
	if err != nil {
		return State{}, err
	}
	return State{Position: t.position, Playing: t.playing}, nil
}

// Render overwrites dst, advances by exactly len(dst) frames, and clips only the final sum.
func (m *Mixer) Render(dst []Frame) {
	clear(dst)
	for i := range dst {
		for _, t := range m.tracks {
			frame := t.next(m.sampleRate)
			dst[i].Left += frame.Left
			dst[i].Right += frame.Right
		}
		dst[i].Left = math.Max(-1, math.Min(1, dst[i].Left))
		dst[i].Right = math.Max(-1, math.Min(1, dst[i].Right))
	}
}

func (t *track) next(sampleRate int) Frame {
	gain := t.gain.step()
	envelope := t.envelope.step()
	if !t.playing && envelope == 0 {
		return Frame{}
	}
	if t.position == t.region.End {
		t.finish()
		if !t.playing {
			return Frame{}
		}
	}
	fadeFrames := min(2*sampleRate, (t.region.End-t.region.LoopBack)/3, t.region.End-t.region.Start)
	if t.playing && t.repeating && t.overlap == nil && fadeFrames > 0 && t.position >= t.region.End-fadeFrames {
		t.overlap = &overlap{start: t.position, incoming: t.region.LoopBack}
	}
	frame := t.samples[t.position]
	if t.overlap != nil {
		progress := float64(t.position-t.overlap.start) / float64(t.region.End-t.overlap.start)
		head := t.samples[t.overlap.incoming]
		frame.Left = frame.Left*(1-progress) + head.Left*progress
		frame.Right = frame.Right*(1-progress) + head.Right*progress
		t.overlap.incoming++
	}
	t.position++
	if t.position == t.region.End {
		t.finish()
	}
	frame.Left *= gain * envelope
	frame.Right *= gain * envelope
	return frame
}

func (t *track) finish() {
	if !t.playing || !t.repeating {
		t.position = t.region.Start
		t.playing = false
		t.envelope = ramp{}
		t.overlap = nil
		return
	}
	t.position = t.region.LoopBack
	if t.overlap != nil {
		t.position = t.overlap.incoming
		t.overlap = nil
	}
}

func (t *track) setPlaying(playing bool, sampleRate int) {
	if t.playing == playing {
		return
	}
	t.playing = playing
	if !playing {
		t.overlap = nil
	}
	target := 0.0
	if playing {
		target = 1
	}
	if t.repeating {
		t.envelope.retarget(target, 2*sampleRate)
		return
	}
	t.envelope = ramp{to: target}
}

func (r *ramp) retarget(target float64, frames int) {
	*r = ramp{from: r.value(), to: target, total: frames}
}

func (r *ramp) step() float64 {
	value := r.value()
	if r.elapsed < r.total {
		r.elapsed++
	}
	return value
}

func (r *ramp) value() float64 {
	if r.elapsed >= r.total {
		return r.to
	}
	return r.from + (r.to-r.from)*float64(r.elapsed)/float64(r.total)
}

func (m *Mixer) find(id string) (*track, error) {
	for _, t := range m.tracks {
		if t.id == id {
			return t, nil
		}
	}
	return nil, fmt.Errorf("track %q not found", id)
}

func validSample(sample float64) bool {
	return !math.IsNaN(sample) && !math.IsInf(sample, 0) && sample >= -1 && sample <= 1
}
