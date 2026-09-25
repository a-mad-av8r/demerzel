package cpa

import (
	"bytes"
	"encoding/json"
)

// CPA cross-protocol streams inject a local estimate of all input into message_start, but Anthropic's
// input_tokens represents uncached input. Use a zero placeholder to avoid reporting fictitious uncached input,
// or treating a local estimate as billed usage after interruption. Native Anthropic does not use this correction.
func normalizeConvertedAnthropicStartUsage(payload []byte) ([]byte, error) {
	if !bytes.Contains(payload, []byte(`"message_start"`)) {
		return payload, nil
	}
	// The CPA converter produces single-line JSON data fields; one chunk can contain several complete events.
	lines := bytes.SplitAfter(payload, []byte{'\n'})
	for i, line := range lines {
		if !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		body := bytes.TrimSpace(line[len("data:"):])
		var event map[string]json.RawMessage
		if err := json.Unmarshal(body, &event); err != nil {
			return nil, err
		}
		if string(event["type"]) != `"message_start"` {
			continue
		}
		rawMessage, exists := event["message"]
		if !exists {
			continue
		}
		var message map[string]json.RawMessage
		if err := json.Unmarshal(rawMessage, &message); err != nil {
			return nil, err
		}
		rawUsage, exists := message["usage"]
		if !exists {
			continue
		}
		var usage map[string]json.RawMessage
		if err := json.Unmarshal(rawUsage, &usage); err != nil {
			return nil, err
		}
		if _, exists := usage["input_tokens"]; !exists {
			continue
		}
		usage["input_tokens"] = json.RawMessage("0")
		var err error
		message["usage"], err = json.Marshal(usage)
		if err != nil {
			return nil, err
		}
		event["message"], err = json.Marshal(message)
		if err != nil {
			return nil, err
		}
		rewritten, err := json.Marshal(event)
		if err != nil {
			return nil, err
		}
		terminator := line[len(bytes.TrimRight(line, "\r\n")):]
		lines[i] = append(append([]byte("data: "), rewritten...), terminator...)
	}
	return bytes.Join(lines, nil), nil
}
