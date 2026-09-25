package main

import (
	"bytes"
	"maps"
	"strings"
	"sync"
	"testing"

	"github.com/sirupsen/logrus"

	"gpt-load/internal/platform/redact"
	"gpt-load/internal/subscription/providers/codex"
)

type runtimeLogCapture struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (capture *runtimeLogCapture) Write(body []byte) (int, error) {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	return capture.buf.Write(body)
}

func (capture *runtimeLogCapture) String() string {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	return capture.buf.String()
}

func (*runtimeLogCapture) Levels() []logrus.Level { return logrus.AllLevels }

func (capture *runtimeLogCapture) Fire(entry *logrus.Entry) error {
	// As with the Windows Event Log, format and write within the hook rather than waiting for final output.
	formatted, err := entry.Logger.Formatter.Format(entry)
	if err != nil {
		return err
	}
	_, err = capture.Write(formatted)
	return err
}

func TestCodexWSRedactionPrecedesRuntimeLogHooks(t *testing.T) {
	// The non-parallel test keeps the hook installed during package initialisation and only appends the runtime log chain temporarily.
	logger := logrus.StandardLogger()
	originalOutput, originalHooks := logger.Out, maps.Clone(logger.Hooks)
	t.Cleanup(func() {
		logger.SetOutput(originalOutput)
		logger.ReplaceHooks(originalHooks)
	})
	var output, sink runtimeLogCapture
	logger.SetOutput(&output)
	logger.AddHook(redact.NewHook(redact.New()))
	logger.AddHook(&sink)

	// Reproduce the order in which the Session is first created after service startup; creating its handle does not connect.
	session, err := codex.NewWSSession(codex.WSSessionOptions{
		CredentialID: "test", Credential: codex.Credential{
			Type: codex.Provider, AccessToken: "test-access", RefreshToken: "test-refresh", AccountID: "test-account",
		}, ProxyURL: "direct",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Error(err)
		}
	})
	const marker = "private-close-reason-marker"
	// Use the pinned SDK disconnection-log format to verify the combined bridge wrapper and project log chain.
	logger.Infof("codex websockets: upstream disconnected session=gptload-codex-ws-log-order-test auth=test url=wss://example.test/responses reason=read_error err=websocket: close 1008 (policy violation): %s", marker)
	for name, capture := range map[string]*runtimeLogCapture{"output": &output, "sink": &sink} {
		message := capture.String()
		if strings.Contains(message, marker) {
			t.Errorf("close reason leaked into %s", name)
		}
		if !strings.Contains(message, "session=[REDACTED]") ||
			!strings.Contains(message, "ws_close_code=1008") ||
			!strings.Contains(message, "error_class=websocket_closed") {
			t.Errorf("missing redacted session or safe close diagnostics in %s", name)
		}
	}
}
