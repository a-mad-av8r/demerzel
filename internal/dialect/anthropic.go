package dialect

import (
	"bytes"
	"encoding/json"
	"fmt"

	"gpt-load/internal/execution"
	"gpt-load/internal/protocol"
)

type Anthropic struct{}

var _ Dialect = (*Anthropic)(nil)

func NewAnthropic() *Anthropic {
	return &Anthropic{}
}

func (*Anthropic) Protocol() protocol.Protocol {
	return protocol.Anthropic
}

// AnthropicRequestsZeroOutput determines whether a request explicitly asks for zero output, without treating missing, null, or string values as numeric zero.
func AnthropicRequestsZeroOutput(body []byte) bool {
	var request struct {
		MaxTokens json.RawMessage `json:"max_tokens"`
	}
	if err := json.Unmarshal(body, &request); err != nil {
		return false
	}
	mantissa := request.MaxTokens
	if index := bytes.IndexAny(mantissa, "eE"); index >= 0 {
		mantissa = mantissa[:index]
	}
	// Judge by significant digits so a very small non-zero value cannot underflow a float and be mistaken for zero.
	return bytes.IndexByte(mantissa, '0') >= 0 && len(bytes.Trim(mantissa, "-0.")) == 0
}

func (d *Anthropic) InspectRequest(req *ParsedRequest) (RequestMetadata, error) {
	if req == nil {
		return RequestMetadata{}, fmt.Errorf("parsed request is required")
	}

	metadata, err := inspectJSONRequestFields(req.Body, true, false)
	if err != nil {
		return RequestMetadata{}, fmt.Errorf("decode %s request: %w", d.Protocol(), err)
	}
	if req.Path == "/v1/messages/count_tokens" {
		metadata.Stream = false
		metadata.ObserveUsage = false
		metadata.Operation = execution.OperationCountTokens
		metadata.RouteRequirement = execution.RouteRequirementAny
		return metadata, nil
	}

	metadata.ObserveUsage = true
	metadata.AffinityPrefix = inspectPromptAffinityPrefix(d.Protocol(), req.Body)
	metadata.Reasoning = inspectAnthropicReasoning(req.Body)
	metadata.Operation, metadata.RouteRequirement = chatExecutionMetadata(
		d.Protocol(),
		req.Body,
	)
	return metadata, nil
}
