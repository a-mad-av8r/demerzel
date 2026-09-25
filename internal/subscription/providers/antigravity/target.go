package antigravity

import (
	"context"

	cpaembedded "github.com/router-for-me/CLIProxyAPI/v7/gptload-embedded/embedded"
)

// antigravityAPIOptions sets a proxy target only for business calls after credential preparation completes.
func antigravityAPIOptions(ctx context.Context, apiRoot string) (cpaembedded.AntigravityOptions, error) {
	options, err := antigravityOptions(ctx)
	if err != nil {
		return cpaembedded.AntigravityOptions{}, err
	}
	endpoints, err := cpaembedded.ResolveAntigravityAPIEndpoints(apiRoot)
	if err != nil {
		return cpaembedded.AntigravityOptions{}, err
	}
	options.FetchModelsURL = endpoints.FetchModelsURL
	options.LoadCodeAssistURL = endpoints.LoadCodeAssistURL
	options.RetrieveUserQuotaURL = endpoints.RetrieveUserQuotaURL
	return options, nil
}
