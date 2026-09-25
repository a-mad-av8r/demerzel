package parameteroverride

import (
	"bytes"
	"encoding/json"
	"unicode/utf8"

	"github.com/buger/jsonparser"
)

// Create nodes only for paths covered by rules; all other values continue to reference the original JSON without expanding arrays or objects.
// raw nil means the field is absent; JSON null remains four bytes.
type requestValue struct {
	raw    []byte
	fields map[string]*requestValue
	loaded bool
	dirty  bool
	// The final occurrence of this field in the parent object's original JSON; override values do not alter it.
	start int
	end   int
	// Object generations invalidate stale child fields without clearing every rule field on each traversal.
	generation       uint
	parentGeneration uint
}

func (value *requestValue) field(key string) *requestValue {
	if value.fields == nil {
		value.fields = make(map[string]*requestValue)
	}
	if value.fields[key] == nil {
		value.fields[key] = &requestValue{}
	}
	return value.fields[key]
}

func (value *requestValue) planSet(source map[string]any) {
	for key, item := range source {
		child := value.field(key)
		if object, ok := item.(map[string]any); ok {
			child.planSet(object)
		}
	}
}

func (value *requestValue) planPath(path []string) {
	for _, key := range path {
		value = value.field(key)
	}
}

func (value *requestValue) reset(raw []byte) {
	value.raw, value.loaded, value.dirty = raw, false, false
}

func (value *requestValue) isObject() bool {
	return len(value.raw) > 0 && value.raw[0] == '{'
}

func (value *requestValue) load() error {
	if value.loaded {
		return nil
	}
	_, err := value.scan(value.raw)
	return err
}

// Traverse directly along rule paths and return the object end; do not scan a whole child subtree before recursing.
func (value *requestValue) scan(body []byte) (int, error) {
	value.generation++
	end, err := value.eachField(body, true, nil)
	if err != nil {
		return 0, err
	}
	value.raw, value.loaded = body[:end], true
	return end, nil
}

func (value *requestValue) currentField(key string) *requestValue {
	child := value.fields[key]
	if child.parentGeneration != value.generation {
		child.reset(nil)
		child.start, child.end = 0, 0
		child.parentGeneration = value.generation
	}
	return child
}

func (value *requestValue) remove(path []string) error {
	if !value.isObject() {
		return nil
	}
	if err := value.load(); err != nil {
		return err
	}
	child := value.currentField(path[0])
	if len(path) == 1 {
		child.reset(nil)
		child.dirty = true
	} else if err := child.remove(path[1:]); err != nil {
		return err
	}
	value.dirty = value.dirty || child.dirty
	return nil
}

func (value *requestValue) merge(source any) error {
	if object, ok := source.(map[string]any); ok && value.isObject() {
		if len(object) == 0 {
			return nil
		}
		if err := value.load(); err != nil {
			return err
		}
		for key, item := range object {
			if err := value.currentField(key).merge(item); err != nil {
				return err
			}
		}
		value.dirty = true
		return nil
	}
	// Serialise only replacement values limited by configuration size; large client values never enter a general JSON tree.
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(source); err != nil {
		return err
	}
	value.reset(bytes.TrimSuffix(encoded.Bytes(), []byte("\n")))
	value.dirty = true
	return nil
}

type requestOutput struct {
	body []byte
	size int
}

func (output *requestOutput) append(raw []byte) {
	output.size += len(raw)
	if output.body != nil {
		output.body = append(output.body, raw...)
	}
}

func (value *requestValue) write(output *requestOutput) error {
	if !value.dirty || !value.loaded {
		output.append(value.raw)
		return nil
	}
	output.append([]byte{'{'})
	first := true
	separator := func() {
		if !first {
			output.append([]byte{','})
		}
		first = false
	}
	if _, err := value.eachField(value.raw, false, func(key, member []byte) {
		if child := value.fields[string(key)]; child != nil && child.parentGeneration == value.generation && child.dirty {
			return
		}
		separator()
		output.append(member)
	}); err != nil {
		return err
	}
	for key, child := range value.fields {
		if child.parentGeneration != value.generation || !child.dirty || child.raw == nil {
			continue
		}
		separator()
		encodedKey, err := json.Marshal(key)
		if err != nil {
			return err
		}
		output.append(encodedKey)
		output.append([]byte{':'})
		if err := child.write(output); err != nil {
			return err
		}
	}
	output.append([]byte{'}'})
	return nil
}

// The caller already validated the complete request with json.Valid. Scan only field boundaries here, retaining references to raw values;
// decode only keys containing escapes or invalid UTF-8, preserving encoding/json key-name semantics.
func (value *requestValue) eachField(body []byte, discover bool, visit func(key, member []byte)) (int, error) {
	remaining := bytes.TrimLeft(body[1:], " \t\r\n")
	for remaining[0] != '}' {
		member := remaining
		key, _, end, err := jsonparser.Get(remaining)
		if err != nil {
			return 0, err
		}
		if bytes.ContainsRune(key, '\\') || !utf8.Valid(key) {
			var decoded string
			if err := json.Unmarshal(remaining[:end], &decoded); err != nil {
				return 0, err
			}
			key = []byte(decoded)
		}
		remaining = bytes.TrimLeft(remaining[end:], " \t\r\n")
		remaining = bytes.TrimLeft(remaining[1:], " \t\r\n") // Skip colon.
		start := len(body) - len(remaining)
		child := value.fields[string(key)]
		switch {
		case discover && child != nil && remaining[0] == '{' && len(child.fields) > 0:
			child.reset(nil)
			end, err = child.scan(remaining)
		case !discover && child != nil && child.parentGeneration == value.generation && child.start == start && child.end > start:
			// Measure and output directly from an already discovered end, without rescanning deeply nested large values.
			end = child.end - start
		default:
			_, _, end, err = jsonparser.Get(remaining)
			if discover && child != nil {
				child.reset(nil)
			}
		}
		if err != nil {
			return 0, err
		}
		if discover && child != nil {
			// For duplicate keys, retain only the final position so index size does not grow with client field count.
			child.raw = remaining[:end]
			child.start, child.end = start, start+end
			child.parentGeneration = value.generation
		}
		if visit != nil {
			visit(key, member[:len(member)-len(remaining)+end])
		}
		remaining = bytes.TrimLeft(remaining[end:], " \t\r\n")
		if remaining[0] == ',' {
			remaining = bytes.TrimLeft(remaining[1:], " \t\r\n")
		}
	}
	return len(body) - len(remaining) + 1, nil
}
