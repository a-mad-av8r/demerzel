package embedded

// CodexAPIEndpoints declares the service endpoints used after credentials are prepared, excluding OAuth and identity verification.
type CodexAPIEndpoints struct {
	ExecutionBase string
	AccountBase   string
}

// ResolveCodexAPIEndpoints retains the complete official target set or maps native paths to the proxy root URL.
func ResolveCodexAPIEndpoints(apiRoot string) (CodexAPIEndpoints, error) {
	endpoints := CodexAPIEndpoints{
		ExecutionBase: defaultCodexBaseURL,
		AccountBase:   defaultCodexAPIBase,
	}
	for _, endpoint := range []*string{&endpoints.ExecutionBase, &endpoints.AccountBase} {
		resolved, err := ResolveAPIEndpoint(apiRoot, *endpoint)
		if err != nil {
			return CodexAPIEndpoints{}, err
		}
		*endpoint = resolved
	}
	return endpoints, nil
}
