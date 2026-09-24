package gateway

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"gpt-load/internal/scheduler"
	"gpt-load/internal/state"
)

func TestConfiguredSelectorChangesWhichAccountServesRequest(t *testing.T) {
	forwarder := &scriptedForwarder{results: []UpstreamResult{{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       []byte(`{"id":"ok","choices":[]}`),
	}}}
	handler, _, _ := newHandlerForTest(t, forwarder, "sk-primary", "sk-secondary")
	if err := handler.ConfigureSelectionFactory(func(snapshot *state.ConfigSnapshot, source scheduler.CredentialSource, query scheduler.Query) scheduler.SelectionIterator {
		query.PreferredCredentialID = 2
		return scheduler.New(snapshot, source, query)
	}); err != nil {
		t.Fatal(err)
	}
	engine := gin.New()
	bindGatewayRoutesForTest(t, engine, handler)
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}]}`))
	request.Header.Set("Authorization", "Bearer gl-client")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	if response.Code != http.StatusOK || len(forwarder.inputs) != 1 || forwarder.inputs[0].Credential.ID != 2 {
		t.Fatalf("status = %d, attempts = %d, selected account = %v; body = %s", response.Code, len(forwarder.inputs), selectedCredentialID(forwarder.inputs), response.Body.String())
	}
}

func TestSelectorReturningNoCandidateNeverCallsUpstream(t *testing.T) {
	forwarder := &scriptedForwarder{results: []UpstreamResult{{StatusCode: http.StatusOK}}}
	handler, _, _ := newHandlerForTest(t, forwarder, "sk-primary")
	if err := handler.ConfigureSelectionFactory(func(*state.ConfigSnapshot, scheduler.CredentialSource, scheduler.Query) scheduler.SelectionIterator {
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	engine := gin.New()
	bindGatewayRoutesForTest(t, engine, handler)
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}]}`))
	request.Header.Set("Authorization", "Bearer gl-client")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	if response.Code == http.StatusOK || len(forwarder.inputs) != 0 {
		t.Fatalf("selector failure dispatched: status = %d, attempts = %d", response.Code, len(forwarder.inputs))
	}
}

func selectedCredentialID(inputs []ForwardInput) any {
	if len(inputs) == 0 {
		return nil
	}
	return inputs[0].Credential.ID
}
