package bifrost

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/maximhq/bifrost/core/schemas"

	"gpt-load/internal/dialect"
	"gpt-load/internal/execution"
	"gpt-load/internal/protocol"
	"gpt-load/internal/usage"
)

type anthropicWireUsage struct {
	extractor dialect.UsageStreamSnapshotExtractor
	seen      bool
	invalid   bool
	lastUsage json.RawMessage
}

func captureAnthropicWireUsage(ctx *schemas.BifrostContext, prepared preparedAttempt) *anthropicWireUsage {
	if prepared.upstreamProtocol != protocol.Anthropic {
		return nil
	}
	// Use only the raw response returned with each SDK chunk; do not buffer the entire answer.
	// Re-enable the response hook's integration type to prevent SDK dispatch from clearing raw capture for compatible upstreams.
	ctx.SetValue(schemas.BifrostContextKeyAllowPerRequestRawOverride, true)
	ctx.SetValue(schemas.BifrostContextKeySendBackRawResponse, true)
	ctx.SetValue(anthropicUsageCaptureKey, true)
	return &anthropicWireUsage{extractor: dialect.NewAnthropic().NewUsageStreamSnapshotExtractor()}
}

func (capture *anthropicWireUsage) observe(raw *any) {
	if capture == nil || raw == nil || *raw == nil {
		return
	}
	value := *raw
	*raw = nil // Raw answers must not enter subsequent client serialisation or ordinary logs.
	capture.seen = true
	body, ok := rawRequestJSON(value)
	if !ok {
		capture.invalid = true
		return
	}
	if err := capture.extractor.Observe(body); err != nil {
		capture.invalid = true
	}
	// Retain evidence only for the final raw usage object, never message content.
	var event struct {
		Usage   json.RawMessage `json:"usage"`
		Message struct {
			Usage json.RawMessage `json:"usage"`
		} `json:"message"`
	}
	if err := json.Unmarshal(body, &event); err != nil {
		capture.invalid = true
		return
	}
	if len(event.Usage) > 0 && string(event.Usage) != "null" {
		capture.lastUsage = event.Usage
	} else if len(event.Message.Usage) > 0 && string(event.Message.Usage) != "null" {
		capture.lastUsage = event.Message.Usage
	}
}

func (capture *anthropicWireUsage) evidence() *execution.UsageEvidence {
	if capture == nil || !capture.seen {
		return nil
	}
	result := capture.extractor.Snapshot()
	if capture.invalid {
		result.Diagnostics.Add(usage.DiagnosticInvalidPayload)
	}
	return &execution.UsageEvidence{Normalized: result, Raw: append([]byte(nil), capture.lastUsage...)}
}

func (capture *anthropicWireUsage) discardErrorRaw(err *schemas.BifrostError) {
	if capture != nil && err != nil {
		err.ExtraFields.RawResponse = nil
	}
}

func (capture *anthropicWireUsage) applyResponses(target *schemas.ResponsesResponseUsage) error {
	evidence := capture.evidence()
	if target == nil || evidence == nil {
		return nil
	}
	tokens := evidence.Normalized.Tokens
	total, ok := usage.CheckedTotal(tokens)
	if !ok || total > int64(math.MaxInt) {
		return fmt.Errorf("Anthropic usage exceeds SDK token range")
	}
	target.InputTokens = int(total - tokens.Output)
	target.OutputTokens = int(tokens.Output)
	target.TotalTokens = int(total)
	if target.InputTokensDetails == nil {
		target.InputTokensDetails = &schemas.ResponsesResponseInputTokens{}
	}
	details := target.InputTokensDetails
	details.CachedReadTokens = int(tokens.CacheRead)
	details.CachedWriteTokens = int(tokens.CacheWrite5M + tokens.CacheWrite1H + tokens.CacheWriteUnknown)
	details.CachedWriteTokenDetails = &schemas.ChatCachedWriteTokenDetails{
		CachedWriteTokens5m: int(tokens.CacheWrite5M), CachedWriteTokens1h: int(tokens.CacheWrite1H),
	}
	return nil
}

// Enable message_delta in the synchronous hook for the first response event, retaining subsequent usage events
// without triggering SDK request dispatch to clear raw capture and URL behaviour by provider allow-list.
// Set this marker only for streaming attempts already known to use the Anthropic upstream protocol.
type anthropicUsageContextKey struct{}

var anthropicUsageCaptureKey anthropicUsageContextKey

type anthropicUsageStreamHook struct{}

func (anthropicUsageStreamHook) GetName() string { return "gpt-load-anthropic-usage" }
func (anthropicUsageStreamHook) Cleanup() error  { return nil }
func (anthropicUsageStreamHook) PreRequestHook(_ *schemas.BifrostContext, _ *schemas.BifrostRequest) error {
	return nil
}
func (anthropicUsageStreamHook) PreLLMHook(_ *schemas.BifrostContext, req *schemas.BifrostRequest) (*schemas.BifrostRequest, *schemas.LLMPluginShortCircuit, error) {
	return req, nil, nil
}
func (anthropicUsageStreamHook) PostLLMHook(ctx *schemas.BifrostContext, resp *schemas.BifrostResponse, err *schemas.BifrostError) (*schemas.BifrostResponse, *schemas.BifrostError, error) {
	if enabled, _ := ctx.Value(anthropicUsageCaptureKey).(bool); enabled && resp != nil && resp.ResponsesStreamResponse != nil {
		ctx.Root().SetValue(schemas.BifrostContextKeyIntegrationType, "anthropic")
	}
	return resp, err, nil
}
