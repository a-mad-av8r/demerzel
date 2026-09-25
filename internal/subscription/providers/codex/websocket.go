package codex

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	cpaembedded "github.com/router-for-me/CLIProxyAPI/v7/gptload-embedded/embedded"
)

const (
	WSNotSent   = "not_sent"
	WSMaybeSent = "maybe_sent"
)

// WSSessionOptions fixes the caller's selected credential, API proxy root, and outbound proxy.
// ProxyURL reuses an existing proxy policy's direct or proxy URL; callers must resolve environment proxies first.
type WSSessionOptions struct {
	CredentialID    string
	Credential      Credential
	BaseURL         string
	ProxyURL        string
	TurnTimeout     time.Duration
	MaxRequestBytes int
	MaxEventBytes   int
	Headers         http.Header
	ObserveHeaders  func(http.Header, time.Time)
}

// WSTurnResult retains upstream JSON for Usage and uses nil when it is missing; do not fabricate zero usage.
type WSTurnResult struct {
	ResponseID       string
	Status           string
	Usage            json.RawMessage
	Headers          http.Header
	HeaderObservedAt time.Time
	DispatchState    string
}

// WSError exposes stable classification and send evidence; error text excludes upstream bodies and credentials.
type WSError struct {
	Code          string
	UpstreamType  string
	UpstreamCode  string
	HTTPStatus    int
	RetryAfter    time.Duration
	DispatchState string
	cause         error
}

func (err *WSError) Error() string { return "codex websocket: " + err.Code }
func (err *WSError) Unwrap() error { return err.cause }

// WSSession is independent of the existing HTTP Executor and does not register with the data plane or global lifecycle.
type WSSession struct{ bridge *cpaembedded.CodexWSSession }

// NewWSSession creates an independent handle; the first ExecuteTurn establishes the upstream connection.
func NewWSSession(options WSSessionOptions) (*WSSession, error) {
	bridge, err := cpaembedded.NewCodexWSSession(cpaembedded.CodexWSSessionOptions{
		CredentialID: options.CredentialID, Credential: credentialToBridge(options.Credential),
		BaseURL: options.BaseURL, ProxyURL: options.ProxyURL,
		Headers:        options.Headers.Clone(),
		ObserveHeaders: options.ObserveHeaders,
		TurnTimeout:    options.TurnTimeout, MaxRequestBytes: options.MaxRequestBytes, MaxEventBytes: options.MaxEventBytes,
	})
	if err != nil {
		return nil, wsErrorFromBridge(err)
	}
	return &WSSession{bridge: bridge}, nil
}

// ExecuteTurn synchronously executes native Responses Create without referencing a previous response on the first turn.
// emit receives native JSON events in order, must return promptly and observe ctx; nil means ignore the event.
// The caller stores response IDs and starts serial continuations through the same Session.
func (s *WSSession) ExecuteTurn(ctx context.Context, payload json.RawMessage, emit func(context.Context, json.RawMessage) error) (WSTurnResult, error) {
	var bridge *cpaembedded.CodexWSSession
	if s != nil {
		bridge = s.bridge
	}
	result, err := bridge.ExecuteTurn(ctx, payload, emit)
	return WSTurnResult{
		ResponseID: result.ResponseID, Status: result.Status, Usage: result.Usage,
		Headers: result.Headers, DispatchState: result.DispatchState,
		HeaderObservedAt: result.HeaderObservedAt,
	}, wsErrorFromBridge(err)
}

// Done notifies callers after this Session becomes invalid or closes.
func (s *WSSession) Done() <-chan struct{} {
	var bridge *cpaembedded.CodexWSSession
	if s != nil {
		bridge = s.bridge
	}
	return bridge.Done()
}

// Close closes this Session idempotently without affecting other Sessions.
func (s *WSSession) Close() error {
	if s == nil {
		return nil
	}
	return wsErrorFromBridge(s.bridge.Close())
}

func wsErrorFromBridge(err error) error {
	if err == nil {
		return nil
	}
	var failure *cpaembedded.CodexWSError
	if errors.As(err, &failure) {
		return &WSError{
			Code: failure.Code, UpstreamType: failure.UpstreamType, UpstreamCode: failure.UpstreamCode,
			HTTPStatus: failure.HTTPStatus, RetryAfter: failure.RetryAfter,
			DispatchState: failure.DispatchState, cause: failure.Unwrap(),
		}
	}
	return &WSError{Code: "internal_error", DispatchState: WSMaybeSent}
}
