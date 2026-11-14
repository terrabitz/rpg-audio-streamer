// Package livemixer owns table-scoped PCM engines and applies live GM commands.
package livemixer

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/terrabitz/rpg-audio-streamer/internal/mixer"
)

type Playback struct {
	StartTime    float64  `json:"startTime"`
	EndTime      *float64 `json:"endTime"`
	LoopBackTime *float64 `json:"loopBackTime"`
}

type Clip struct {
	Frames    []mixer.Frame
	Type      string
	Repeating bool
	Playback  Playback
}

type Source func(context.Context, string, string) (Clip, error)

type Patch struct {
	FileID      string    `json:"fileID"`
	IsPlaying   *bool     `json:"isPlaying"`
	IsRepeating *bool     `json:"isRepeating"`
	CurrentTime *float64  `json:"currentTime"`
	Volume      *float64  `json:"volume"`
	Playback    *Playback `json:"playback"`
}

type TrackStatus struct {
	FileID      string  `json:"fileID"`
	CurrentTime float64 `json:"currentTime"`
	IsPlaying   bool    `json:"isPlaying"`
	Gain        float64 `json:"gain"`
}

type Status struct {
	Revision       uint64        `json:"revision"`
	FramesRendered uint64        `json:"framesRendered"`
	Peak           float64       `json:"peak"`
	Tracks         []TrackStatus `json:"tracks"`
}

// Commands are serialized separately from rendering so decoding never holds the PCM lock.
type Service struct {
	commands    sync.Mutex
	mu          sync.Mutex
	sampleRate  int
	source      Source
	tables      map[string]*session
	generations map[string]uint64
	listeners   map[string]map[*Subscription]bool
}

type session struct {
	sampleRate  int
	engine      *mixer.Mixer
	tracks      map[string]track
	typeVolumes map[string]float64
	status      Status
	buffer      []mixer.Frame
}

type track struct {
	clip      Clip
	region    mixer.Region
	volume    float64
	repeating bool
}

type change struct {
	patch Patch
	track track
}

func New(sampleRate int, source Source) (*Service, error) {
	if _, err := mixer.New(sampleRate); err != nil {
		return nil, err
	}
	if source == nil {
		return nil, fmt.Errorf("PCM source is required")
	}
	return &Service{
		sampleRate: sampleRate, source: source,
		tables: make(map[string]*session), generations: make(map[string]uint64),
		listeners: make(map[string]map[*Subscription]bool),
	}, nil
}

func (s *Service) Update(tableID string, payload json.RawMessage) error {
	generation := s.generation(tableID)
	s.commands.Lock()
	defer s.commands.Unlock()
	var patch Patch
	if err := json.Unmarshal(payload, &patch); err != nil {
		return fmt.Errorf("decode track command: %w", err)
	}
	change, err := s.prepare(tableID, patch)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.generations[tableID] != generation {
		return fmt.Errorf("mix session closed while loading audio")
	}
	session := s.get(tableID)
	if err := session.apply(change); err != nil {
		return err
	}
	session.status.Revision++
	return nil
}

func (s *Service) Sync(tableID string, payload json.RawMessage) error {
	return s.SyncContext(context.Background(), tableID, payload)
}

func (s *Service) SyncContext(ctx context.Context, tableID string, payload json.RawMessage) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	generation := s.generation(tableID)
	s.commands.Lock()
	defer s.commands.Unlock()
	if tableID == "" {
		return fmt.Errorf("table ID is required")
	}
	var snapshot struct {
		Tracks      *[]Patch           `json:"tracks"`
		TypeVolumes map[string]float64 `json:"typeVolumes"`
	}
	if err := json.Unmarshal(payload, &snapshot); err != nil {
		return fmt.Errorf("decode mix snapshot: %w", err)
	}
	if snapshot.Tracks == nil {
		return fmt.Errorf("mix snapshot requires tracks")
	}
	if err := validateVolumes(snapshot.TypeVolumes); err != nil {
		return err
	}
	changes := make([]change, 0, len(*snapshot.Tracks))
	seen := make(map[string]bool)
	for _, patch := range *snapshot.Tracks {
		if seen[patch.FileID] {
			return fmt.Errorf("duplicate track %q in snapshot", patch.FileID)
		}
		seen[patch.FileID] = true
		change, err := s.prepare(tableID, patch)
		if err != nil {
			return err
		}
		changes = append(changes, change)
	}
	replacement := s.newSession()
	replacement.typeVolumes = snapshot.TypeVolumes
	for _, change := range changes {
		if err := replacement.apply(change); err != nil {
			return err
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.generations[tableID] != generation {
		return fmt.Errorf("mix session closed while loading audio")
	}
	if old := s.tables[tableID]; old != nil {
		replacement.status = old.status
	}
	replacement.status.Revision++
	replacement.status.Peak = 0
	s.tables[tableID] = replacement
	return nil
}

func (s *Service) Volumes(tableID string, payload json.RawMessage) error {
	generation := s.generation(tableID)
	s.commands.Lock()
	defer s.commands.Unlock()
	if tableID == "" {
		return fmt.Errorf("table ID is required")
	}
	var volumes struct {
		TypeVolumes map[string]float64 `json:"typeVolumes"`
	}
	if err := json.Unmarshal(payload, &volumes); err != nil {
		return fmt.Errorf("decode mix volumes: %w", err)
	}
	if volumes.TypeVolumes == nil {
		return fmt.Errorf("typeVolumes is required")
	}
	if err := validateVolumes(volumes.TypeVolumes); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.generations[tableID] != generation {
		return fmt.Errorf("mix session closed while applying volumes")
	}
	session := s.get(tableID)
	session.typeVolumes = volumes.TypeVolumes
	for id, track := range session.tracks {
		if err := session.engine.SetGain(id, session.gain(track)); err != nil {
			return err
		}
	}
	session.status.Revision++
	return nil
}

func (s *Service) RemoveTrack(tableID, fileID string) {
	s.commands.Lock()
	defer s.commands.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, session := range s.tables {
		if tableID != "" && id != tableID {
			continue
		}
		if _, exists := session.tracks[fileID]; !exists {
			continue
		}
		_ = session.engine.Remove(fileID) // The session map and engine are updated together.
		delete(session.tracks, fileID)
		session.status.Revision++
	}
}

func (s *Service) CloseTable(tableID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.generations[tableID]++
	s.closeStreams(tableID, ErrSessionClosed)
	delete(s.tables, tableID)
}

// Run advances active sessions in 20 ms PCM blocks until ctx is cancelled.
func (s *Service) Run(ctx context.Context) {
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	defer func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		for tableID := range s.listeners {
			s.closeStreams(tableID, ErrRendererStopped)
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.mu.Lock()
			for tableID, session := range s.tables {
				if session.buffer == nil {
					session.buffer = make([]mixer.Frame, max(1, s.sampleRate/50))
				}
				session.render(session.buffer)
				s.publish(tableID, session.buffer)
			}
			s.mu.Unlock()
		}
	}
}

// Render is also a deterministic offline driver for testing command timelines.
func (s *Service) Render(tableID string, count int) []mixer.Frame {
	s.mu.Lock()
	defer s.mu.Unlock()
	frames := make([]mixer.Frame, count)
	if session := s.tables[tableID]; session != nil {
		session.render(frames)
		s.publish(tableID, frames)
	}
	return frames
}

func (s *Service) Status(tableID string) Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	status := Status{Tracks: []TrackStatus{}}
	session := s.tables[tableID]
	if session == nil {
		return status
	}
	status.Revision = session.status.Revision
	status.FramesRendered = session.status.FramesRendered
	status.Peak = session.status.Peak
	ids := make([]string, 0, len(session.tracks))
	for id := range session.tracks {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		state, _ := session.engine.State(id)
		status.Tracks = append(status.Tracks, TrackStatus{
			FileID: id, CurrentTime: float64(state.Position) / float64(s.sampleRate),
			IsPlaying: state.Playing, Gain: session.gain(session.tracks[id]),
		})
	}
	return status
}

func (s *Service) prepare(tableID string, patch Patch) (change, error) {
	if tableID == "" || patch.FileID == "" {
		return change{}, fmt.Errorf("table and track IDs are required")
	}
	s.mu.Lock()
	session := s.tables[tableID]
	var current track
	var exists bool
	if session != nil {
		current, exists = session.tracks[patch.FileID]
	}
	s.mu.Unlock()
	if !exists {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		clip, err := s.source(ctx, tableID, patch.FileID)
		if err != nil {
			return change{}, fmt.Errorf("load track %q: %w", patch.FileID, err)
		}
		for _, frame := range clip.Frames {
			if !finite(frame.Left) || !finite(frame.Right) || math.Abs(frame.Left) > 1 || math.Abs(frame.Right) > 1 {
				return change{}, fmt.Errorf("source contains invalid PCM")
			}
		}
		current = track{clip: clip, volume: 100, repeating: clip.Repeating}
	}
	if patch.Volume != nil {
		if !finite(*patch.Volume) || *patch.Volume < 0 || *patch.Volume > 200 {
			return change{}, fmt.Errorf("volume must be between zero and 200")
		}
		current.volume = *patch.Volume
	}
	if patch.IsRepeating != nil {
		current.repeating = *patch.IsRepeating
	}
	region := current.region
	if !exists || patch.Playback != nil {
		playback := current.clip.Playback
		if patch.Playback != nil {
			playback = *patch.Playback
		}
		var err error
		region, err = s.region(playback, len(current.clip.Frames))
		if err != nil {
			return change{}, err
		}
	}
	current.region = region
	if patch.CurrentTime != nil {
		position := *patch.CurrentTime * float64(s.sampleRate)
		if !finite(position) || position < float64(region.Start) || position > float64(region.End) {
			return change{}, fmt.Errorf("seek must be within the playback region")
		}
	}
	return change{patch: patch, track: current}, nil
}

func (s *Service) region(playback Playback, length int) (mixer.Region, error) {
	rate := float64(s.sampleRate)
	end := float64(length) / rate
	if playback.EndTime != nil {
		end = *playback.EndTime
	}
	loop := playback.StartTime
	if playback.LoopBackTime != nil {
		loop = *playback.LoopBackTime
	}
	if !finite(playback.StartTime) || !finite(end) || !finite(loop) ||
		playback.StartTime < 0 || end > float64(length)/rate || end <= playback.StartTime ||
		loop < playback.StartTime || loop >= end {
		return mixer.Region{}, fmt.Errorf("invalid playback region")
	}
	region := mixer.Region{Start: int(playback.StartTime * rate), End: int(end * rate), LoopBack: int(loop * rate)}
	if region.End <= region.Start || region.LoopBack >= region.End {
		return mixer.Region{}, fmt.Errorf("playback region is shorter than one PCM frame")
	}
	return region, nil
}

func (s *Service) get(tableID string) *session {
	session := s.tables[tableID]
	if session == nil {
		session = s.newSession()
		s.tables[tableID] = session
	}
	return session
}

func (s *Service) generation(tableID string) uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.generations[tableID]
}

func (s *Service) newSession() *session {
	engine, _ := mixer.New(s.sampleRate) // New validated the service sample rate.
	return &session{sampleRate: s.sampleRate, engine: engine, tracks: make(map[string]track)}
}

func (s *session) apply(change change) error {
	id := change.patch.FileID
	if _, exists := s.tracks[id]; !exists {
		if err := s.engine.Add(id, change.track.clip.Frames); err != nil {
			return err
		}
	}
	if err := s.engine.SetRegion(id, change.track.region); err != nil {
		return err
	}
	if err := s.engine.SetRepeating(id, change.track.repeating); err != nil {
		return err
	}
	if err := s.engine.SetGain(id, s.gain(change.track)); err != nil {
		return err
	}
	if change.patch.CurrentTime != nil {
		if err := s.engine.Seek(id, int(*change.patch.CurrentTime*float64(s.sampleRate))); err != nil {
			return err
		}
	}
	if change.patch.IsPlaying != nil {
		var err error
		if *change.patch.IsPlaying {
			err = s.engine.Play(id)
		} else {
			err = s.engine.Pause(id)
		}
		if err != nil {
			return err
		}
	}
	s.tracks[id] = change.track
	return nil
}

func (s *session) gain(track track) float64 {
	typeVolume, exists := s.typeVolumes[track.clip.Type]
	if !exists {
		typeVolume = 100
	}
	return min(1, track.volume/100*typeVolume/100)
}

func (s *session) render(frames []mixer.Frame) {
	s.engine.Render(frames)
	s.status.FramesRendered += uint64(len(frames))
	s.status.Peak = 0
	for _, frame := range frames {
		s.status.Peak = max(s.status.Peak, math.Abs(frame.Left), math.Abs(frame.Right))
	}
}

func validateVolumes(volumes map[string]float64) error {
	for name, volume := range volumes {
		if name == "" || !finite(volume) || volume < 0 || volume > 100 {
			return fmt.Errorf("type volumes must be between zero and 100")
		}
	}
	return nil
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
