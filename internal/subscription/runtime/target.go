package subscriptionruntime

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"gpt-load/internal/channel/spec"
)

// BaseURL returns the optional subscription API proxy root; an empty value retains the official endpoint set.
func (target Target) BaseURL() (string, error) {
	if len(bytes.TrimSpace(target.Config)) == 0 {
		return "", nil
	}
	var config struct {
		BaseURL string `json:"base_url"`
	}
	if err := json.Unmarshal(target.Config, &config); err != nil {
		return "", fmt.Errorf("decode subscription target: %w", err)
	}
	if strings.TrimSpace(config.BaseURL) == "" {
		return "", nil
	}
	return spec.NormalizeHTTPSBaseURL(config.BaseURL)
}
