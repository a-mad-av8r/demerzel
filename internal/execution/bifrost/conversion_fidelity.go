package bifrost

import (
	"context"

	"github.com/maximhq/bifrost/core/providers/anthropic"
	"github.com/maximhq/bifrost/core/schemas"

	"gpt-load/internal/channel"
	"gpt-load/internal/dialect"
	"gpt-load/internal/execution"
)

func finishConvertedPreparation(spec execution.AttemptSpec, providerKind channel.ProviderKind, prepared preparedAttempt) (preparedAttempt, *execution.AttemptResult) {
	if spec.RouteMode != execution.RouteConverted ||
		(spec.Operation != execution.OperationChatCompletion && spec.Operation != execution.OperationResponsesCreate) {
		return prepared, nil
	}
	var toolConstraintsValid bool
	var needsToolHistoryCheck bool
	prepared, needsToolHistoryCheck, toolConstraintsValid = prepareConvertedToolConstraints(spec.ClientProtocol, providerKind, spec.UpstreamModel, prepared)
	if !toolConstraintsValid {
		failure := notSentConversionFailure(execution.ErrorCodeCriticalSemanticLoss, "conversion cannot preserve requested tools or tool choice")
		return preparedAttempt{}, &failure
	}
	vertexChat := providerKind == channel.ProviderGoogleVertex && vertexUsesChatFallback(spec.UpstreamModel)
	if providerKind == channel.ProviderAnthropic || providerKind == channel.ProviderGemini ||
		providerKind == channel.ProviderAWSBedrock || (providerKind == channel.ProviderGoogleVertex && !vertexChat) {
		preserveResponsesGlobalInstructions(prepared.responsesRequest)
	}
	chatFallback := providerKind == channel.ProviderOpenAICompatible || providerKind == channel.ProviderGroq ||
		providerKind == channel.ProviderDeepSeek || vertexChat
	if chatFallback {
		dropsTools, newlyAllowed := chatFallbackToolCompatibility(prepared.responsesRequest)
		needsToolHistoryCheck = needsToolHistoryCheck || newlyAllowed
		// Compatible preserves the SDK's best-effort conversion: unsupported tools and choices can be filtered
		// rather than rejecting the entire request; ordinary-function allow-lists are still adapted by the preparation stage.
		if dropsTools && providerKind != channel.ProviderOpenAICompatible {
			failure := notSentConversionFailure(execution.ErrorCodeCriticalSemanticLoss, "Chat conversion cannot preserve requested tools or tool choice")
			return preparedAttempt{}, &failure
		}
		if providerKind == channel.ProviderDeepSeek && deepSeekConversionDisablesThinking(prepared.responsesRequest) {
			failure := notSentConversionFailure(execution.ErrorCodeCriticalSemanticLoss, "DeepSeek conversion cannot preserve explicit thinking with this tool choice or history")
			return preparedAttempt{}, &failure
		}
	}
	if needsToolHistoryCheck && !convertedTargetPreservesToolHistory(providerKind, spec.UpstreamModel, prepared.responsesRequest) {
		failure := notSentConversionFailure(execution.ErrorCodeCriticalSemanticLoss, "conversion cannot preserve requested tool history")
		return preparedAttempt{}, &failure
	}
	wantSystems := dialect.CountMidConversationSystemMessages(spec.ClientProtocol, spec.Body)
	if wantSystems == 0 {
		return prepared, nil
	}
	preserved := true
	switch providerKind {
	case channel.ProviderGemini, channel.ProviderAWSBedrock:
		// These existing converters can only move mid-conversation system instructions earlier or turn them into user text.
		preserved = false
	case channel.ProviderGoogleVertex:
		// Vertex's non-Claude/Gemini models use the OpenAI wire format, preserving system messages in place.
		preserved = vertexChat
	case channel.ProviderAnthropic:
		// Reuse the locked SDK's ModelCaps and actual placement rules only for this input shape, avoiding copied model tables or approximate tool grouping.
		ctx := schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)
		defer ctx.Cancel()
		var request *anthropic.AnthropicMessageRequest
		var err error
		if prepared.request != nil {
			request, err = anthropic.ToAnthropicChatRequest(ctx, prepared.request)
		} else {
			request, err = anthropic.ToAnthropicResponsesRequest(ctx, prepared.responsesRequest)
		}
		keptSystems := 0
		if err == nil && request != nil {
			for _, message := range request.Messages {
				if message.Role == anthropic.AnthropicMessageRoleSystem {
					keptSystems++
				}
			}
		}
		preserved = keptSystems == wantSystems
	}
	if !preserved {
		failure := notSentConversionFailure(execution.ErrorCodeCriticalSemanticLoss, "conversion cannot preserve mid-conversation system instructions")
		return preparedAttempt{}, &failure
	}
	return prepared, nil
}

func vertexUsesChatFallback(model string) bool {
	ctx := schemas.NewBifrostContext(context.Background(), schemas.NoDeadline)
	defer ctx.Cancel()
	return !schemas.IsAnthropicModelFamily(ctx, model) && !schemas.IsGeminiModelFamily(ctx, model) &&
		!schemas.IsGemmaModelFamily(ctx, model) && !schemas.IsAllDigitsASCII(model)
}

func preserveResponsesGlobalInstructions(request *schemas.BifrostResponsesRequest) {
	if request == nil || request.Params == nil || request.Params.Instructions == nil || *request.Params.Instructions == "" {
		return
	}
	// These SDK converters prioritise system within input and may override concurrent instructions.
	// Put global instructions at the front of input so the same converter processes them in order without sending duplicates.
	request.Input = append([]schemas.ResponsesMessage{{
		Type:    schemas.Ptr(schemas.ResponsesMessageTypeMessage),
		Role:    schemas.Ptr(schemas.ResponsesInputMessageRoleSystem),
		Content: &schemas.ResponsesMessageContent{ContentStr: request.Params.Instructions},
	}}, request.Input...)
	request.Params.Instructions = nil
}

func deepSeekConversionDisablesThinking(request *schemas.BifrostResponsesRequest) bool {
	if request == nil || request.Params == nil || request.Params.Reasoning == nil {
		return false
	}
	reasoning := request.Params.Reasoning
	enabled := (reasoning.Effort != nil && *reasoning.Effort != "" && *reasoning.Effort != "none") ||
		(reasoning.MaxTokens != nil && *reasoning.MaxTokens != 0)
	if !enabled {
		return false
	}
	chat := request.ToChatRequest()
	// Align with the locked SDK's requiresDeepSeekThinkingDisabled: block only conflicting inputs that explicitly enable reasoning.
	if choice := chat.Params.ToolChoice; choice != nil {
		var kind schemas.ChatToolChoiceType
		if choice.ChatToolChoiceStr != nil {
			kind = schemas.ChatToolChoiceType(*choice.ChatToolChoiceStr)
		} else if choice.ChatToolChoiceStruct != nil {
			kind = choice.ChatToolChoiceStruct.Type
		}
		switch kind {
		case schemas.ChatToolChoiceTypeRequired, schemas.ChatToolChoiceTypeAny, schemas.ChatToolChoiceTypeFunction,
			schemas.ChatToolChoiceTypeCustom, schemas.ChatToolChoiceTypeAllowedTools:
			return true
		}
	}
	for _, message := range chat.Input {
		if message.Role == schemas.ChatMessageRoleAssistant && message.ChatAssistantMessage != nil &&
			len(message.ToolCalls) != 0 && message.Reasoning == nil {
			return true
		}
	}
	return false
}
