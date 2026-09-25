package bifrost

import (
	"net/http"
	"strings"

	"gpt-load/internal/execution"
	"gpt-load/internal/platform/version"
)

// withUserAgent fills outbound identity only for ordinary channels; subscription channels are handled by their own adapters.
func withUserAgent(spec execution.AttemptSpec) execution.AttemptSpec {
	// System or group rules are applied first; retain their configured value or the client's original identity.
	if strings.TrimSpace(spec.Header.Get("User-Agent")) != "" {
		return spec
	}
	spec.Header = spec.Header.Clone()
	if spec.Header == nil {
		spec.Header = make(http.Header)
	}
	// Use project identity when there is no valid UA, including cases where a rule explicitly removed or blanked it.
	spec.Header.Set("User-Agent", "GPT-Load/"+version.Version)
	return spec
}
