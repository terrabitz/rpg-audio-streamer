package server

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/terrabitz/rpg-audio-streamer/internal/mixer"
)

func TestMixerPCMStreamsBoundedHLSWindowsAndSeeks(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup(t)
	track := saveWaveformTrack(t, ts)
	playlist := filepath.Join(track.Path, "index.m3u8")
	cmd := exec.Command("ffmpeg", "-nostdin", "-v", "error",
		"-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000:duration=5",
		"-f", "lavfi", "-i", "sine=frequency=550:sample_rate=48000:duration=5",
		"-f", "lavfi", "-i", "sine=frequency=660:sample_rate=48000:duration=5",
		"-f", "lavfi", "-i", "sine=frequency=770:sample_rate=48000:duration=5",
		"-filter_complex", "[0:a][1:a][2:a][3:a]concat=n=4:v=0:a=1[a]", "-map", "[a]",
		"-c:a", "aac", "-f", "hls", "-hls_time", "2", "-hls_list_size", "0",
		"-hls_segment_filename", filepath.Join(track.Path, "segment%03d.ts"), playlist)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generate HLS: %v: %s", err, output)
	}

	segments, duration, err := readHLSPlaylist(playlist)
	if err != nil {
		t.Fatal(err)
	}
	if duration < 19 || duration > 21 {
		t.Fatalf("playlist duration = %v, want about 20 seconds", duration)
	}
	source, err := newWindowedPCMSource(mixerSampleRate, int(math.Round(duration*mixerSampleRate)), segments)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()

	for _, sample := range []struct {
		start, frequency int
	}{
		{start: mixerSampleRate / 2, frequency: 440},
		{start: 6*mixerSampleRate + mixerSampleRate/2, frequency: 550},
		{start: 11*mixerSampleRate + mixerSampleRate/2, frequency: 660},
		{start: 16*mixerSampleRate + mixerSampleRate/2, frequency: 770},
	} {
		frequency, err := waitForPCMFrequency(source, sample.start)
		if err != nil {
			t.Fatalf("decode seek at %.1f seconds: %v", float64(sample.start)/mixerSampleRate, err)
		}
		if math.Abs(frequency-float64(sample.frequency)) > 25 {
			t.Fatalf("decoded %.1f Hz near %.1f seconds, want %d Hz", frequency, float64(sample.start)/mixerSampleRate, sample.frequency)
		}
	}
	source.mu.Lock()
	cached := len(source.windows)
	source.mu.Unlock()
	if cached > pcmCachedWindows {
		t.Fatalf("cached %d windows, limit is %d", cached, pcmCachedWindows)
	}
	for _, seconds := range []int{0, 5, 10, 15} {
		if _, ready := source.FrameAt(seconds*mixerSampleRate + mixerSampleRate/2); !ready {
			t.Fatal("two crossfading cursors and their lookahead windows cannot stay decoded together")
		}
	}

	longPlaylist := filepath.Join(track.Path, "long.m3u8")
	var longManifest strings.Builder
	longManifest.WriteString("#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-TARGETDURATION:3\n")
	for i := 0; i < 1800; i++ {
		fmt.Fprintf(&longManifest, "#EXTINF:%.6f,\n%s\n", segments[0].duration, segments[0].path)
	}
	longManifest.WriteString("#EXT-X-ENDLIST\n")
	if err := os.WriteFile(longPlaylist, []byte(longManifest.String()), 0600); err != nil {
		t.Fatal(err)
	}
	longSegments, longDuration, err := readHLSPlaylist(longPlaylist)
	if err != nil {
		t.Fatal(err)
	}
	if longDuration < 3600 {
		t.Fatalf("long HLS fixture duration = %v, want at least one hour", longDuration)
	}
	longSource, err := newWindowedPCMSource(mixerSampleRate, int(math.Round(longDuration*mixerSampleRate)), longSegments)
	if err != nil {
		t.Fatal(err)
	}
	defer longSource.Close()
	longPosition := 3595*mixerSampleRate + mixerSampleRate/2
	if frequency, err := waitForPCMFrequency(longSource, longPosition); err != nil {
		t.Fatalf("decode one-hour seek: %v", err)
	} else if math.Abs(frequency-440) > 25 {
		t.Fatalf("one-hour seek decoded %.1f Hz, want 440 Hz", frequency)
	}
	longSource.mu.Lock()
	longCachedFrames := 0
	for _, element := range longSource.windows {
		longCachedFrames += len(element.Value.(*pcmWindow).frames)
	}
	longSource.mu.Unlock()
	if longCachedFrames > pcmWindowSeconds*mixerSampleRate {
		t.Fatalf("one-hour source cached %d frames, over one window", longCachedFrames)
	}

	engine, err := mixer.New(mixerSampleRate)
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.AddSource("loop", source); err != nil {
		t.Fatal(err)
	}
	source.mu.Lock()
	if head := source.windows[0]; head != nil {
		delete(source.windows, 0)
		source.lru.Remove(head)
	}
	source.mu.Unlock()
	if err := engine.SetRegion("loop", mixer.Region{End: 20 * mixerSampleRate}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Seek("loop", 18*mixerSampleRate); err != nil {
		t.Fatal(err)
	}
	if err := engine.Play("loop"); err != nil {
		t.Fatal(err)
	}
	if err := engine.SetRepeating("loop", true); err != nil {
		t.Fatal(err)
	}
	coldLoop := make([]mixer.Frame, mixerSampleRate/10)
	engine.Render(coldLoop)
	energy := 0.0
	for _, frame := range coldLoop {
		energy += frame.Left * frame.Left
	}
	if energy/float64(len(coldLoop)) < 0.001 {
		t.Fatal("cold loop-back window silenced the decoded outgoing tail")
	}
	if err := engine.SetRegion("loop", mixer.Region{
		Start: 0, End: 20 * mixerSampleRate, LoopBack: 10 * mixerSampleRate,
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SetRepeating("loop", true); err != nil {
		t.Fatal(err)
	}
	if err := engine.Seek("loop", 18*mixerSampleRate); err != nil {
		t.Fatal(err)
	}
	if err := engine.Play("loop"); err != nil {
		t.Fatal(err)
	}
	fade := 2 * mixerSampleRate
	out := make([]mixer.Frame, fade+mixerSampleRate/10)
	engine.Render(out)
	frequency := estimateFrequency(out[fade:])
	if math.Abs(frequency-660) > 25 {
		t.Fatalf("loop resumed at %.1f Hz, want loop-back audio at 660 Hz", frequency)
	}
	engine.Close()
}

func TestMixerPCMStreamsUploadedHLS(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup(t)
	track := saveWaveformTrack(t, ts)
	input := filepath.Join(ts.tempDir, "tone.mp3")
	if output, err := exec.Command("ffmpeg", "-nostdin", "-v", "error",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=12", "-f", "mp3", input).CombinedOutput(); err != nil {
		t.Fatalf("generate upload audio: %v: %s", err, output)
	}
	playlist := filepath.Join(track.Path, "index.m3u8")
	if output, err := exec.Command("ffmpeg", "-nostdin", "-v", "error", "-i", input,
		"-c:a", "aac", "-b:a", "128k", "-ac", "2", "-ar", "44100", "-hls_time", "6",
		"-hls_playlist_type", "event", "-hls_segment_filename", filepath.Join(track.Path, "segment_%03d.ts"),
		"-vn", "-f", "hls", playlist).CombinedOutput(); err != nil {
		t.Fatalf("generate uploaded HLS: %v: %s", err, output)
	}
	segments, duration, err := readHLSPlaylist(playlist)
	if err != nil {
		t.Fatal(err)
	}
	source, err := newWindowedPCMSource(mixerSampleRate, int(math.Round(duration*mixerSampleRate)), segments)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	frequency, err := waitForPCMFrequency(source, mixerSampleRate/2)
	if err != nil {
		t.Fatalf("decode uploaded HLS: %v", err)
	}
	if math.Abs(frequency-440) > 25 {
		t.Fatalf("uploaded HLS decoded %.1f Hz, want 440 Hz", frequency)
	}
}

func waitForPCMFrequency(source *windowedPCMSource, position int) (float64, error) {
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if source.Err() != nil {
			return 0, source.Err()
		}
		_, ready := source.FrameAt(position)
		if ready {
			crossings := 0
			previous := 0.0
			for i := 0; i < mixerSampleRate/10; i++ {
				frame, ok := source.FrameAt(position + i)
				if !ok {
					return 0, context.DeadlineExceeded
				}
				if i > 0 && (frame.Left < 0) != (previous < 0) {
					crossings++
				}
				previous = frame.Left
			}
			return float64(crossings) * 5, nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return 0, context.DeadlineExceeded
}

func estimateFrequency(frames []mixer.Frame) float64 {
	crossings := 0
	for i := 1; i < len(frames); i++ {
		if (frames[i].Left < 0) != (frames[i-1].Left < 0) {
			crossings++
		}
	}
	return float64(crossings) / (2 * float64(len(frames)) / mixerSampleRate)
}
