package embedded

import (
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

const codexWSSessionIDPrefix = "gptload-codex-ws-"

// Register before the application runtime's generic redaction and log-writing hooks, so the session prefix cannot be
// obscured before matching or the raw error body persisted by another hook.
func init() {
	logrus.AddHook(codexWSLogHook{})
}

// CPA v7.2.151 writes the CloseError body to the default logger before notifying the lifecycle.
// Process only the log entry for this wrapper session; do not alter the global level, output, or logs from other executors.
type codexWSLogHook struct{}

func (codexWSLogHook) Levels() []logrus.Level { return []logrus.Level{logrus.InfoLevel} }

func (codexWSLogHook) Fire(entry *logrus.Entry) error {
	if !strings.HasPrefix(entry.Message, "codex websockets: upstream disconnected session="+codexWSSessionIDPrefix) {
		return nil
	}
	message, rawError, found := strings.Cut(entry.Message, " err=")
	if !found {
		return nil
	}
	entry.Message = message
	entry.Data["error_class"] = "upstream_error"
	// Extract only Gorilla CloseError's numeric close code; unknown formats do not retain the error body either.
	if detail, ok := strings.CutPrefix(rawError, "websocket: close "); ok {
		if end := strings.IndexAny(detail, " :"); end >= 0 {
			detail = detail[:end]
		}
		if code, err := strconv.Atoi(detail); err == nil && code >= 1000 && code < 5000 {
			entry.Data["ws_close_code"] = code
			entry.Data["error_class"] = "websocket_closed"
		}
	}
	return nil
}
