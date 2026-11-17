package musicprocessing

import (
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
)

type Track struct {
	ID              uuid.UUID        `json:"id,omitempty"`
	CreatedAt       time.Time        `json:"createdAt,omitempty"`
	Name            string           `json:"name,omitempty"`
	Path            string           `json:"path,omitempty"`
	TypeID          uuid.UUID        `json:"typeID,omitempty"`
	Playback        PlaybackSettings `json:"playback"`
	Status          string           `json:"status,omitempty"`
	ProcessingError string           `json:"processingError,omitempty"`
}

func (t Track) Ready() bool {
	return t.Status == "" || t.Status == "ready"
}

type PlaybackSettings struct {
	StartTime    float64  `json:"startTime"`
	EndTime      *float64 `json:"endTime"`
	LoopBackTime *float64 `json:"loopBackTime"`
}

func (p PlaybackSettings) Validate() error {
	if math.IsNaN(p.StartTime) || math.IsInf(p.StartTime, 0) || p.StartTime < 0 {
		return fmt.Errorf("beginning must be a finite, nonnegative time")
	}
	if p.EndTime != nil && (math.IsNaN(*p.EndTime) || math.IsInf(*p.EndTime, 0) || *p.EndTime <= p.StartTime) {
		return fmt.Errorf("end must be after the beginning")
	}
	if p.LoopBackTime != nil && (math.IsNaN(*p.LoopBackTime) || math.IsInf(*p.LoopBackTime, 0) ||
		*p.LoopBackTime < p.StartTime || (p.EndTime != nil && *p.LoopBackTime >= *p.EndTime)) {
		return fmt.Errorf("loop-back point must be at or after the beginning and before the end")
	}
	return nil
}
