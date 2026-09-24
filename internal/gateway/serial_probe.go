package gateway

import (
	"net/http"
	"sort"
	"time"

	"gpt-load/internal/dialect"
	"gpt-load/internal/execution"
	"gpt-load/internal/health"
	"gpt-load/internal/protocol"
	"gpt-load/internal/scheduler"
	"gpt-load/internal/state"
)

type serialProbeReporter interface {
	RecordSerialProbe(scheduler.Selection, scheduler.SerialProbeResult, time.Time)
}

type serialAttemptReporter interface {
	RecordAttempt(scheduler.Selection, health.Decision, time.Time, bool)
}

func serialListModelsRoute(
	group state.GroupView,
	dialects dialect.Set,
) (protocol.Protocol, execution.RouteMode, dialect.Dialect, bool) {
	protocols := append([]protocol.Protocol(nil), group.ClientProtocols...)
	sort.Slice(protocols, func(i, j int) bool { return protocols[i] < protocols[j] })
	if group.ValidationProtocol.Valid() {
		protocols = append([]protocol.Protocol{group.ValidationProtocol}, protocols...)
	}
	seen := make(map[protocol.Protocol]struct{}, len(protocols))
	for _, candidate := range protocols {
		if _, duplicate := seen[candidate]; duplicate {
			continue
		}
		seen[candidate] = struct{}{}
		descriptor := dialects[candidate]
		if descriptor == nil {
			continue
		}
		mode, supported := group.ResolvedTarget.Mode(candidate, execution.OperationListModels)
		if supported {
			return candidate, execution.RouteMode(mode), descriptor, true
		}
	}
	return "", "", nil, false
}

func reportSerialProbe(
	iterator scheduler.SelectionIterator,
	selection scheduler.Selection,
	result scheduler.SerialProbeResult,
	now time.Time,
) {
	if reporter, ok := iterator.(serialProbeReporter); ok {
		reporter.RecordSerialProbe(selection, result, now)
	}
}

func reportSerialAttempt(
	iterator scheduler.SelectionIterator,
	selection scheduler.Selection,
	decision health.Decision,
	now time.Time,
	willRetry bool,
) {
	if reporter, ok := iterator.(serialAttemptReporter); ok {
		reporter.RecordAttempt(selection, decision, now, willRetry)
	}
}

func serialListModelsProbeSucceeded(result UpstreamResult) bool {
	return result.Err == nil && result.ExecutionError == nil &&
		result.StatusCode >= http.StatusOK && result.StatusCode < http.StatusMultipleChoices
}
