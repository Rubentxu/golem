// Package work provides narrow-port adapters over the graph store.
package work

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Rubentxu/golem/internal/ports"
)

// graphStoreWorkItemReader implements WorkItemReader over a GraphStore.
type graphStoreWorkItemReader struct {
	gs ports.GraphStore
}

// NewWorkItemReaderOverGraphStore creates a WorkItemReader that delegates to gs.
func NewWorkItemReaderOverGraphStore(gs ports.GraphStore) WorkItemReader {
	return &graphStoreWorkItemReader{gs: gs}
}

// GetTypeDef implements WorkItemReader by looking up the WorkType node.
func (g *graphStoreWorkItemReader) GetTypeDef(ctx context.Context, tenant, typeName string) (WorkTypeDef, error) {
	n, err := g.gs.GetNode(ctx, ports.TenantID(tenant), typeName)
	if err != nil {
		if err == ports.ErrNodeNotFound {
			return WorkTypeDef{}, fmt.Errorf("%w: %s", ErrUnknownTypeName, typeName)
		}
		return WorkTypeDef{}, err
	}
	if n.Kind != "WorkType" {
		return WorkTypeDef{}, fmt.Errorf("%w: %s", ErrUnknownTypeName, typeName)
	}
	return typeFromNode(n), nil
}

// typeFromNode rebuilds a WorkTypeDef from a graph node's attributes.
func typeFromNode(n ports.Node) WorkTypeDef {
	def := WorkTypeDef{}
	if s, ok := n.Attributes["name"].(string); ok {
		def.Name = s
	}
	if s, ok := n.Attributes["initial"].(string); ok {
		def.Initial = s
	}

	// Helper to decode JSON RawMessage or []byte into a target value
	decodeRaw := func(v any, out any) bool {
		switch raw := v.(type) {
		case json.RawMessage:
			return json.Unmarshal(raw, out) == nil
		case []byte:
			return json.Unmarshal(raw, out) == nil
		default:
			return false
		}
	}

	// Decode states ([]string directly)
	var statesRaw any
	if decodeRaw(n.Attributes["states"], &statesRaw) {
		if states, ok := statesRaw.([]any); ok {
			for _, s := range states {
				if str, ok := s.(string); ok {
					def.States = append(def.States, str)
				}
			}
		}
	} else if states, ok := n.Attributes["states"].([]any); ok {
		for _, s := range states {
			if str, ok := s.(string); ok {
				def.States = append(def.States, str)
			}
		}
	}

	// Decode transitions: []{"from":"A","to":"B"} → ["A→B"]
	var transitionsRaw any
	if decodeRaw(n.Attributes["transitions"], &transitionsRaw) {
		if transitions, ok := transitionsRaw.([]any); ok {
			for _, t := range transitions {
				if m, ok := t.(map[string]any); ok {
					from, _ := m["from"].(string)
					to, _ := m["to"].(string)
					if from != "" && to != "" {
						def.Transitions = append(def.Transitions, from+"→"+to)
					}
				}
			}
		}
	} else if transitions, ok := n.Attributes["transitions"].([]any); ok {
		for _, t := range transitions {
			if m, ok := t.(map[string]any); ok {
				from, _ := m["from"].(string)
				to, _ := m["to"].(string)
				if from != "" && to != "" {
					def.Transitions = append(def.Transitions, from+"→"+to)
				}
			}
		}
	}

	// Decode fields: []{"name":"priority","type":"string","required":true} → ["priority:string:required"]
	var fieldsRaw any
	if decodeRaw(n.Attributes["fields"], &fieldsRaw) {
		// fieldsRaw already unmarshaled
	} else if arr, ok := n.Attributes["fields"].([]any); ok {
		fieldsRaw = arr
	}
	if fields, ok := fieldsRaw.([]any); ok {
		for _, f := range fields {
			if m, ok := f.(map[string]any); ok {
				name, _ := m["name"].(string)
				fType, _ := m["type"].(string)
				required, _ := m["required"].(bool)
				if name != "" {
					s := name
					if fType != "" {
						s += ":" + fType
					}
					if required {
						s += ":required"
					}
					def.Fields = append(def.Fields, s)
				}
			}
		}
	}

	return def
}
