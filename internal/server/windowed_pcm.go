package server

import (
	"bufio"
	"container/list"
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/terrabitz/rpg-audio-streamer/internal/mixer"
)

const (
	pcmWindowSeconds = 5
	pcmDecodePadding = 0.1
	pcmCachedWindows = 4 // Two crossfading cursors, each with a current and lookahead window.
	pcmRequestQueue  = 16
)

type pcmWindow struct {
	index  int
	frames []mixer.Frame
}

type hlsSegment struct {
	path     string
	duration float64
}

type windowedPCMSource struct {
	segments       []hlsSegment
	sampleRate     int
	length         int
	windowFrames   int
	targetDuration int
	ctx            context.Context
	cancel         context.CancelFunc
	requests       chan int
	done           chan struct{}

	mu      sync.Mutex
	windows map[int]*list.Element
	lru     *list.List
	pending map[int]bool
	err     error
}

func newWindowedPCMSource(sampleRate int, length int, segments []hlsSegment) (*windowedPCMSource, error) {
	if sampleRate <= 0 || length <= 0 || len(segments) == 0 {
		return nil, fmt.Errorf("invalid PCM stream bounds")
	}
	targetDuration := 1
	for _, segment := range segments {
		targetDuration = max(targetDuration, int(math.Ceil(segment.duration)))
	}
	ctx, cancel := context.WithCancel(context.Background())
	source := &windowedPCMSource{
		segments: segments, sampleRate: sampleRate, length: length,
		windowFrames: sampleRate * pcmWindowSeconds, targetDuration: targetDuration,
		ctx: ctx, cancel: cancel, requests: make(chan int, pcmRequestQueue),
		done: make(chan struct{}), windows: make(map[int]*list.Element),
		lru: list.New(), pending: make(map[int]bool),
	}
	go source.run()
	return source, nil
}

func (s *windowedPCMSource) Len() int {
	return s.length
}

func (s *windowedPCMSource) FrameAt(position int) (mixer.Frame, bool) {
	if position < 0 || position >= s.length {
		return mixer.Frame{}, false
	}
	index := position / s.windowFrames
	s.mu.Lock()
	if element := s.windows[index]; element != nil {
		window := element.Value.(*pcmWindow)
		offset := position - index*s.windowFrames
		if offset < len(window.frames) {
			s.lru.MoveToFront(element)
			frame := window.frames[offset]
			s.mu.Unlock()
			if position%s.windowFrames >= s.windowFrames-s.sampleRate {
				s.requestWindow(index + 1)
			}
			return frame, true
		}
		s.err = fmt.Errorf("decoder window %d ended before frame %d", index, position)
	}
	s.mu.Unlock()

	s.requestWindow(index)
	if position%s.windowFrames >= s.windowFrames-s.sampleRate {
		s.requestWindow(index + 1)
	}
	return mixer.Frame{}, false
}

func (s *windowedPCMSource) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

func (s *windowedPCMSource) Close() error {
	s.cancel()
	<-s.done
	return nil
}

func (s *windowedPCMSource) run() {
	defer close(s.done)
	for {
		select {
		case <-s.ctx.Done():
			return
		case index := <-s.requests:
			frames, err := s.decodeWindow(index)
			s.mu.Lock()
			delete(s.pending, index)
			if err != nil && s.ctx.Err() == nil && s.err == nil {
				s.err = fmt.Errorf("decode PCM window %d: %w", index, err)
			} else if err == nil && s.ctx.Err() == nil {
				s.storeWindow(index, frames)
			}
			s.mu.Unlock()
		}
	}
}

func (s *windowedPCMSource) requestWindow(index int) {
	if index < 0 || index*s.windowFrames >= s.length {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil || s.ctx.Err() != nil || s.pending[index] || s.windows[index] != nil {
		return
	}
	s.pending[index] = true
	select {
	case s.requests <- index:
	default:
		delete(s.pending, index)
	}
}

func (s *windowedPCMSource) storeWindow(index int, frames []mixer.Frame) {
	window := &pcmWindow{index: index, frames: frames}
	if existing := s.windows[index]; existing != nil {
		existing.Value = window
		s.lru.MoveToFront(existing)
		return
	}
	s.windows[index] = s.lru.PushFront(window)
	if s.lru.Len() > pcmCachedWindows {
		oldest := s.lru.Back()
		delete(s.windows, oldest.Value.(*pcmWindow).index)
		s.lru.Remove(oldest)
	}
}

func (s *windowedPCMSource) decodeWindow(index int) ([]mixer.Frame, error) {
	start := index * s.windowFrames
	count := min(s.windowFrames, s.length-start)
	decodeSeconds := float64(count)/float64(s.sampleRate) + pcmDecodePadding
	manifest, offset, err := s.windowPlaylist(start, decodeSeconds)
	if err != nil {
		return nil, err
	}
	defer os.Remove(manifest)
	var stdout, stderr limitedWriter
	stdout.limit = int(count*8 + s.sampleRate*8)
	stderr.limit = 64 << 10
	cmd := exec.CommandContext(s.ctx, "ffmpeg", "-nostdin", "-v", "error",
		"-i", manifest,
		"-ss", strconv.FormatFloat(offset, 'f', 6, 64),
		"-t", strconv.FormatFloat(decodeSeconds, 'f', 6, 64),
		"-vn", "-ac", "2", "-ar", strconv.Itoa(s.sampleRate), "-f", "f32le", "-")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if s.ctx.Err() != nil {
			return nil, s.ctx.Err()
		}
		return nil, fmt.Errorf("%w: %s", err, stderr.String())
	}
	data := stdout.data
	if len(data) == 0 || len(data)%8 != 0 {
		return nil, fmt.Errorf("decoder returned empty or incomplete stereo PCM")
	}
	decodedFrames := len(data) / 8
	paddingFrames := int(math.Ceil(float64(s.sampleRate) * pcmDecodePadding))
	if decodedFrames < count-paddingFrames {
		return nil, fmt.Errorf("decoder returned %d of %d expected frames", decodedFrames, count)
	}
	frames := make([]mixer.Frame, count)
	for i := 0; i < min(count, decodedFrames); i++ {
		left := float64(math.Float32frombits(binary.LittleEndian.Uint32(data[i*8:])))
		right := float64(math.Float32frombits(binary.LittleEndian.Uint32(data[i*8+4:])))
		if math.IsNaN(left) || math.IsInf(left, 0) || math.IsNaN(right) || math.IsInf(right, 0) {
			return nil, fmt.Errorf("decoder returned non-finite PCM")
		}
		frames[i] = mixer.Frame{Left: max(-1, min(1, left)), Right: max(-1, min(1, right))}
	}
	return frames, nil
}

func (s *windowedPCMSource) windowPlaylist(start int, windowSeconds float64) (string, float64, error) {
	startSeconds := float64(start) / float64(s.sampleRate)
	elapsed := 0.0
	first := -1
	for index, segment := range s.segments {
		if startSeconds < elapsed+segment.duration {
			first = index
			break
		}
		elapsed += segment.duration
	}
	if first < 0 {
		return "", 0, fmt.Errorf("PCM window starts beyond HLS duration")
	}
	offset := startSeconds - elapsed
	file, err := os.CreateTemp("", "rpg-audio-window-*.m3u8")
	if err != nil {
		return "", 0, fmt.Errorf("create temporary HLS window: %w", err)
	}
	path := file.Name()
	writeErr := func() error {
		if _, err := fmt.Fprintf(file, "#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-TARGETDURATION:%d\n#EXT-X-MEDIA-SEQUENCE:%d\n", s.targetDuration, first); err != nil {
			return err
		}
		available := 0.0
		for _, segment := range s.segments[first:] {
			if _, err := fmt.Fprintf(file, "#EXTINF:%.6f,\n%s\n", segment.duration, filepath.Clean(segment.path)); err != nil {
				return err
			}
			available += segment.duration
			if available >= offset+windowSeconds {
				break
			}
		}
		_, err := fmt.Fprintln(file, "#EXT-X-ENDLIST")
		return err
	}()
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(path)
		if writeErr != nil {
			return "", 0, fmt.Errorf("write temporary HLS window: %w", writeErr)
		}
		return "", 0, fmt.Errorf("close temporary HLS window: %w", closeErr)
	}
	return path, offset, nil
}

func readHLSPlaylist(path string) ([]hlsSegment, float64, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("open audio playlist: %w", err)
	}
	defer file.Close()

	var segments []hlsSegment
	var pendingDuration float64
	hasDuration := false
	var duration float64
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#EXTINF:") {
			if hasDuration {
				return nil, 0, fmt.Errorf("HLS segment is missing its media path")
			}
			value, _, ok := strings.Cut(strings.TrimPrefix(line, "#EXTINF:"), ",")
			if !ok {
				value = strings.TrimPrefix(line, "#EXTINF:")
			}
			seconds, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
			if err != nil || seconds <= 0 || math.IsInf(seconds, 0) || math.IsNaN(seconds) {
				return nil, 0, fmt.Errorf("invalid HLS segment duration")
			}
			pendingDuration, hasDuration = seconds, true
			continue
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !hasDuration {
			return nil, 0, fmt.Errorf("HLS segment has no duration")
		}
		segmentPath := line
		if !filepath.IsAbs(segmentPath) {
			segmentPath = filepath.Join(filepath.Dir(path), segmentPath)
		}
		segmentPath, err = filepath.Abs(segmentPath)
		if err != nil {
			return nil, 0, fmt.Errorf("resolve HLS segment path: %w", err)
		}
		segments = append(segments, hlsSegment{path: segmentPath, duration: pendingDuration})
		duration += pendingDuration
		pendingDuration, hasDuration = 0, false
		if math.IsInf(duration, 0) {
			return nil, 0, fmt.Errorf("HLS playlist duration overflows")
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, 0, fmt.Errorf("read audio playlist: %w", err)
	}
	if hasDuration {
		return nil, 0, fmt.Errorf("HLS segment is missing its media path")
	}
	if duration <= 0 || len(segments) == 0 {
		return nil, 0, fmt.Errorf("HLS playlist has no audio segments")
	}
	return segments, duration, nil
}

type limitedWriter struct {
	data  []byte
	limit int
}

func (w *limitedWriter) Write(data []byte) (int, error) {
	if len(data) > w.limit-len(w.data) {
		return 0, fmt.Errorf("decoder diagnostics exceed %d byte limit", w.limit)
	}
	w.data = append(w.data, data...)
	return len(data), nil
}

func (w *limitedWriter) String() string {
	return string(w.data)
}
