package mixer

import (
	"fmt"
	"math"
	"testing"
)

func TestMixStereoTracks(t *testing.T) {
	m := newMixer(t, 6)
	addPlaying(t, m, "music", []Frame{{0.8, -0.4}, {0.2, 0.3}})
	addPlaying(t, m, "ambiance", []Frame{{0.7, 0.2}})
	addPlaying(t, m, "effect", []Frame{{-0.6, -0.9}})

	out := make([]Frame, 3)
	m.Render(out)
	assertFrames(t, out, []Frame{{0.9, -1}, {0.2, 0.3}, {0, 0}})
}

func TestRenderContinuesAcrossBlocks(t *testing.T) {
	m := newMixer(t, 6)
	addPlaying(t, m, "music", []Frame{{0.1, 0.2}, {0.3, 0.4}})
	out := []Frame{{1, 1}}
	m.Render(out)
	assertFrames(t, out, []Frame{{0.1, 0.2}})
	m.Render(out)
	assertFrames(t, out, []Frame{{0.3, 0.4}})
	m.Render(out)
	assertFrames(t, out, []Frame{{0, 0}})
}

func TestSourceUnderflowPausesTrackPositionUntilPCMIsReady(t *testing.T) {
	source := &delayedSource{frame: Frame{Left: 0.5}}
	m := newMixer(t, 6)
	must(t, m.AddSource("stream", source))
	must(t, m.Play("stream"))
	out := make([]Frame, 3)
	m.Render(out)
	assertFrames(t, out, []Frame{{}, {}, {}})
	assertState(t, m, "stream", State{Position: 0, Playing: true})
	source.ready = true
	m.Render(out[:1])
	assertFrames(t, out[:1], []Frame{{Left: 0.5}})
	assertState(t, m, "stream", State{Position: 1, Playing: true})
	m.Close()
	if !source.closed {
		t.Fatal("closing the mixer did not close its source")
	}
}

func TestPlaybackControls(t *testing.T) {
	m := newMixer(t, 6)
	samples := []Frame{{0.1, 0.2}, {0.3, 0.4}, {0.5, 0.6}}
	must(t, m.Add("music", samples))
	samples[0] = Frame{1, 1}
	out := make([]Frame, 1)
	m.Render(out)
	assertFrames(t, out, []Frame{{0, 0}})

	must(t, m.Play("music"))
	m.Render(out)
	assertFrames(t, out, []Frame{{0.1, 0.2}})
	must(t, m.Pause("music"))
	m.Render(out)
	assertFrames(t, out, []Frame{{0, 0}})
	assertState(t, m, "music", State{Position: 1, Playing: false})

	must(t, m.Seek("music", 2))
	m.Render(out)
	assertFrames(t, out, []Frame{{0, 0}})
	must(t, m.Play("music"))
	m.Render(out)
	assertFrames(t, out, []Frame{{0.5, 0.6}})
	assertState(t, m, "music", State{Position: 0, Playing: false})

	must(t, m.Play("music"))
	m.Render(out)
	assertFrames(t, out, []Frame{{0.1, 0.2}})
	must(t, m.Seek("music", 1))
	m.Render(out)
	assertFrames(t, out, []Frame{{0.3, 0.4}})
	must(t, m.Remove("music"))
	m.Render(out)
	assertFrames(t, out, []Frame{{0, 0}})
	if _, err := m.State("music"); err == nil {
		t.Fatal("removed track still exists")
	}

}

type delayedSource struct {
	frame      Frame
	ready      bool
	readyUntil int
	readyFrom  int
	closed     bool
}

func (*delayedSource) Len() int { return 12 }

func (s *delayedSource) FrameAt(position int) (Frame, bool) {
	return s.frame, s.ready || position < s.readyUntil || s.readyFrom > 0 && position >= s.readyFrom
}

func (*delayedSource) Err() error { return nil }

func (s *delayedSource) Close() error {
	s.closed = true
	return nil
}

func TestPlaybackRegion(t *testing.T) {
	m := newMixer(t, 6)
	must(t, m.Add("music", []Frame{{0.1, 0.1}, {0.2, 0.2}, {0.3, 0.3}, {0.4, 0.4}}))
	must(t, m.SetRegion("music", Region{Start: 1, End: 3, LoopBack: 2}))
	must(t, m.Play("music"))
	out := make([]Frame, 3)
	m.Render(out)
	assertFrames(t, out, []Frame{{0.2, 0.2}, {0.3, 0.3}, {0, 0}})
	assertState(t, m, "music", State{Position: 1})
	for _, position := range []int{0, 4} {
		if err := m.Seek("music", position); err == nil {
			t.Fatalf("accepted seek outside region: %d", position)
		}
		if err := m.CrossfadeSeek("music", position); err == nil {
			t.Fatalf("accepted crossfade seek outside region: %d", position)
		}
	}
	must(t, m.Seek("music", 2))
	must(t, m.SetRegion("music", Region{Start: 2, End: 4, LoopBack: 2}))
	must(t, m.Play("music"))
	m.Render(out)
	assertFrames(t, out, []Frame{{0.3, 0.3}, {0.4, 0.4}, {0, 0}})
}

func TestLoopKeepsOutgoingAudioWhileIncomingPCMIsBuffered(t *testing.T) {
	m := newMixer(t, 1)
	source := &delayedSource{frame: Frame{Left: 0.5}, readyFrom: 8}
	must(t, m.AddSource("music", source))
	must(t, m.Play("music"))
	must(t, m.SetRepeating("music", true))
	must(t, m.Seek("music", 10))
	out := make([]Frame, 1)
	m.Render(out)
	assertFrames(t, out, []Frame{{Left: 0.5}})
	assertState(t, m, "music", State{Position: 11, Playing: true})
	source.ready = true
	m.Render(out)
	assertFrames(t, out, []Frame{{Left: 0.5}})
	assertState(t, m, "music", State{Position: 1, Playing: true})
}

func TestRegionCommitCrossfadesToBeginningAndLoopsPastEnd(t *testing.T) {
	m := newMixer(t, 2)
	frames := constantFrames(40, Frame{Left: 0.5})
	for i := 10; i < len(frames); i++ {
		frames[i] = Frame{Right: 0.5}
	}
	addPlaying(t, m, "music", frames)
	must(t, m.Seek("music", 2))
	must(t, m.SetRegion("music", Region{Start: 10, End: 40, LoopBack: 20}))
	out := make([]Frame, 5)
	m.Render(out)
	assertFrames(t, out, []Frame{{Left: 0.5}, {0.375, 0.125}, {0.25, 0.25}, {0.125, 0.375}, {Right: 0.5}})
	must(t, m.SetRepeating("music", true))
	must(t, m.Seek("music", 35))
	must(t, m.SetRegion("music", Region{Start: 10, End: 30, LoopBack: 20}))
	m.Render(out[:1])
	assertState(t, m, "music", State{Position: 21, Playing: true})
}

func TestFirstFrameUsesConfiguredGainWithoutAFullVolumeRamp(t *testing.T) {
	for _, gain := range []float64{0, 0.25} {
		m := newMixer(t, 2)
		must(t, m.Add("effect", constantFrames(10, Frame{1, 1})))
		must(t, m.SetGain("effect", gain))
		must(t, m.Play("effect"))
		out := make([]Frame, 3)
		m.Render(out)
		assertFrames(t, out, constantFrames(3, Frame{gain, gain}))
	}
}

func TestSeekingToEndResolvesOnPlayback(t *testing.T) {
	for _, repeating := range []bool{false, true} {
		m := newMixer(t, 6)
		addPlaying(t, m, "music", []Frame{{0.1, 0.2}, {0.3, 0.4}})
		must(t, m.Pause("music"))
		must(t, m.SetRepeating("music", repeating))
		must(t, m.Seek("music", 2))
		assertState(t, m, "music", State{Position: 2})
		must(t, m.Play("music"))
		out := make([]Frame, 1)
		m.Render(out)
		assertFrames(t, out, []Frame{{0, 0}})
		if repeating {
			assertState(t, m, "music", State{Position: 1, Playing: true})
		} else {
			assertState(t, m, "music", State{Position: 0})
		}
	}
}

func TestInvalidRegionLeavesPlaybackUnchanged(t *testing.T) {
	for _, region := range []Region{
		{Start: -1, End: 3}, {Start: 2, End: 2, LoopBack: 2},
		{End: 5}, {Start: 1, End: 4}, {End: 4, LoopBack: 4},
	} {
		t.Run(fmt.Sprint(region), func(t *testing.T) {
			m := newMixer(t, 6)
			addPlaying(t, m, "music", []Frame{{0.1, 0.2}, {0.3, 0.4}, {0.5, 0.6}, {0.7, 0.8}})
			if err := m.SetRegion("music", region); err == nil {
				t.Fatal("accepted invalid playback region")
			}
			out := make([]Frame, 1)
			m.Render(out)
			assertFrames(t, out, []Frame{{0.1, 0.2}})
		})
	}
}

func TestLoopCrossfadeSkipsIntroAndConsumedHead(t *testing.T) {
	m := newMixer(t, 3)
	samples := make([]Frame, 12)
	for i := range samples {
		samples[i] = Frame{float64(i) / 20, -float64(i) / 20}
	}
	addPlaying(t, m, "music", samples)
	must(t, m.SetRegion("music", Region{Start: 1, End: 12, LoopBack: 3}))
	must(t, m.SetRepeating("music", true))
	out := make([]Frame, 13)
	m.Render(out)
	assertFrames(t, out, []Frame{
		{0.05, -0.05}, {0.1, -0.1}, {0.15, -0.15}, {0.2, -0.2},
		{0.25, -0.25}, {0.3, -0.3}, {0.35, -0.35}, {0.4, -0.4},
		{0.45, -0.45}, {0.4, -0.4}, {0.35, -0.35},
		{0.3, -0.3}, {0.35, -0.35},
	})
	assertState(t, m, "music", State{Position: 8, Playing: true})
}

func TestLoopCrossfadeIsCappedAtTwoSeconds(t *testing.T) {
	m := newMixer(t, 1)
	samples := make([]Frame, 12)
	samples[1] = Frame{0.4, 0.4}
	samples[9], samples[10], samples[11] = Frame{0.6, 0.6}, Frame{0.8, 0.8}, Frame{1, 1}
	addPlaying(t, m, "music", samples)
	must(t, m.SetRepeating("music", true))
	must(t, m.Seek("music", 9))
	out := make([]Frame, 4)
	m.Render(out)
	assertFrames(t, out, []Frame{{0.6, 0.6}, {0.8, 0.8}, {0.7, 0.7}, {0, 0}})
	assertState(t, m, "music", State{Position: 3, Playing: true})
}

func TestSeekCancelsOverlapAndLateSeekShortensIt(t *testing.T) {
	m := newMixer(t, 3)
	samples := make([]Frame, 12)
	for i := range samples {
		samples[i] = Frame{float64(i) / 20, float64(i) / 20}
	}
	addPlaying(t, m, "music", samples)
	must(t, m.SetRepeating("music", true))
	must(t, m.Seek("music", 9))
	m.Render(make([]Frame, 2))
	must(t, m.Seek("music", 11))
	out := make([]Frame, 2)
	m.Render(out)
	assertFrames(t, out, []Frame{{0.55, 0.55}, {0.05, 0.05}})
	must(t, m.Seek("music", 9))
	m.Render(make([]Frame, 1))
	must(t, m.SetRepeating("music", false))
	out = make([]Frame, 3)
	m.Render(out)
	assertFrames(t, out, []Frame{{0.5, 0.5}, {0.55, 0.55}, {0, 0}})
}

func TestSeekCrossfadesOutgoingAndIncomingPCMForTwoSeconds(t *testing.T) {
	for _, repeating := range []bool{false, true} {
		t.Run(fmt.Sprint(repeating), func(t *testing.T) {
			m := newMixer(t, 2)
			samples := make([]Frame, 80)
			for i := range samples {
				samples[i] = Frame{Left: float64(i) / 100, Right: -float64(i) / 100}
			}
			addPlaying(t, m, "music", samples)
			must(t, m.SetRepeating("music", repeating))
			must(t, m.Seek("music", 20))
			must(t, m.CrossfadeSeek("music", 40))
			assertState(t, m, "music", State{Position: 36, Playing: true})
			out := make([]Frame, 5)
			m.Render(out)
			assertFrames(t, out, []Frame{
				{0.20, -0.20}, {0.25, -0.25}, {0.30, -0.30}, {0.35, -0.35}, {0.40, -0.40},
			})
			assertState(t, m, "music", State{Position: 41, Playing: true})
		})
	}
}

func TestSeekNearBeginningCrossfadesTheFullOpening(t *testing.T) {
	for _, tc := range []struct {
		start, target, outgoing int
		want                    []Frame
	}{
		{0, 0, 20, []Frame{{0.20, 0}, {0.16, 0}, {0.12, 0}, {0.08, 0}, {0.04, 0}}},
		{0, 2, 20, []Frame{{0.20, 0}, {0.16, 0}, {0.12, 0}, {0.08, 0}, {0.04, 0}}},
		{0, 4, 20, []Frame{{0.20, 0}, {0.16, 0}, {0.12, 0}, {0.08, 0}, {0.04, 0}}},
		{0, 2, 2, []Frame{{0.02, 0}, {0.025, 0}, {0.03, 0}, {0.035, 0}, {0.04, 0}}},
		{10, 10, 30, []Frame{{0.30, 0}, {0.26, 0}, {0.22, 0}, {0.18, 0}, {0.14, 0}}},
		{10, 12, 30, []Frame{{0.30, 0}, {0.26, 0}, {0.22, 0}, {0.18, 0}, {0.14, 0}}},
		{10, 14, 30, []Frame{{0.30, 0}, {0.26, 0}, {0.22, 0}, {0.18, 0}, {0.14, 0}}},
	} {
		t.Run(fmt.Sprint(tc.start, "/", tc.target, "/", tc.outgoing), func(t *testing.T) {
			m := newMixer(t, 2)
			samples := make([]Frame, 80)
			for i := range samples {
				samples[i].Left = float64(i) / 100
			}
			addPlaying(t, m, "music", samples)
			must(t, m.SetRegion("music", Region{Start: tc.start, End: 80, LoopBack: tc.start}))
			must(t, m.Seek("music", tc.outgoing))
			must(t, m.CrossfadeSeek("music", tc.target))
			assertState(t, m, "music", State{Position: tc.start, Playing: true})
			out := make([]Frame, len(tc.want))
			m.Render(out)
			assertFrames(t, out, tc.want)
			assertState(t, m, "music", State{Position: tc.start + 5, Playing: true})
		})
	}
}

func TestSeekNearBeginningKeepsTheOpeningWhenTheTailIsShort(t *testing.T) {
	m := newMixer(t, 2)
	samples := make([]Frame, 80)
	for i := range samples {
		samples[i].Left = float64(i) / 100
	}
	addPlaying(t, m, "music", samples)
	must(t, m.Seek("music", 78))
	must(t, m.CrossfadeSeek("music", 4))
	out := make([]Frame, 3)
	m.Render(out)
	assertFrames(t, out, []Frame{{Left: 0.78}, {Left: 0.40}, {Left: 0.02}})
	assertState(t, m, "music", State{Position: 3, Playing: true})
}

func TestSeekNearBeginningKeepsTheOpeningWhilePCMIsBuffered(t *testing.T) {
	for _, target := range []int{0, 2, 4} {
		t.Run(fmt.Sprint(target), func(t *testing.T) {
			m := newMixer(t, 2)
			source := &delayedSource{frame: Frame{Left: 0.5}, readyFrom: 8}
			must(t, m.AddSource("music", source))
			must(t, m.Play("music"))
			must(t, m.Seek("music", 8))
			must(t, m.CrossfadeSeek("music", target))
			out := make([]Frame, 2)
			m.Render(out)
			assertFrames(t, out, constantFrames(2, Frame{Left: 0.5}))
			assertState(t, m, "music", State{Position: 0, Playing: true})
			source.ready = true
			m.Render(out)
			assertFrames(t, out, constantFrames(2, Frame{Left: 0.5}))
			assertState(t, m, "music", State{Position: 2, Playing: true})
		})
	}
}

func TestSeekCrossfadeShortensAtBoundariesAndPausedSeeksStaySilent(t *testing.T) {
	m := newMixer(t, 2)
	addPlaying(t, m, "music", constantFrames(20, Frame{Left: 0.5}))
	must(t, m.Seek("music", 18))
	must(t, m.CrossfadeSeek("music", 4))
	m.Render(make([]Frame, 2))
	assertState(t, m, "music", State{Position: 2, Playing: true})
	must(t, m.CrossfadeSeek("music", 19))
	m.Render(make([]Frame, 4))
	assertState(t, m, "music", State{Position: 16, Playing: true})
	must(t, m.Pause("music"))
	must(t, m.CrossfadeSeek("music", 10))
	out := make([]Frame, 4)
	m.Render(out)
	assertFrames(t, out, make([]Frame, 4))
	assertState(t, m, "music", State{Position: 10})
}

func TestSeekDuringCrossfadeReplacesThePreviousTail(t *testing.T) {
	m := newMixer(t, 2)
	samples := make([]Frame, 80)
	for i := range samples {
		samples[i].Left = float64(i) / 100
	}
	addPlaying(t, m, "music", samples)
	must(t, m.CrossfadeSeek("music", 20))
	m.Render(make([]Frame, 2))
	must(t, m.CrossfadeSeek("music", 40))
	out := make([]Frame, 5)
	m.Render(out)
	assertFrames(t, out, []Frame{{Left: 0.18}, {Left: 0.235}, {Left: 0.29}, {Left: 0.345}, {Left: 0.40}})
	assertState(t, m, "music", State{Position: 41, Playing: true})
}

func TestSeekKeepsOutgoingPCMPlayingWhileTheDestinationBuffers(t *testing.T) {
	m := newMixer(t, 2)
	source := &delayedSource{frame: Frame{Left: 0.5}, readyUntil: 6}
	must(t, m.AddSource("music", source))
	must(t, m.Play("music"))
	must(t, m.CrossfadeSeek("music", 10))
	out := make([]Frame, 2)
	m.Render(out)
	assertFrames(t, out, []Frame{{Left: 0.5}, {Left: 0.5}})
	assertState(t, m, "music", State{Position: 6, Playing: true})
	source.ready = true
	out = make([]Frame, 5)
	m.Render(out)
	assertFrames(t, out, constantFrames(5, Frame{Left: 0.5}))
	assertState(t, m, "music", State{Position: 11, Playing: true})
}

func TestSeekLandingLeavesRoomForTheNaturalLoopCrossfade(t *testing.T) {
	m := newMixer(t, 2)
	addPlaying(t, m, "music", constantFrames(60, Frame{Left: 0.5}))
	must(t, m.SetRepeating("music", true))
	must(t, m.SetRegion("music", Region{End: 60, LoopBack: 10}))
	must(t, m.Seek("music", 20))
	must(t, m.CrossfadeSeek("music", 60))
	m.Render(make([]Frame, 4))
	assertState(t, m, "music", State{Position: 56, Playing: true})
	m.Render(make([]Frame, 4))
	assertState(t, m, "music", State{Position: 14, Playing: true})
	m.Render(make([]Frame, 2))
	assertState(t, m, "music", State{Position: 16, Playing: true})
}

func TestSeekLandingStaysInsideTrimmedAndShortRegions(t *testing.T) {
	for _, tc := range []struct {
		region Region
		target int
		want   int
	}{
		{Region{Start: 10, End: 30, LoopBack: 10}, 11, 14},
		{Region{Start: 10, End: 30, LoopBack: 10}, 10, 14},
		{Region{Start: 10, End: 30, LoopBack: 10}, 30, 26},
		{Region{Start: 10, End: 12, LoopBack: 10}, 12, 11},
	} {
		t.Run(fmt.Sprint(tc.region, tc.target), func(t *testing.T) {
			m := newMixer(t, 2)
			addPlaying(t, m, "music", constantFrames(40, Frame{Left: 0.5}))
			must(t, m.SetRegion("music", tc.region))
			must(t, m.Seek("music", min(tc.region.Start+8, tc.region.End-1)))
			must(t, m.CrossfadeSeek("music", tc.target))
			if overlap := m.tracks[0].overlap; overlap != nil {
				m.Render(make([]Frame, overlap.end-overlap.start))
			}
			assertState(t, m, "music", State{Position: tc.want, Playing: true})
			must(t, m.Pause("music"))
			must(t, m.CrossfadeSeek("music", tc.region.End))
			assertState(t, m, "music", State{Position: max(tc.region.Start, tc.region.End-4)})
		})
	}
}

func TestSubFrameCrossfadeLoopsWithoutOverlap(t *testing.T) {
	for _, samples := range [][]Frame{
		{{0.1, 0.2}}, {{0.1, 0.2}, {0.3, 0.4}},
	} {
		m := newMixer(t, 3)
		addPlaying(t, m, "music", samples)
		must(t, m.SetRepeating("music", true))
		out := make([]Frame, len(samples)*4)
		m.Render(out)
		for i := 0; i < 4; i++ {
			assertFrames(t, out[i*len(samples):(i+1)*len(samples)], samples)
		}
	}
}

func TestVolumeRampAndInterruptedAdjustment(t *testing.T) {
	m := newMixer(t, 2)
	addPlaying(t, m, "music", constantFrames(30, Frame{1, -1}))
	must(t, m.SetGain("music", 0))
	out := make([]Frame, 2)
	m.Render(out)
	assertFrames(t, out, []Frame{{1, -1}, {0.75, -0.75}})
	must(t, m.SetGain("music", 1))
	out = make([]Frame, 5)
	m.Render(out)
	assertFrames(t, out, []Frame{{0.5, -0.5}, {0.625, -0.625}, {0.75, -0.75}, {0.875, -0.875}, {1, -1}})
	must(t, m.SetGain("music", 0))
	m.Render(out)
	assertFrames(t, out, []Frame{{1, -1}, {0.75, -0.75}, {0.5, -0.5}, {0.25, -0.25}, {0, 0}})
}

func TestEffectiveGainAdvancesWhilePausedAndBeforeSummation(t *testing.T) {
	m := newMixer(t, 2)
	must(t, m.Add("music", constantFrames(10, Frame{1, 0.5})))
	must(t, m.Add("ambiance", constantFrames(10, Frame{0.8, -0.5})))
	must(t, m.SetGain("music", 0.5*0.5*0.5))
	out := make([]Frame, 4)
	m.Render(out)
	assertFrames(t, out, make([]Frame, 4))
	must(t, m.Play("music"))
	must(t, m.Play("ambiance"))
	m.Render(out[:1])
	assertFrames(t, out[:1], []Frame{{0.925, -0.4375}})
}

func TestRepeatingPlayPauseFadesAndFreezesAfterSilence(t *testing.T) {
	m := newMixer(t, 2)
	must(t, m.Add("music", constantFrames(30, Frame{1, -1})))
	must(t, m.SetRepeating("music", true))
	must(t, m.Play("music"))
	out := make([]Frame, 5)
	m.Render(out)
	assertFrames(t, out, []Frame{{0, 0}, {0.25, -0.25}, {0.5, -0.5}, {0.75, -0.75}, {1, -1}})
	must(t, m.Pause("music"))
	m.Render(out)
	assertFrames(t, out, []Frame{{1, -1}, {0.75, -0.75}, {0.5, -0.5}, {0.25, -0.25}, {0, 0}})
	assertState(t, m, "music", State{Position: 9})
	m.Render(out)
	assertFrames(t, out, make([]Frame, 5))
	assertState(t, m, "music", State{Position: 9})
}

func TestResumeAndDuplicateCommandsPreserveCurrentEnvelope(t *testing.T) {
	m := newMixer(t, 2)
	must(t, m.Add("music", constantFrames(30, Frame{1, 1})))
	must(t, m.SetRepeating("music", true))
	must(t, m.Play("music"))
	m.Render(make([]Frame, 2))
	must(t, m.Play("music"))
	out := make([]Frame, 3)
	m.Render(out)
	assertFrames(t, out, []Frame{{0.5, 0.5}, {0.75, 0.75}, {1, 1}})
	must(t, m.Pause("music"))
	m.Render(out[:2])
	must(t, m.Pause("music"))
	must(t, m.Play("music"))
	out = make([]Frame, 5)
	m.Render(out)
	assertFrames(t, out, []Frame{{0.5, 0.5}, {0.625, 0.625}, {0.75, 0.75}, {0.875, 0.875}, {1, 1}})
}

func TestVolumeAndPlaybackEnvelopesAreIndependent(t *testing.T) {
	m := newMixer(t, 2)
	must(t, m.Add("music", constantFrames(30, Frame{1, 1})))
	must(t, m.SetRepeating("music", true))
	must(t, m.Play("music"))
	must(t, m.SetGain("music", 0))
	out := make([]Frame, 5)
	m.Render(out)
	assertFrames(t, out, []Frame{{0, 0}, {0.1875, 0.1875}, {0.25, 0.25}, {0.1875, 0.1875}, {0, 0}})
}

func TestPauseCancelsLoopOverlap(t *testing.T) {
	m := newMixer(t, 1)
	samples := constantFrames(12, Frame{1, 1})
	samples[0], samples[1] = Frame{}, Frame{}
	addPlaying(t, m, "music", samples)
	must(t, m.SetRepeating("music", true))
	must(t, m.Seek("music", 10))
	m.Render(make([]Frame, 1))
	must(t, m.Pause("music"))
	out := make([]Frame, 3)
	m.Render(out)
	assertFrames(t, out, []Frame{{1, 1}, {0, 0}, {0, 0}})
	assertState(t, m, "music", State{Position: 0})
}

func TestDuplicateGainDoesNotRestartRamp(t *testing.T) {
	m := newMixer(t, 2)
	addPlaying(t, m, "music", constantFrames(20, Frame{1, 1}))
	must(t, m.SetGain("music", 0))
	m.Render(make([]Frame, 2))
	must(t, m.SetGain("music", 0))
	out := make([]Frame, 3)
	m.Render(out)
	assertFrames(t, out, []Frame{{0.5, 0.5}, {0.25, 0.25}, {0, 0}})
}

func TestDuplicateLoopSettingsPreserveOverlap(t *testing.T) {
	for name, update := range map[string]func(*Mixer) error{
		"region":    func(m *Mixer) error { return m.SetRegion("music", Region{End: 12}) },
		"repeating": func(m *Mixer) error { return m.SetRepeating("music", true) },
	} {
		t.Run(name, func(t *testing.T) {
			m := newMixer(t, 3)
			samples := make([]Frame, 12)
			for i := range samples {
				samples[i] = Frame{float64(i) / 20, float64(i) / 20}
			}
			addPlaying(t, m, "music", samples)
			must(t, m.SetRepeating("music", true))
			must(t, m.Seek("music", 9))
			m.Render(make([]Frame, 1))
			must(t, update(m))
			out := make([]Frame, 3)
			m.Render(out)
			assertFrames(t, out, []Frame{{0.35, 0.35}, {0.25, 0.25}, {0.15, 0.15}})
		})
	}
}

func TestRenderPartitionInvariant(t *testing.T) {
	render := func(blocks []int) ([]Frame, State) {
		m := newMixer(t, 6)
		samples := make([]Frame, 36)
		for i := range samples {
			samples[i] = Frame{math.Sin(float64(i)), math.Cos(float64(i))}
		}
		must(t, m.Add("music", samples))
		must(t, m.SetRegion("music", Region{Start: 2, End: 30, LoopBack: 8}))
		must(t, m.SetRepeating("music", true))
		must(t, m.Play("music"))
		addPlaying(t, m, "effect", constantFrames(45, Frame{0.4, -0.2}))
		var output []Frame
		pull := func(count int) {
			segment := make([]Frame, count)
			for offset, block := 0, 0; offset < count; block++ {
				m.Render(nil)
				end := min(count, offset+blocks[block%len(blocks)])
				m.Render(segment[offset:end])
				offset = end
			}
			output = append(output, segment...)
		}
		pull(37)
		must(t, m.SetGain("music", 0.3))
		pull(19)
		must(t, m.Pause("music"))
		pull(4)
		must(t, m.Play("music"))
		must(t, m.Seek("music", 20))
		pull(22)
		must(t, m.CrossfadeSeek("music", 10))
		pull(16)
		must(t, m.SetRegion("music", Region{Start: 4, End: 25, LoopBack: 8}))
		pull(17)
		must(t, m.SetRepeating("music", false))
		pull(50)
		state, err := m.State("music")
		must(t, err)
		return output, state
	}
	want, wantState := render([]int{300})
	for _, blocks := range [][]int{{1}, {1, 3, 7, 2, 11}} {
		got, gotState := render(blocks)
		assertFrames(t, got, want)
		if gotState != wantState || gotState != (State{Position: 4}) {
			t.Fatalf("partitioned state %+v, whole state %+v", gotState, wantState)
		}
	}
}

func TestInvalidInputsAndMissingTracks(t *testing.T) {
	for _, rate := range []int{-1, 0, math.MaxInt} {
		if _, err := New(rate); err == nil {
			t.Fatalf("accepted invalid sample rate %d", rate)
		}
	}
	m := newMixer(t, 6)
	for _, frame := range []Frame{
		{math.NaN(), 0}, {0, math.Inf(1)}, {math.Inf(-1), 0}, {1.1, 0}, {0, -1.1},
	} {
		if err := m.Add("invalid", []Frame{frame}); err == nil {
			t.Fatalf("accepted invalid PCM %+v", frame)
		}
	}
	if err := m.Add("", []Frame{{0, 0}}); err == nil {
		t.Fatal("accepted empty track ID")
	}
	if err := m.Add("empty", nil); err == nil {
		t.Fatal("accepted empty clip")
	}
	addPlaying(t, m, "music", constantFrames(10, Frame{0.5, 0.5}))
	if err := m.Add("music", []Frame{{1, 1}}); err == nil {
		t.Fatal("accepted duplicate track")
	}
	for _, gain := range []float64{math.NaN(), math.Inf(1), -0.1, 1.1} {
		if err := m.SetGain("music", gain); err == nil {
			t.Fatalf("accepted invalid gain %v", gain)
		}
	}
	for name, control := range map[string]func() error{
		"play":      func() error { return m.Play("missing") },
		"pause":     func() error { return m.Pause("missing") },
		"seek":      func() error { return m.Seek("missing", 0) },
		"seek fade": func() error { return m.CrossfadeSeek("missing", 0) },
		"remove":    func() error { return m.Remove("missing") },
		"region":    func() error { return m.SetRegion("missing", Region{End: 1}) },
		"repeating": func() error { return m.SetRepeating("missing", true) },
		"gain":      func() error { return m.SetGain("missing", 0.5) },
	} {
		if err := control(); err == nil {
			t.Errorf("%s accepted missing track", name)
		}
	}
	out := make([]Frame, 1)
	m.Render(out)
	assertFrames(t, out, []Frame{{0.5, 0.5}})
}

func constantFrames(count int, frame Frame) []Frame {
	frames := make([]Frame, count)
	for i := range frames {
		frames[i] = frame
	}
	return frames
}

func assertState(t *testing.T, m *Mixer, id string, want State) {
	t.Helper()
	got, err := m.State(id)
	must(t, err)
	if got != want {
		t.Fatalf("state: got %+v, want %+v", got, want)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func newMixer(t *testing.T, sampleRate int) *Mixer {
	t.Helper()
	m, err := New(sampleRate)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func addPlaying(t *testing.T, m *Mixer, id string, samples []Frame) {
	t.Helper()
	if err := m.Add(id, samples); err != nil {
		t.Fatal(err)
	}
	if err := m.Play(id); err != nil {
		t.Fatal(err)
	}
}

func assertFrames(t *testing.T, got, want []Frame) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d frames, want %d", len(got), len(want))
	}
	for i := range want {
		if math.Abs(got[i].Left-want[i].Left) > 1e-9 ||
			math.Abs(got[i].Right-want[i].Right) > 1e-9 {
			t.Fatalf("frame %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
}
