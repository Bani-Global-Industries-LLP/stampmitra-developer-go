package stampmitra

import (
	"bytes"
	"encoding/json"
	"errors"
)

// GenericResponse holds a response whose shape the API does not fix (e-Stamp reference data,
// duty calculations, parties, webhook event lists and replays). The full body is preserved, so no
// field is ever dropped; use Get, Map, Items or Decode to read it.
type GenericResponse struct {
	raw json.RawMessage
}

// UnmarshalJSON stores the body verbatim.
func (g *GenericResponse) UnmarshalJSON(b []byte) error {
	g.raw = append(g.raw[:0], b...)
	return nil
}

// MarshalJSON returns the stored body.
func (g GenericResponse) MarshalJSON() ([]byte, error) {
	if len(g.raw) == 0 {
		return []byte("null"), nil
	}
	return g.raw, nil
}

// Raw returns a copy of the JSON body.
func (g *GenericResponse) Raw() json.RawMessage {
	if g == nil {
		return nil
	}
	return append(json.RawMessage(nil), g.raw...)
}

// Decode unmarshals the body into v.
func (g *GenericResponse) Decode(v any) error {
	if g == nil || len(g.raw) == 0 {
		return errors.New("stampmitra: empty response")
	}
	return json.Unmarshal(g.raw, v)
}

// Map returns the body as a JSON object keyed by field; it fails when the body is not an object.
func (g *GenericResponse) Map() (map[string]json.RawMessage, error) {
	var m map[string]json.RawMessage
	if err := g.Decode(&m); err != nil {
		return nil, err
	}
	return m, nil
}

// Get returns one top-level field of an object body.
func (g *GenericResponse) Get(key string) (json.RawMessage, bool) {
	m, err := g.Map()
	if err != nil {
		return nil, false
	}
	v, ok := m[key]
	return v, ok
}

// Items returns the elements of a top-level array body, or of the "items" field of an object body.
func (g *GenericResponse) Items() []json.RawMessage {
	if g == nil {
		return nil
	}
	var arr []json.RawMessage
	if t := bytes.TrimSpace(g.raw); len(t) > 0 && t[0] == '[' {
		if json.Unmarshal(t, &arr) == nil {
			return arr
		}
		return nil
	}
	if v, ok := g.Get("items"); ok {
		if json.Unmarshal(v, &arr) == nil {
			return arr
		}
	}
	return nil
}

// NextCursor returns the "nextCursor" field of a paged object body, or "" when there is none.
func (g *GenericResponse) NextCursor() string {
	v, ok := g.Get("nextCursor")
	if !ok {
		return ""
	}
	var s string
	if json.Unmarshal(v, &s) != nil {
		return ""
	}
	return s
}
