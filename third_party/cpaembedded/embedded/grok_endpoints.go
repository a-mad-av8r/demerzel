package embedded

import xaiauth "github.com/router-for-me/CLIProxyAPI/v7/internal/auth/xai"

// GrokAPIEndpoints declares the service endpoints used after credentials are prepared, excluding OAuth and identity verification.
type GrokAPIEndpoints struct {
	ExecutionBase     string
	ModelsURL         string
	BillingWeeklyURL  string
	BillingMonthlyURL string
}

// ResolveGrokAPIEndpoints retains the complete official target set or maps native paths to the proxy root URL.
func ResolveGrokAPIEndpoints(apiRoot string) (GrokAPIEndpoints, error) {
	endpoints := GrokAPIEndpoints{
		ExecutionBase:     xaiauth.CLIChatProxyBaseURL,
		ModelsURL:         grokModelsURL,
		BillingWeeklyURL:  grokBillingWeeklyURL,
		BillingMonthlyURL: grokBillingMonthlyURL,
	}
	for _, endpoint := range []*string{&endpoints.ExecutionBase, &endpoints.ModelsURL, &endpoints.BillingWeeklyURL, &endpoints.BillingMonthlyURL} {
		resolved, err := ResolveAPIEndpoint(apiRoot, *endpoint)
		if err != nil {
			return GrokAPIEndpoints{}, err
		}
		*endpoint = resolved
	}
	return endpoints, nil
}
