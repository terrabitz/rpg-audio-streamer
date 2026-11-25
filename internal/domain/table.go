package domain

import (
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

var DefaultTableID = uuid.MustParse("00000000-0000-4000-8000-000000000001")

var (
	ErrInvalidTableName     = Invalid("Table name must contain 1 to 200 characters")
	ErrInvalidTableSettings = Invalid("Invalid table settings")
)

// Table is a game session. Players reach it through its rotatable ExternalID link.
type Table struct {
	ID         uuid.UUID `json:"id"`
	ExternalID string    `json:"externalID"`
	Name       string    `json:"name"`
}

type TableSettings struct {
	MasterVolume float64            `json:"masterVolume"`
	TypeVolumes  map[string]float64 `json:"typeVolumes"`
	TrackOrder   []uuid.UUID        `json:"trackOrder"`
}

type TableSettingsPatch struct {
	MasterVolume *float64           `json:"masterVolume,omitempty"`
	TypeVolumes  map[string]float64 `json:"typeVolumes,omitempty"`
	TrackOrder   *[]uuid.UUID       `json:"trackOrder,omitempty"`
}

// ParseTableName trims a requested table name and enforces its length.
func ParseTableName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" || utf8.RuneCountInString(name) > 200 {
		return "", ErrInvalidTableName
	}
	return name, nil
}

func (p TableSettingsPatch) Validate() error {
	if p.MasterVolume != nil && (*p.MasterVolume < 0 || *p.MasterVolume > 200) {
		return ErrInvalidTableSettings
	}
	for name, volume := range p.TypeVolumes {
		if strings.TrimSpace(name) == "" || volume < 0 || volume > 100 {
			return ErrInvalidTableSettings
		}
	}
	if p.TrackOrder != nil {
		seen := make(map[uuid.UUID]bool, len(*p.TrackOrder))
		for _, id := range *p.TrackOrder {
			if id == uuid.Nil || seen[id] {
				return ErrInvalidTableSettings
			}
			seen[id] = true
		}
	}
	return nil
}
