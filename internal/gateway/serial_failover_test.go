package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"gpt-load/internal/channel"
	"gpt-load/internal/config"
	"gpt-load/internal/execution"
	"gpt-load/internal/protocol"
	"gpt-load/internal/state"
	"gpt-load/internal/testutil/fakeupstream"
)

func serialGatewayHarness(
	t *testing.T,
	forwarder AttemptForwarder,
	baseURL string,
	credentials ...string,
) (*gin.Engine, *Handler, *state.CredentialRegistry) {
	t.Helper()
	handler, manager, registry := newHandlerForTest(t, forwarder, credentials...)
	channelID, params := testChannelConfig(t, protocol.OpenAICompletions, baseURL)
	credentialConfigs := make([]state.CredentialConfig, 0, len(registry.Snapshot()))
	for _, credential := range registry.Snapshot() {
		credentialConfigs = append(credentialConfigs, state.CredentialConfig{
			ID: credential.ID, GroupID: credential.GroupID, Status: credential.Status,
			Version: credential.Version, IdentityGeneration: credential.IdentityGeneration,
			Fingerprint: "serial-credential-" + strconv.FormatUint(uint64(credential.ID), 10),
		})
	}
	accessKeys := make([]state.AccessKeyConfig, 0, len(manager.Current().AccessKeysByHash))
	for keyHash, accessKey := range manager.Current().AccessKeysByHash {
		accessKeys = append(accessKeys, state.AccessKeyConfig{
			ID: accessKey.ID, Name: accessKey.Name, KeyHash: keyHash,
			Status: accessKey.Status, RPMLimit: accessKey.RPMLimit,
		})
	}
	_, err := manager.Publish(state.CompileInput{
		SystemSettings: config.Settings{state.SettingRetryCount: 1},
		ChannelRegistry: channel.NewRegistry(),
		Groups: []state.GroupConfig{{
			ID: 1, Name: "serial pool", ChannelID: channelID, ConnectionType: "api_key",
			Params: params, Models: []state.ModelConfig{{ID: "gpt-4o"}}, Enabled: true,
			Settings: config.Settings{
				state.SettingAccountSelection:          string(state.AccountSelectionSerial),
				state.SettingSerialQuotaReservePercent: 10,
			},
		}},
		Credentials: credentialConfigs, AccessKeys: accessKeys,
	})
	if err != nil {
		t.Fatalf("publish serial test group: %v", err)
	}
	engine := gin.New()
	bindGatewayRoutesForTest(t, engine, handler)
	return engine, handler, registry
}

func serialChatRequest(t *testing.T, engine *gin.Engine) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(
		`{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}]}`,
	))
	request.Header.Set("Authorization", "Bearer gl-client")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	return response
}

func TestGatewaySerialAccount429SelectsNextCredential(t *testing.T) {
	forwarder := &scriptedForwarder{results: []UpstreamResult{
		{
			StatusCode: http.StatusTooManyRequests, Header: http.Header{"Retry-After": {"60"}},
			Body: []byte(`{"error":{"type":"rate_limit_error","code":"quota_exceeded"}}`),
			RequestWritten: true, DispatchState: execution.DispatchMaybeSent,
			ExecutionError: &execution.ErrorEvidence{
				Kind: execution.ErrorKindHTTP, Hint: execution.FailureHintRateLimited,
				OriginHint: execution.ErrorOriginUpstream, ScopeHint: execution.ErrorScopeCredential,
				StatusCode: http.StatusTooManyRequests,
				ReplaySafety: execution.ReplaySafetyRejectedBeforeProcessing,
			},
		},
		successfulAffinityResult(),
	}}
	engine, _, _ := serialGatewayHarness(t, forwarder, "https://upstream.example/v1", "sk-primary", "sk-fallback")
	response := serialChatRequest(t, engine)
	if response.Code != http.StatusOK || len(forwarder.inputs) != 2 ||
		forwarder.inputs[0].APIKey != "sk-primary" || forwarder.inputs[1].APIKey != "sk-fallback" {
		t.Fatalf("serial account failover response=%d inputs=%#v body=%s", response.Code, forwarder.inputs, response.Body.String())
	}
}

func TestGatewaySerialReplaySafe5xxRetriesSameCredential(t *testing.T) {
	forwarder := &scriptedForwarder{results: []UpstreamResult{
		{
			StatusCode: http.StatusServiceUnavailable, Header: make(http.Header),
			Body: []byte(`{"error":{"type":"service_unavailable_error","code":"server_is_overloaded"}}`),
			RequestWritten: true, DispatchState: execution.DispatchMaybeSent,
			ExecutionError: &execution.ErrorEvidence{
				Kind: execution.ErrorKindHTTP, Hint: execution.FailureHintHostError,
				OriginHint: execution.ErrorOriginUpstream, ScopeHint: execution.ErrorScopeGroup,
				StatusCode: http.StatusServiceUnavailable, Code: "server_is_overloaded",
				ReplaySafety: execution.ReplaySafetyRejectedBeforeProcessing,
			},
		},
		successfulAffinityResult(),
	}}
	engine, _, _ := serialGatewayHarness(t, forwarder, "https://upstream.example/v1", "sk-primary", "sk-fallback")
	response := serialChatRequest(t, engine)
	if response.Code != http.StatusOK || len(forwarder.inputs) != 2 ||
		forwarder.inputs[0].APIKey != "sk-primary" || forwarder.inputs[1].APIKey != "sk-primary" {
		t.Fatalf("replay-safe host retry response=%d inputs=%#v body=%s", response.Code, forwarder.inputs, response.Body.String())
	}
}

func seedDueSerialProbe(registry *state.CredentialRegistry, now time.Time) {
	registry.SchedulingState().WithLock(func(ledger *state.SchedulingLedger) {
		reset := now.Add(-2 * time.Minute)
		ledger.SerialGroups[1] = &state.SerialGroupState{
			CursorCredentialID: 2, CursorIdentityGeneration: 2,
			Accounts: map[uint]*state.SerialAccountState{
				1: {
					IdentityGeneration: 1, ResetAt: reset, CooldownUntil: reset,
					NextProbeAt: reset.Add(time.Minute),
				},
			},
		}
	})
}

func TestGatewaySerialFailbackUsesSafeListModelsProbe(t *testing.T) {
	upstream := fakeupstream.New(
		fakeupstream.Step{Status: http.StatusOK, Fixture: "models.json"},
		fakeupstream.Step{Status: http.StatusOK, Fixture: "success.json"},
	)
	defer upstream.Close()
	engine, _, registry := serialGatewayHarness(
		t, newTestExecutionForwarder(t), upstream.URL+"/v1", "sk-primary", "sk-fallback",
	)
	seedDueSerialProbe(registry, time.Now())

	response := serialChatRequest(t, engine)
	requests := upstream.Requests()
	if response.Code != http.StatusOK || len(requests) != 2 ||
		requests[0].Method != http.MethodGet || requests[0].Path != "/v1/models" || len(requests[0].Body) != 0 ||
		requests[0].Headers.Get("Authorization") != "Bearer sk-primary" ||
		requests[1].Method != http.MethodPost || requests[1].Path != "/v1/chat/completions" ||
		requests[1].Headers.Get("Authorization") != "Bearer sk-primary" {
		t.Fatalf("safe serial probe response=%d requests=%#v body=%s", response.Code, requests, response.Body.String())
	}
}

func TestGatewayFailedSerialProbeKeepsFallbackAndBacksOff(t *testing.T) {
	upstream := fakeupstream.New(
		fakeupstream.Step{Status: http.StatusInternalServerError, Fixture: "500.json"},
		fakeupstream.Step{Status: http.StatusOK, Fixture: "success.json"},
	)
	defer upstream.Close()
	engine, _, registry := serialGatewayHarness(
		t, newTestExecutionForwarder(t), upstream.URL+"/v1", "sk-primary", "sk-fallback",
	)
	seedDueSerialProbe(registry, time.Now())

	first := serialChatRequest(t, engine)
	requests := upstream.Requests()
	if first.Code != http.StatusOK || len(requests) != 2 ||
		requests[0].Method != http.MethodGet || requests[0].Headers.Get("Authorization") != "Bearer sk-primary" ||
		requests[1].Method != http.MethodPost || requests[1].Headers.Get("Authorization") != "Bearer sk-fallback" {
		t.Fatalf("failed safe probe did not keep the caller on fallback: response=%d requests=%#v", first.Code, requests)
	}
	checkpoint := registry.SchedulingState().CaptureCheckpoint()
	if len(checkpoint.SerialGroups) != 1 || len(checkpoint.SerialGroups[0].Accounts) != 1 ||
		checkpoint.SerialGroups[0].Accounts[0].ProbeFailures != 1 ||
		!checkpoint.SerialGroups[0].Accounts[0].NextProbeAt.After(time.Now()) {
		t.Fatalf("failed probe backoff state = %#v", checkpoint.SerialGroups)
	}
}

type quotaMismatchForwarder struct {
	inputs []ForwardInput
}

func (forwarder *quotaMismatchForwarder) Forward(_ context.Context, input ForwardInput) UpstreamResult {
	forwarder.inputs = append(forwarder.inputs, input)
	switch {
	case input.Operation == execution.OperationListModels:
		return UpstreamResult{
			StatusCode: http.StatusOK, Header: make(http.Header), Body: []byte(`{"data":[]}`),
			RequestWritten: true, DispatchState: execution.DispatchMaybeSent,
		}
	case input.APIKey == "sk-primary":
		return UpstreamResult{
			StatusCode: http.StatusTooManyRequests, Header: http.Header{"Retry-After": {"1800"}},
			Body: []byte(`{"error":{"type":"rate_limit_error","code":"quota_exceeded"}}`),
			RequestWritten: true, DispatchState: execution.DispatchMaybeSent,
			ExecutionError: &execution.ErrorEvidence{
				Kind: execution.ErrorKindHTTP, Hint: execution.FailureHintRateLimited,
				OriginHint: execution.ErrorOriginUpstream, ScopeHint: execution.ErrorScopeCredential,
				StatusCode: http.StatusTooManyRequests, RetryAfter: 30 * time.Minute,
				ReplaySafety: execution.ReplaySafetyRejectedBeforeProcessing,
			},
		}
	default:
		return successfulAffinityResult()
	}
}

func (forwarder *quotaMismatchForwarder) ForwardStream(
	ctx context.Context,
	input ForwardInput,
	_ http.ResponseWriter,
) UpstreamResult {
	return forwarder.Forward(ctx, input)
}

func TestGatewayListModelsSuccessDoesNotPromoteQuotaLimitedInference(t *testing.T) {
	forwarder := &quotaMismatchForwarder{}
	engine, _, registry := serialGatewayHarness(
		t, forwarder, "https://upstream.example/v1", "sk-primary", "sk-fallback",
	)
	seedDueSerialProbe(registry, time.Now())

	response := serialChatRequest(t, engine)
	if response.Code != http.StatusOK || len(forwarder.inputs) != 3 ||
		forwarder.inputs[0].Operation != execution.OperationListModels || forwarder.inputs[0].APIKey != "sk-primary" ||
		forwarder.inputs[1].Operation != execution.OperationChatCompletion || forwarder.inputs[1].APIKey != "sk-primary" ||
		forwarder.inputs[2].Operation != execution.OperationChatCompletion || forwarder.inputs[2].APIKey != "sk-fallback" {
		t.Fatalf("list-models success promoted a quota-limited inference account: status=%d inputs=%#v body=%s",
			response.Code, forwarder.inputs, response.Body.String())
	}
	checkpoint := registry.SchedulingState().CaptureCheckpoint()
	if len(checkpoint.SerialGroups) != 1 || checkpoint.SerialGroups[0].CursorCredentialID != 2 ||
		len(checkpoint.SerialGroups[0].Accounts) != 1 || checkpoint.SerialGroups[0].Accounts[0].CredentialID != 1 ||
		checkpoint.SerialGroups[0].Accounts[0].ProbeFailures != 1 ||
		!checkpoint.SerialGroups[0].Accounts[0].NextProbeAt.After(time.Now()) {
		t.Fatalf("account-scoped inference 429 did not preserve fallback/backoff: %#v", checkpoint.SerialGroups)
	}
}
