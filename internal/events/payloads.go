package events

import "github.com/terrabitz/rpg-audio-streamer/internal/domain"

type Empty struct{}

type PlaybackPatch struct {
	FileID      string                   `json:"fileID"`
	IsPlaying   *bool                    `json:"isPlaying,omitempty"`
	IsRepeating *bool                    `json:"isRepeating,omitempty"`
	Volume      *float64                 `json:"volume,omitempty"`
	CurrentTime *float64                 `json:"currentTime,omitempty"`
	Duration    *float64                 `json:"duration,omitempty"`
	Playback    *domain.PlaybackSettings `json:"playback,omitempty"`
}

type PlaybackTrack struct {
	FileID          string                  `json:"fileID"`
	Name            string                  `json:"name"`
	IsPlaying       bool                    `json:"isPlaying"`
	Volume          float64                 `json:"volume"`
	IsRepeating     bool                    `json:"isRepeating"`
	CurrentTime     float64                 `json:"currentTime"`
	Duration        float64                 `json:"duration"`
	TrackType       string                  `json:"trackType"`
	Playback        domain.PlaybackSettings `json:"playback"`
	PositionVersion uint64                  `json:"positionVersion"`
	SeekVersion     uint64                  `json:"seekVersion"`
	SeekTarget      *float64                `json:"seekTarget"`
	SeekLanding     *float64                `json:"seekLanding"`
}

type Snapshot struct {
	Tracks   []PlaybackTrack       `json:"tracks"`
	Settings *domain.TableSettings `json:"settings,omitempty"`
}

type MixSnapshot struct {
	Tracks      []PlaybackPatch    `json:"tracks"`
	TypeVolumes map[string]float64 `json:"typeVolumes,omitempty"`
}

type MixVolumePatch struct {
	TypeVolumes map[string]float64 `json:"typeVolumes"`
}

type TrackStatusUpdate struct {
	FileID          string `json:"fileID"`
	Status          string `json:"status"`
	ProcessingError string `json:"processingError,omitempty"`
}

type Failure struct {
	Message string `json:"message"`
}
