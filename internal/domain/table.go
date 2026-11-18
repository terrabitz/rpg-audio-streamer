package domain

import "github.com/google/uuid"

type TableSettings struct {
	MasterVolume float64            `json:"masterVolume"`
	TypeVolumes  map[string]float64 `json:"typeVolumes"`
	TrackVolumes map[string]float64 `json:"trackVolumes"`
	TrackOrder   []uuid.UUID        `json:"trackOrder"`
}
