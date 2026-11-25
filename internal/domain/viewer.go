package domain

import "github.com/google/uuid"

// Viewer is whoever performs a use case: the GM, or a player limited to one table.
// The zero value is a player without table access.
type Viewer struct {
	gm      bool
	tableID uuid.UUID
}

func GMViewer() Viewer {
	return Viewer{gm: true}
}

func PlayerViewer(tableID uuid.UUID) Viewer {
	return Viewer{tableID: tableID}
}

func (v Viewer) IsGM() bool {
	return v.gm
}

// TableID is the only table a player may access; it is empty for the GM.
func (v Viewer) TableID() uuid.UUID {
	return v.tableID
}

func (v Viewer) CanAccessTable(tableID uuid.UUID) bool {
	return v.gm || (v.tableID != uuid.Nil && v.tableID == tableID)
}
