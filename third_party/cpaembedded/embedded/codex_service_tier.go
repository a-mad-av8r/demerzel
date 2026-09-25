package embedded

import (
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/runtime/executor/helps"
	codexchat "github.com/router-for-me/CLIProxyAPI/v7/internal/translator/codex/openai/chat-completions"
	codexresponses "github.com/router-for-me/CLIProxyAPI/v7/internal/translator/codex/openai/responses"
	sdktranslator "github.com/router-for-me/CLIProxyAPI/v7/sdk/translator"
	"github.com/tidwall/gjson"
)

// HTTP and WebSocket share this conversion entry point; responses continue to use CPA's native converter.
func init() {
	sdktranslator.Register(sdktranslator.FormatOpenAIResponse, sdktranslator.FormatCodex,
		codexRequestWithServiceTier(codexresponses.ConvertOpenAIResponsesRequestToCodex),
		sdktranslator.ResponseTransform{
			Stream:    codexresponses.ConvertCodexResponseToOpenAIResponses,
			NonStream: codexresponses.ConvertCodexResponseToOpenAIResponsesNonStream,
		})
	sdktranslator.Register(sdktranslator.FormatOpenAI, sdktranslator.FormatCodex,
		codexRequestWithServiceTier(codexchat.ConvertOpenAIRequestToCodex),
		sdktranslator.ResponseTransform{
			Stream:    codexchat.ConvertCodexResponseToOpenAI,
			NonStream: codexchat.ConvertCodexResponseToOpenAINonStream,
		})
}

func codexRequestWithServiceTier(convert sdktranslator.RequestTransform) sdktranslator.RequestTransform {
	return func(model string, raw []byte, stream bool) []byte {
		tier := strings.ToLower(strings.TrimSpace(gjson.GetBytes(raw, "service_tier").String()))
		body := convert(model, raw, stream)
		switch tier {
		case "fast", "priority":
			// The Codex subscription endpoint uses priority; it cannot use the public API's fast alias.
			tier = "priority"
		case "ultrafast":
			// Restore it after CPA's native conversion to avoid removal by its field-filter rules.
		default:
			return body
		}
		return helps.SetStringIfDifferent(body, "service_tier", tier)
	}
}
