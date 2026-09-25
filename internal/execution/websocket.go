package execution

import (
	"context"
	"net/http"
	"time"
)

// WebsocketCapabilities declares native Responses WS capabilities promised by the execution path, rather than deriving them from HTTP storage declarations.
type WebsocketCapabilities struct {
	Native          bool
	Continuation    bool
	StoredResponses bool
	Prewarm         bool
	Multiplex       bool
}

func (c WebsocketCapabilities) Supports(required WebsocketCapabilities) bool {
	return c.Native && (!required.Continuation || c.Continuation) &&
		(!required.StoredResponses || c.StoredResponses) &&
		(!required.Prewarm || c.Prewarm) && (!required.Multiplex || c.Multiplex)
}

// WebsocketResult reports only this turn's send evidence and errors; native events provide usage and response IDs.
type WebsocketResult struct {
	DispatchState    DispatchState
	Header           http.Header
	HeaderObservedAt time.Time
	Error            *ErrorEvidence
}

// WebsocketSession fixes the selected target; the gateway guarantees same-stream order while supported distinct streams may run concurrently.
// payload is Responses Create parameters without type; the native stream_id is retained.
// emit synchronously consumes one complete native JSON event, must observe ctx, and terminates the connection on error.
type WebsocketSession interface {
	ExecuteTurn(ctx context.Context, payload []byte, emit func(context.Context, []byte) error) WebsocketResult
	Done() <-chan struct{}
	Close() error
}

// WebsocketOpener does not choose routes, replay business traffic, or fall back to HTTP.
type WebsocketOpener interface {
	OpenWebsocket(context.Context, AttemptSpec) (WebsocketSession, WebsocketResult)
}
