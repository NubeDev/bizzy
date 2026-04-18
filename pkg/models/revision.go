package models

import (
	"encoding/json"
	"time"
)

// Revision stores a snapshot of any entity (tool, prompt, permissions, etc.)
// before it is modified. This provides a generic undo/history mechanism.
type Revision struct {
	ID           string          `json:"id" gorm:"primaryKey"`
	EntityType   string          `json:"entityType" gorm:"index:idx_rev_entity"`   // "tool", "prompt", "permissions", ...
	EntityID     string          `json:"entityId" gorm:"index:idx_rev_entity"`     // composite key, e.g. "appId:toolName"
	Revision     int             `json:"revision" gorm:"index:idx_rev_entity"`     // auto-incremented per entity
	Data         json.RawMessage `json:"data" gorm:"type:text"`                    // full JSON snapshot
	ChangeSummary string         `json:"changeSummary"`                             // human-readable: "AI edit: added humidity"
	AuthorID     string          `json:"authorId"`
	CreatedAt    time.Time       `json:"createdAt"`
}
