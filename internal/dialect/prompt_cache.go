package dialect

import (
	"bytes"
	"encoding/json"
	"strings"
	"unicode"
	"unicode/utf8"
)

// inspectPromptCacheKey extracts only a bounded, unambiguous top-level soft-affinity signal without changing the client request.
func inspectPromptCacheKey(body []byte) string {
	decoder := json.NewDecoder(bytes.NewReader(body))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return ""
	}
	var key string
	seen := false
	for decoder.More() {
		field, err := decoder.Token()
		if err != nil {
			return ""
		}
		var value json.RawMessage
		if decoder.Decode(&value) != nil {
			return ""
		}
		if field != "prompt_cache_key" {
			continue
		}
		if seen {
			return ""
		}
		seen = true
		if json.Unmarshal(value, &key) != nil {
			return ""
		}
	}
	if key == "" || len(key) > 256 || !utf8.ValidString(key) || strings.TrimSpace(key) != key {
		return ""
	}
	for _, r := range key {
		if unicode.IsControl(r) {
			return ""
		}
	}
	return key
}
