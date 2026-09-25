package embedded

import "strings"

// targetContinuityScope prevents the original upstream session or reasoning replay from being reused after the target changes.
func targetContinuityScope(scope, baseURL string) string {
	if strings.TrimSpace(scope) == "" {
		return ""
	}
	return scope + "\x00target\x00" + baseURL
}
