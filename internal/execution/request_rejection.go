package execution

import "strings"

// ExplicitRequestRejection recognises concrete codes and explicit context-limit failures; generic error types or arbitrary message substrings do not stop retries.
func ExplicitRequestRejection(typeValue, codeValue, message string) bool {
	for _, value := range []string{typeValue, codeValue} {
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "context_length_exceeded", "invalid_parameter", "content_policy_violation":
			return true
		}
	}
	if !strings.EqualFold(strings.TrimSpace(typeValue), "invalid_request_error") || strings.TrimSpace(codeValue) != "" {
		return false
	}
	message = strings.ToLower(strings.TrimSpace(message))
	return strings.TrimSuffix(message, ".") == "prompt is too long" ||
		strings.HasPrefix(message, "prompt is too long:")
}
