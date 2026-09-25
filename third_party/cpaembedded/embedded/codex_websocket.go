package embedded

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptrace"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	internalconfig "github.com/router-for-me/CLIProxyAPI/v7/internal/config"
	internalexecutor "github.com/router-for-me/CLIProxyAPI/v7/internal/runtime/executor"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/auth"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/executor"
	"github.com/router-for-me/CLIProxyAPI/v7/sdk/proxyutil"
	sdktranslator "github.com/router-for-me/CLIProxyAPI/v7/sdk/translator"
)

const (
	CodexWSNotSent            = "not_sent"
	CodexWSMaybeSent          = "maybe_sent"
	defaultCodexWSTurnTimeout = 5 * time.Minute
	defaultCodexWSMaxBytes    = 10 << 20
)

// CodexWSSessionOptions holds the credential, API proxy root URL, and outbound proxy selected by the caller.
// ProxyURL receives the direct or proxy URL selected by the existing proxy policy; it does not read environment proxy settings.
type CodexWSSessionOptions struct {
	CredentialID    string
	Credential      CodexCredential
	BaseURL         string
	ProxyURL        string
	TurnTimeout     time.Duration
	MaxRequestBytes int
	MaxEventBytes   int
	Headers         http.Header
	// ObserveHeaders delivers handshake headers before events and failed-response headers before termination.
	// The callback must return promptly and must not wait for the turn to finish.
	ObserveHeaders func(http.Header, time.Time)
}

// CodexWSTurnResult describes only the current turn; it does not perform health, quota, or log accounting.
type CodexWSTurnResult struct {
	ResponseID       string
	Status           string
	Usage            json.RawMessage
	Headers          http.Header
	HeaderObservedAt time.Time
	DispatchState    string
}

// CodexWSError text contains no upstream responses, credentials, addresses, or proxy passwords.
// UpstreamType, UpstreamCode, HTTPStatus, and RetryAfter provide only the error-classification evidence required by the caller.
type CodexWSError struct {
	Code          string
	UpstreamType  string
	UpstreamCode  string
	HTTPStatus    int
	RetryAfter    time.Duration
	DispatchState string
	cause         error
}

func (err *CodexWSError) Error() string { return "codex websocket: " + err.Code }
func (err *CodexWSError) Unwrap() error { return err.cause }

func codexWSError(code string) *CodexWSError {
	return &CodexWSError{Code: code, DispatchState: CodexWSNotSent}
}

// CodexWSSession is a caller-exclusive Codex upstream session.
type CodexWSSession struct {
	auth              *cliproxyauth.Auth
	inner             *internalexecutor.CodexWebsocketsExecutor
	id                string
	options           CodexWSSessionOptions
	resource          *codexWSResource
	mu                sync.Mutex
	running           bool
	started           bool
	closed            bool
	bound             bool
	cancel            context.CancelFunc
	closeConnection   func() error
	pendingConnection net.Conn
	closeDone         chan struct{}
	closeErr          error
}

// NewCodexWSSession only creates a handle; it does not connect, refresh tokens, or retain the refresh token.
func NewCodexWSSession(options CodexWSSessionOptions) (*CodexWSSession, error) {
	if strings.TrimSpace(options.CredentialID) == "" || validateCredential(options.Credential) != nil {
		return nil, codexWSError("invalid_session_options")
	}
	endpoints, err := ResolveCodexAPIEndpoints(options.BaseURL)
	if err != nil {
		return nil, codexWSError("invalid_session_options")
	}
	proxy, err := proxyutil.Parse(options.ProxyURL)
	if err != nil || (proxy.Mode != proxyutil.ModeDirect && proxy.Mode != proxyutil.ModeProxy) {
		return nil, codexWSError("invalid_proxy")
	}
	if proxy.URL != nil && (proxy.URL.RawQuery != "" || proxy.URL.Fragment != "" || (proxy.URL.Path != "" && proxy.URL.Path != "/")) {
		return nil, codexWSError("invalid_proxy")
	}
	if options.TurnTimeout < 0 || options.MaxRequestBytes < 0 || options.MaxEventBytes < 0 {
		return nil, codexWSError("invalid_session_options")
	}
	if options.TurnTimeout == 0 {
		options.TurnTimeout = defaultCodexWSTurnTimeout
	}
	if options.MaxRequestBytes == 0 {
		options.MaxRequestBytes = defaultCodexWSMaxBytes
	}
	if options.MaxEventBytes == 0 {
		options.MaxEventBytes = defaultCodexWSMaxBytes
	}
	auth := NewCodexAuth(options.CredentialID, options.Credential, endpoints.ExecutionBase)
	delete(auth.Metadata, "refresh_token")
	auth.ProxyURL = proxy.Raw
	options.Credential = CodexCredential{}
	options.Headers = options.Headers.Clone()
	session := &CodexWSSession{
		auth: auth, id: codexWSSessionIDPrefix + uuid.NewString(), options: options, closeDone: make(chan struct{}),
		inner: internalexecutor.NewCodexWebsocketsExecutor(&internalconfig.Config{
			// Disable SDK startup buffering so ExecuteStream returns the handshake before native events are delivered.
			Codex: internalconfig.CodexConfig{ModelLevelCooling: true, StreamBootstrapBuffering: false},
		}),
	}
	session.resource = &codexWSResource{session: session}
	return session, nil
}

// ExecuteTurn synchronously executes one turn; emit is invoked in native upstream event order and chat history is not accumulated.
// emit may be nil; non-nil callbacks must return promptly, respond to ctx, and must not wait for the turn to finish.
// Event size is checked after the SDK reads; it is not a memory limit for raw SDK frame reads.
func (s *CodexWSSession) ExecuteTurn(ctx context.Context, payload json.RawMessage, emit func(context.Context, json.RawMessage) error) (CodexWSTurnResult, error) {
	result := CodexWSTurnResult{DispatchState: CodexWSNotSent}
	if s == nil {
		return result, codexWSError("session_closed")
	}
	model, previous, err := s.validateRequest(payload)
	if err != nil {
		return result, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if ctx.Err() != nil {
		return result, codexWSContextError(ctx.Err(), CodexWSNotSent)
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return result, codexWSError("session_closed")
	}
	if s.running {
		s.mu.Unlock()
		return result, codexWSError("session_busy")
	}
	if !s.started && previous != "" {
		s.mu.Unlock()
		return result, codexWSError("continuation_requires_session")
	}
	var turnCtx context.Context
	var cancel context.CancelFunc
	if _, bounded := ctx.Deadline(); bounded {
		// The gateway deadline for each turn is the business deadline; the creation-time default must not limit later configuration increases.
		turnCtx, cancel = context.WithCancel(ctx)
	} else {
		turnCtx, cancel = context.WithTimeout(ctx, s.options.TurnTimeout)
	}
	s.cancel, s.running = cancel, true
	reuse := s.started
	s.mu.Unlock()
	cleanupDone := make(chan struct{})
	stop := context.AfterFunc(turnCtx, func() { s.invalidate(true); close(cleanupDone) })
	defer func() {
		if !stop() {
			<-cleanupDone
		}
		cancel()
		s.mu.Lock()
		closed := s.closed
		s.mu.Unlock()
		// Even if closing races with SDK session creation, clean up the empty registration left by this invocation.
		if closed {
			s.inner.CloseExecutionSession(s.id)
		}
		s.mu.Lock()
		s.cancel, s.running = nil, false
		s.mu.Unlock()
	}()
	turnCtx = cliproxyexecutor.WithUpstreamAttemptTracker(turnCtx)
	// Gorilla's Upgrade reader sets only a deadline; capture connections not yet bound to the lifecycle
	// so proactive cancellation ends TLS/WS handshakes without waiting for the handshake timeout.
	turnCtx = httptrace.WithClientTrace(turnCtx, &httptrace.ClientTrace{
		GotConn: func(info httptrace.GotConnInfo) { s.trackDialConnection(info.Conn) },
	})
	if reuse {
		turnCtx = cliproxyexecutor.WithRequiredUpstreamWebsocket(turnCtx)
	}
	observation := codexWSTurnObservation{
		result: &result, maxBytes: s.options.MaxEventBytes, emit: emit, cancel: cancel,
		failSession: func() { s.invalidate(false) },
	}
	headersReady := make(chan struct{})
	stream, executionErr := s.inner.ExecuteStream(turnCtx, s.auth, cliproxyexecutor.Request{
		Model: model, Payload: append([]byte(nil), payload...), Format: sdktranslator.FormatOpenAIResponse,
	}, cliproxyexecutor.Options{
		Stream: true, Headers: normalizedCodexHeaders(s.options.Headers), SourceFormat: sdktranslator.FormatOpenAIResponse, ResponseFormat: sdktranslator.FormatOpenAIResponse,
		Metadata:           map[string]any{cliproxyexecutor.ExecutionSessionMetadataKey: s.id},
		ExecutionLifecycle: s.resource,
		WebSocketResponseObserver: func(ctx context.Context, event cliproxyexecutor.WebSocketResponseEvent) {
			// The SDK reading goroutine can run before ExecuteStream returns; do not deliver events until the handshake hand-off completes.
			<-headersReady
			observation.observe(ctx, event)
		},
	})
	if stream != nil {
		result.Headers = stream.Headers.Clone()
		if len(result.Headers) > 0 {
			result.HeaderObservedAt = time.Now()
			if s.options.ObserveHeaders != nil {
				s.options.ObserveHeaders(result.Headers.Clone(), result.HeaderObservedAt)
			}
		}
	}
	close(headersReady)
	if stream != nil {
		// Native JSON is delivered through the observer. Drain the SDK conversion output to ensure full turn cleanup.
		for chunk := range stream.Chunks {
			if chunk.Err != nil && executionErr == nil {
				executionErr = chunk.Err
			}
		}
	}
	s.mu.Lock()
	bound := s.bound
	s.mu.Unlock()
	if bound && cliproxyexecutor.UpstreamAttempted(turnCtx) {
		result.DispatchState = CodexWSMaybeSent
	}
	if observation.err != nil {
		s.invalidate(true)
		observation.err.DispatchState = result.DispatchState
		return result, observation.err
	}
	if turnCtx.Err() != nil {
		s.invalidate(true)
		return result, codexWSContextError(turnCtx.Err(), result.DispatchState)
	}
	if executionErr != nil {
		// The connection deadline may fire before the context timer; retain timeout categorisation.
		var timeout net.Error
		if errors.As(executionErr, &timeout) && timeout.Timeout() {
			s.invalidate(true)
			return result, codexWSContextError(context.DeadlineExceeded, result.DispatchState)
		}
		failure := codexWSError("upstream_error")
		failure.DispatchState, failure.UpstreamCode = result.DispatchState, observation.upstreamCode
		failure.UpstreamType, failure.RetryAfter = codexWSErrorMetadata(executionErr)
		var status interface{ StatusCode() int }
		if errors.As(executionErr, &status) {
			failure.HTTPStatus = status.StatusCode()
		}
		var headers interface{ Headers() http.Header }
		if errors.As(executionErr, &headers) {
			result.Headers = headers.Headers().Clone()
			if len(result.Headers) > 0 {
				result.HeaderObservedAt = time.Now()
				if s.options.ObserveHeaders != nil {
					s.options.ObserveHeaders(result.Headers.Clone(), result.HeaderObservedAt)
				}
			}
		}
		// Keep the existing connection intact when SDK request preprocessing fails without reaching the upstream.
		if cliproxyexecutor.UpstreamAttempted(turnCtx) || cliproxyexecutor.IsUpstreamWebsocketReplayRequired(executionErr) {
			s.invalidate(true)
		}
		return result, failure
	}
	if result.Status != "completed" || result.ResponseID == "" {
		s.invalidate(true)
		failure := codexWSError("incomplete_response")
		failure.DispatchState, failure.UpstreamCode = result.DispatchState, observation.upstreamCode
		return result, failure
	}
	s.mu.Lock()
	s.started = true
	s.mu.Unlock()
	return result, nil
}

func codexWSErrorMetadata(err error) (string, time.Duration) {
	upstreamType := ""
	for current := err; current != nil; current = errors.Unwrap(current) {
		var payload struct {
			Type  string `json:"type"`
			Error struct {
				Type string `json:"type"`
			} `json:"error"`
		}
		if json.Unmarshal([]byte(current.Error()), &payload) != nil {
			continue
		}
		upstreamType = payload.Error.Type
		if upstreamType == "" {
			upstreamType = payload.Type
		}
		upstreamType = safeCodexWSCode(upstreamType)
		if upstreamType != "" {
			break
		}
	}
	var retry interface{ RetryAfter() *time.Duration }
	if errors.As(err, &retry) && retry != nil {
		if value := retry.RetryAfter(); value != nil && *value > 0 {
			return upstreamType, *value
		}
	}
	return upstreamType, 0
}

func (s *CodexWSSession) validateRequest(payload []byte) (string, string, error) {
	if len(payload) > s.options.MaxRequestBytes {
		return "", "", codexWSError("request_too_large")
	}
	var root map[string]json.RawMessage
	if json.Unmarshal(payload, &root) != nil || root == nil {
		return "", "", codexWSError("invalid_request")
	}
	var model, previous string
	if json.Unmarshal(root["model"], &model) != nil || strings.TrimSpace(model) == "" {
		return "", "", codexWSError("invalid_request")
	}
	if raw, ok := root["previous_response_id"]; ok && json.Unmarshal(raw, &previous) != nil {
		return "", "", codexWSError("invalid_request")
	}
	// The initial version supports only native single-turn generation; it does not silently drop multi-stream or background-execution semantics.
	for _, key := range []string{"type", "stream_id", "conversation"} {
		if _, ok := root[key]; ok {
			return "", "", codexWSError("unsupported_request")
		}
	}
	if raw, ok := root["background"]; ok {
		var background bool
		if json.Unmarshal(raw, &background) != nil || background {
			return "", "", codexWSError("unsupported_request")
		}
	}
	return model, previous, nil
}

// Done notifies the caller when this Session becomes invalid or closes.
func (s *CodexWSSession) Done() <-chan struct{} {
	if s == nil || s.closeDone == nil {
		closed := make(chan struct{})
		close(closed)
		return closed
	}
	return s.closeDone
}

// Close may be called from an event callback; it closes the connection and cancels the current turn, after which ExecuteTurn returns.
func (s *CodexWSSession) Close() error {
	if s == nil {
		return nil
	}
	s.invalidate(true)
	<-s.closeDone
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closeErr
}

func (s *CodexWSSession) trackDialConnection(connection net.Conn) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		s.recordCloseError(connection.Close())
		return
	}
	previous := s.pendingConnection
	s.pendingConnection = connection
	s.mu.Unlock()
	if previous != nil {
		s.recordCloseError(previous.Close())
	}
}

func (s *CodexWSSession) recordCloseError(err error) {
	if err == nil || errors.Is(err, net.ErrClosed) {
		return
	}
	s.mu.Lock()
	s.closeErr = codexWSError("close_failed")
	s.mu.Unlock()
}

func (s *CodexWSSession) invalidate(cancelActive bool) {
	s.mu.Lock()
	if s.closed {
		cancel := s.cancel
		s.mu.Unlock()
		if cancelActive && cancel != nil {
			cancel()
		}
		return
	}
	s.closed = true
	cancel, closeConnection := s.cancel, s.closeConnection
	pendingConnection := s.pendingConnection
	s.closeConnection = nil
	s.pendingConnection = nil
	s.mu.Unlock()
	if cancelActive && cancel != nil {
		cancel()
	}
	if pendingConnection != nil {
		s.recordCloseError(pendingConnection.Close())
	}
	if closeConnection != nil {
		s.recordCloseError(closeConnection())
	}
	s.inner.CloseExecutionSession(s.id)
	close(s.closeDone)
}

type codexWSResource struct{ session *CodexWSSession }

// Bind accepts only the first connection; if the SDK redials and binds again, reject it before the second business send.
func (resource *codexWSResource) Bind(closeConnection func() error) error {
	s := resource.session
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.bound {
		return codexWSError("session_closed")
	}
	s.bound, s.closeConnection = true, closeConnection
	s.pendingConnection = nil
	return nil
}

func (resource *codexWSResource) End(string) { resource.session.invalidate(false) }

func codexWSContextError(err error, dispatch string) *CodexWSError {
	code := "canceled"
	if errors.Is(err, context.DeadlineExceeded) {
		code = "timeout"
	}
	return &CodexWSError{Code: code, DispatchState: dispatch, cause: err}
}

type codexWSTurnObservation struct {
	result       *CodexWSTurnResult
	maxBytes     int
	emit         func(context.Context, json.RawMessage) error
	cancel       context.CancelFunc
	err          *CodexWSError
	upstreamCode string
	failSession  func()
}

func (o *codexWSTurnObservation) observe(ctx context.Context, event cliproxyexecutor.WebSocketResponseEvent) {
	if o.err != nil || ctx.Err() != nil {
		return
	}
	fail := func(code string) { o.err = codexWSError(code); o.cancel() }
	if len(event.Payload) > o.maxBytes {
		fail("event_too_large")
		return
	}
	var envelope struct {
		Type     string `json:"type"`
		Response struct {
			ID     string          `json:"id"`
			Status string          `json:"status"`
			Usage  json.RawMessage `json:"usage"`
			Error  struct {
				Code string `json:"code"`
			} `json:"error"`
		} `json:"response"`
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if json.Unmarshal(event.Payload, &envelope) != nil || envelope.Type == "" {
		fail("invalid_event")
		return
	}
	if envelope.Response.ID != "" {
		if o.result.ResponseID != "" && o.result.ResponseID != envelope.Response.ID {
			fail("invalid_event")
			return
		}
		o.result.ResponseID = envelope.Response.ID
	}
	switch envelope.Type {
	case "response.completed":
		o.result.Status, o.result.Usage = "completed", envelope.Response.Usage
	case "response.done":
		o.result.Status, o.result.Usage = envelope.Response.Status, envelope.Response.Usage
		o.upstreamCode = safeCodexWSCode(envelope.Response.Error.Code)
	case "response.incomplete":
		o.result.Status, o.result.Usage = "incomplete", envelope.Response.Usage
	case "response.failed", "error":
		o.result.Status, o.result.Usage = "failed", envelope.Response.Usage
		o.upstreamCode = safeCodexWSCode(envelope.Error.Code)
		if o.upstreamCode == "" {
			o.upstreamCode = safeCodexWSCode(envelope.Response.Error.Code)
		}
	}
	if o.emit != nil {
		if err := o.emit(ctx, append(json.RawMessage(nil), event.Payload...)); err != nil {
			fail("event_consumer_failed")
		}
	}
	// Release the failed session first so the SDK returns the original error classification rather than logging the error body during disconnect.
	if o.result.Status == "failed" || o.result.Status == "incomplete" || (envelope.Type == "response.done" && o.result.Status != "completed") {
		o.failSession()
	}
}

func safeCodexWSCode(code string) string {
	if len(code) > 128 {
		return ""
	}
	for _, char := range code {
		if !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '_' || char == '-' || char == '.') {
			return ""
		}
	}
	return code
}
