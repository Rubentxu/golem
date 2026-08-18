package packs

import (
	"bytes"
	"encoding/json"
	"sort"
)

// MarshalCanonical serialises v to a JSON byte slice with all maps' keys
// sorted recursively (at every nesting level) and with insignificant
// whitespace removed. The output is deterministic: two values that are
// structurally equal produce byte-identical output, making it suitable for
// stable SHA-256 digests.
//
// This implements the "canonical JSON" serialisation defined in
// CAPABILITY_PACKS.md §Manifest for the integrity digest computation.
func MarshalCanonical(v any) ([]byte, error) {
	// Step 1: round-trip through json.Marshal/Unmarshal to get an interface{}
	// representation where struct field names become map keys.
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var iface any
	if err := json.Unmarshal(b, &iface); err != nil {
		return nil, err
	}
	// Step 2: re-serialise with sorted keys at every level.
	buf := new(bytes.Buffer)
	if err := encodeCanonical(buf, iface); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// encodeCanonical recursively walks v and writes canonical JSON to w.
// Maps are encoded with keys in sorted order; arrays are encoded element by
// element. Primitive values (strings, numbers, bools, null) are written
// directly. This produces deterministic output for SHA-256 digests.
func encodeCanonical(w *bytes.Buffer, v any) error {
	switch val := v.(type) {
	case map[string]any:
		if err := w.WriteByte('{'); err != nil {
			return err
		}
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for i, k := range keys {
			if i > 0 {
				if _, err := w.WriteString(","); err != nil {
					return err
				}
			}
			keyBytes, err := json.Marshal(k)
			if err != nil {
				return err
			}
			if _, err := w.Write(keyBytes); err != nil {
				return err
			}
			if err := w.WriteByte(':'); err != nil {
				return err
			}
			if err := encodeCanonical(w, val[k]); err != nil {
				return err
			}
		}
		return w.WriteByte('}')
	case []any:
		if _, err := w.WriteString("["); err != nil {
			return err
		}
		for i, elem := range val {
			if i > 0 {
				if _, err := w.WriteString(","); err != nil {
					return err
				}
			}
			if err := encodeCanonical(w, elem); err != nil {
				return err
			}
		}
		return w.WriteByte(']')
	case string:
		b, err := json.Marshal(val)
		if err != nil {
			return err
		}
		_, err = w.Write(b)
		return err
	case float64:
		b, err := json.Marshal(val)
		if err != nil {
			return err
		}
		_, err = w.Write(b)
		return err
	case bool:
		if val {
			_, err := w.WriteString("true")
			return err
		}
		_, err := w.WriteString("false")
		return err
	case nil:
		_, err := w.WriteString("null")
		return err
	default:
		return nil
	}
}
