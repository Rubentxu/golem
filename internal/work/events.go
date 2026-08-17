// Package work defines the Work bounded context: work items, their
// workflow and relations (BOUNDED_CONTEXTS). Events follow
// <context>.<entity>.<verb>.v<major> naming (EVENT_MODEL).
package work

// ItemCreated is the payload of work.item.created.v1. TypeName (optional)
// binds the item to a registered WorkType: Fields are then validated
// against the type schema and Status starts at the workflow initial
// state. Untyped items keep the legacy free-form behavior.
type ItemCreated struct {
	ItemID   string         `json:"item_id"`
	Title    string         `json:"title"`
	ItemType string         `json:"type"`
	TypeName string         `json:"type_name,omitempty"`
	Status   string         `json:"status"`
	Fields   map[string]any `json:"fields,omitempty"`
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
	EventItemCreated    = "work.item.created.v1"
	EventItemUpdated    = "work.item.updated.v1"
	EventItemLinked     = "work.item.linked.v1"
	EventTypeRegistered = "work.type.registered.v1"
)

// FieldDef is one custom field of a WorkType schema. Type is one of
// "string", "number", "bool".
type FieldDef struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
}

// Transition is one allowed status change of a workflow.
type Transition struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// TypeRegistered is the payload of work.type.registered.v1: the full
// schema + workflow definition of a WorkType (Tuleap-tracker analogue).
// Re-registration upserts the definition.
type TypeRegistered struct {
	Name        string       `json:"name"`
	Initial     string       `json:"initial"`
	States      []string     `json:"states"`
	Transitions []Transition `json:"transitions"`
	Fields      []FieldDef   `json:"fields"`
}
