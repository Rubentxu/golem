// Package work defines the Work bounded context: work items, their
// workflow and relations (BOUNDED_CONTEXTS). Events follow
// <context>.<entity>.<verb>.v<major> naming (EVENT_MODEL).
package work

// ItemCreated is the payload of work.item.created.v1.
type ItemCreated struct {
	ItemID   string `json:"item_id"`
	Title    string `json:"title"`
	ItemType string `json:"type"`
	Status   string `json:"status"`
}

// ItemUpdated is the payload of work.item.updated.v1. Nil fields are
// "not changed", enabling additive schema evolution within a major.
type ItemUpdated struct {
	ItemID string  `json:"item_id"`
	Title  *string `json:"title,omitempty"`
	Status *string `json:"status,omitempty"`
}

// ItemLinked is the payload of work.item.linked.v1. Relation is a
// canonical uppercase relation name (e.g. DEPENDS_ON, IMPLEMENTS).
type ItemLinked struct {
	FromID   string `json:"from_id"`
	ToID     string `json:"to_id"`
	Relation string `json:"relation"`
}

// Event type names of this context.
const (
	EventItemCreated = "work.item.created.v1"
	EventItemUpdated = "work.item.updated.v1"
	EventItemLinked  = "work.item.linked.v1"
)
