package grok

import (
	"context"

	cpaembedded "github.com/router-for-me/CLIProxyAPI/v7/gptload-embedded/embedded"
)

// grokAPIOptions sets a proxy target only for business calls after credential preparation completes.
func grokAPIOptions(ctx context.Context, apiRoot string) (cpaembedded.GrokOptions, error) {
	options, err := grokOptions(ctx)
	if err != nil {
		return cpaembedded.GrokOptions{}, err
	}
	endpoints, err := cpaembedded.ResolveGrokAPIEndpoints(apiRoot)
	if err != nil {
		return cpaembedded.GrokOptions{}, err
	}
	options.ModelsURL = endpoints.ModelsURL
	options.BillingWeeklyURL = endpoints.BillingWeeklyURL
	options.BillingMonthlyURL = endpoints.BillingMonthlyURL
	return options, nil
}
