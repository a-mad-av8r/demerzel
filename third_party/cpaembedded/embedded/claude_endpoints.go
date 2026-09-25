package embedded

import claudeauth "github.com/router-for-me/CLIProxyAPI/v7/internal/auth/claude"

// ClaudeAPIEndpoints declares the service endpoints used after credentials are prepared, excluding OAuth and identity verification.
type ClaudeAPIEndpoints struct {
	ExecutionBase string
	ProfileURL    string
	RolesURL      string
	BootstrapURL  string
	UsageURL      string
}

// ResolveClaudeAPIEndpoints retains the complete official target set or maps native paths to the proxy root URL.
func ResolveClaudeAPIEndpoints(apiRoot string) (ClaudeAPIEndpoints, error) {
	endpoints := ClaudeAPIEndpoints{
		ExecutionBase: "https://api.anthropic.com",
		ProfileURL:    claudeauth.ProfileURL,
		RolesURL:      claudeauth.RolesURL,
		BootstrapURL:  ClaudeBootstrapURL,
		UsageURL:      ClaudeUsageURL,
	}
	for _, endpoint := range []*string{&endpoints.ExecutionBase, &endpoints.ProfileURL, &endpoints.RolesURL, &endpoints.BootstrapURL, &endpoints.UsageURL} {
		resolved, err := ResolveAPIEndpoint(apiRoot, *endpoint)
		if err != nil {
			return ClaudeAPIEndpoints{}, err
		}
		*endpoint = resolved
	}
	return endpoints, nil
}
