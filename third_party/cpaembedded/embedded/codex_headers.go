package embedded

import (
	"net/http"
	"strings"
)

// CodexClientVersion must match the pinned CPA dependency and GPT-Load Codex model catalogue version; tests enforce it.
const CodexClientVersion = "0.154.0"

// codexHeadersRoundTripper preserves CPA's UA and pins the version and HTTP session headers.
type codexHeadersRoundTripper struct {
	base       http.RoundTripper
	source     http.Header
	configured []string
}

func (transport codexHeadersRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	request = request.Clone(request.Context())
	for _, name := range transport.configured {
		name = http.CanonicalHeaderKey(name)
		switch name {
		case "Originator":
			value, present := codexHeaderValue(transport.source, name)
			if present {
				request.Header.Set(name, value)
			} else {
				request.Header.Del(name)
			}
		}
	}
	request.Header.Set("Version", CodexClientVersion)
	normalizeCodexSessionHeader(request.Header)
	// CPA's direct image route does not read opts.Headers; restore the session explicitly provided by the caller.
	if request.Header.Get("Session-Id") == "" {
		if session := transport.source.Get("Session-Id"); session != "" {
			request.Header.Set("Session-Id", session)
		}
	}
	return transport.base.RoundTrip(request)
}

func normalizedCodexHeaders(headers http.Header) http.Header {
	cloned := headers.Clone()
	if cloned == nil {
		cloned = make(http.Header)
	}
	// Only Codex ignores version identities from the client and grouping rules; CPA generates the UA.
	for name := range cloned {
		if strings.EqualFold(name, "User-Agent") || strings.EqualFold(name, "Version") {
			delete(cloned, name)
		}
	}
	cloned.Set("Version", CodexClientVersion)
	normalizeCodexSessionHeader(cloned)
	return cloned
}

// normalizeCodexSessionHeader accepts the underscore spelling; the hyphenated field takes precedence when both are present.
func normalizeCodexSessionHeader(headers http.Header) {
	session, _ := codexHeaderValue(headers, "Session-Id")
	session = strings.TrimSpace(session)
	if session == "" {
		session, _ = codexHeaderValue(headers, "Session_id")
		session = strings.TrimSpace(session)
	}
	for name := range headers {
		if strings.EqualFold(name, "Session-Id") || strings.EqualFold(name, "Session_id") {
			delete(headers, name)
		}
	}
	if session != "" {
		headers.Set("Session-Id", session)
	}
}

func codexHeaderValue(headers http.Header, name string) (string, bool) {
	values, present := headers[http.CanonicalHeaderKey(name)]
	if !present {
		for key, candidate := range headers {
			if strings.EqualFold(key, name) {
				values, present = candidate, true
				break
			}
		}
	}
	if len(values) > 0 {
		return values[0], present
	}
	return "", present
}
