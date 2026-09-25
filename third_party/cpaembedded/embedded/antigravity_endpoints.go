package embedded

import antigravityauth "github.com/router-for-me/CLIProxyAPI/v7/internal/auth/antigravity"

// AntigravityAPIEndpoints declares the service endpoints used after credentials are prepared, excluding OAuth and identity verification.
type AntigravityAPIEndpoints struct {
	ExecutionBase        string
	FetchModelsURL       string
	LoadCodeAssistURL    string
	RetrieveUserQuotaURL string
}

// ResolveAntigravityAPIEndpoints retains the complete official target set or maps native paths to the proxy root URL.
func ResolveAntigravityAPIEndpoints(apiRoot string) (AntigravityAPIEndpoints, error) {
	endpoints := AntigravityAPIEndpoints{
		ExecutionBase:        antigravityExecutionBase,
		FetchModelsURL:       antigravityExecutionBase + "/" + antigravityauth.APIVersion + ":fetchAvailableModels",
		LoadCodeAssistURL:    antigravityauth.APIEndpoint + "/" + antigravityauth.APIVersion + ":loadCodeAssist",
		RetrieveUserQuotaURL: antigravityauth.DailyAPIEndpoint + "/" + antigravityauth.APIVersion + ":retrieveUserQuotaSummary",
	}
	for _, endpoint := range []*string{&endpoints.ExecutionBase, &endpoints.FetchModelsURL, &endpoints.LoadCodeAssistURL, &endpoints.RetrieveUserQuotaURL} {
		resolved, err := ResolveAPIEndpoint(apiRoot, *endpoint)
		if err != nil {
			return AntigravityAPIEndpoints{}, err
		}
		*endpoint = resolved
	}
	return endpoints, nil
}
