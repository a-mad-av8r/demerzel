package embedded

import (
	"fmt"
	"net/url"
	"strings"
)

// ResolveAPIEndpoint combines the subscription API proxy root URL with the native path of an official endpoint.
// An empty root retains the official endpoint; a custom root path prefix is neither replaced nor heuristically trimmed.
func ResolveAPIEndpoint(apiRoot, officialURL string) (string, error) {
	apiRoot = strings.TrimSpace(apiRoot)
	if apiRoot == "" {
		return officialURL, nil
	}
	root, err := url.Parse(apiRoot)
	if err != nil || root == nil || !strings.EqualFold(root.Scheme, "https") || root.Opaque != "" ||
		root.Host == "" || root.User != nil || root.RawQuery != "" || root.ForceQuery || strings.Contains(apiRoot, "#") {
		return "", fmt.Errorf("invalid subscription API proxy root")
	}
	official, err := url.Parse(officialURL)
	if err != nil || official == nil || official.Host == "" || official.Opaque != "" ||
		official.User != nil || official.Fragment != "" {
		return "", fmt.Errorf("invalid subscription official API endpoint")
	}
	root.Scheme = "https"
	root.Host = strings.ToLower(root.Host)
	if root.Port() == "443" {
		hostname := root.Hostname()
		if strings.Contains(hostname, ":") {
			hostname = "[" + hostname + "]"
		}
		root.Host = hostname
	}
	path := strings.TrimRight(root.EscapedPath(), "/")
	if official.Path != "" {
		path += "/" + strings.TrimLeft(official.EscapedPath(), "/")
	}
	root.Path, err = url.PathUnescape(path)
	if err != nil {
		return "", fmt.Errorf("invalid subscription API endpoint path")
	}
	root.RawPath = path
	root.RawQuery, root.ForceQuery = official.RawQuery, official.ForceQuery
	return root.String(), nil
}
