package stampmitra

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"sync"
)

var knownFieldsCache sync.Map // reflect.Type -> map[string]struct{} (lower-cased JSON names)

// knownFields returns the lower-cased JSON field names of the struct type behind v (a pointer to a
// struct). encoding/json matches keys case-insensitively, so comparison is case-insensitive too.
func knownFields(v any) map[string]struct{} {
	t := reflect.TypeOf(v)
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if cached, ok := knownFieldsCache.Load(t); ok {
		return cached.(map[string]struct{})
	}
	out := make(map[string]struct{}, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		tag := t.Field(i).Tag.Get("json")
		if tag == "-" {
			continue
		}
		name := strings.Split(tag, ",")[0]
		if name == "" {
			name = t.Field(i).Name
		}
		out[strings.ToLower(name)] = struct{}{}
	}
	knownFieldsCache.Store(t, out)
	return out
}

// decodeWithExtra decodes b into target (a pointer to a struct type WITHOUT an UnmarshalJSON method,
// i.e. the local "plain" alias) and stores every JSON key target does not declare in *extra.
func decodeWithExtra(b []byte, target any, extra *map[string]json.RawMessage) error {
	if bytes.Equal(bytes.TrimSpace(b), []byte("null")) {
		return nil
	}
	if err := json.Unmarshal(b, target); err != nil {
		return err
	}
	var all map[string]json.RawMessage
	if err := json.Unmarshal(b, &all); err != nil {
		return err
	}
	known := knownFields(target)
	var rest map[string]json.RawMessage
	for k, v := range all {
		if _, ok := known[strings.ToLower(k)]; ok {
			continue
		}
		if rest == nil {
			rest = make(map[string]json.RawMessage)
		}
		rest[k] = append(json.RawMessage(nil), v...)
	}
	*extra = rest
	return nil
}

// encodeWithExtra marshals v (a value of the plain alias type) and merges extra into the object;
// declared fields win over same-named extras.
func encodeWithExtra(v any, extra map[string]json.RawMessage) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil || len(extra) == 0 {
		return b, err
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(b, &obj); err != nil {
		return nil, err
	}
	for k, raw := range extra {
		if _, exists := obj[k]; !exists {
			obj[k] = raw
		}
	}
	return json.Marshal(obj)
}
