package codex

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/buger/jsonparser"
)

// Compare original absolute time and the countdown from the same event separately; do not mix in local receipt time.
type websocketQuotaWindowAnchor struct {
	windowSeconds     int64
	resetAtSeconds    int64
	resetAfterSeconds int64
}

type websocketQuotaRateObservation struct {
	windows              []quotaWindow
	comparisonWindows    map[string]websocketQuotaWindowAnchor
	comparisonIncomplete bool
}

// NormalizeWebsocketQuotaWindows reads native Codex quota events without changing messages sent to clients.
// Named additional WS quota sources are resolved from existing snapshots and cannot derive SourceID from HTTP response-header namespaces.
func NormalizeWebsocketQuotaWindows(payload []byte, observedAt time.Time) []quotaWindow {
	kind, err := jsonparser.GetString(payload, "type")
	if err != nil || kind != "codex.rate_limits" {
		return nil
	}
	event, ok := decodeWebsocketQuotaEvent(payload)
	if !ok || cleanString(event["type"]) != "codex.rate_limits" {
		return nil
	}

	var additionalRates map[string]any
	additionalValid := true
	if rawAdditional, present := event["additional_rate_limits"]; present && rawAdditional != nil {
		additionalRates, additionalValid = object(rawAdditional)
	}
	if additionalValid && len(additionalRates) > 8 {
		return nil
	}
	additional := make([]quotaWindow, 0, 2*len(additionalRates))
	comparisonWindows := make([]websocketQuotaWindowAnchor, 0, 2*len(additionalRates))
	comparisonIncomplete := !additionalValid
	names := make([]string, 0, len(additionalRates))
	for name := range additionalRates {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, rawName := range names {
		observation := normalizeWebsocketQuotaRate(additionalRates[rawName], observedAt)
		for _, anchor := range observation.comparisonWindows {
			comparisonWindows = append(comparisonWindows, anchor)
		}
		comparisonIncomplete = comparisonIncomplete || observation.comparisonIncomplete
		name := strings.TrimSpace(rawName)
		if name == "" {
			continue
		}
		for _, window := range observation.windows {
			window.SourceName = name
			additional = append(additional, window)
		}
	}

	sourceID := normalizeQuotaSourceID(cleanString(event["metered_limit_name"]))
	if sourceID == "" {
		sourceID = normalizeQuotaSourceID(cleanString(event["limit_name"]))
	}
	if sourceID == codexAccountActiveLimit {
		sourceID = codexAccountQuotaSourceID
	}

	primary := normalizeWebsocketQuotaRate(event["rate_limits"], observedAt)
	result := make([]quotaWindow, 0, len(primary.windows)+len(additional))
	for _, window := range primary.windows {
		if sourceID == "" {
			if !websocketQuotaTopLevelIsAccount(primary.comparisonWindows[window.ID], comparisonWindows, comparisonIncomplete) {
				continue
			}
			window.SourceID = codexAccountQuotaSourceID
		} else {
			window.SourceID = sourceID
		}
		result = append(result, window)
	}
	// Named sources still resolve SourceID from existing snapshots; top-level copies were deduplicated within this event.
	return append(result, additional...)
}

func decodeWebsocketQuotaEvent(payload []byte) (map[string]any, bool) {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	var event map[string]any
	if decoder.Decode(&event) != nil || event == nil {
		return nil, false
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		return nil, false
	}
	return event, true
}

// websocketQuotaTopLevelIsAccount excludes top-level copies using additional windows from the same event.
// Slots and usage do not establish identity; same-period windows must exclude copies against a shared time basis.
func websocketQuotaTopLevelIsAccount(window websocketQuotaWindowAnchor, additional []websocketQuotaWindowAnchor, comparisonIncomplete bool) bool {
	// If an additional window has an unreadable period, the top-level window cannot be proven to belong to this source.
	if comparisonIncomplete {
		return false
	}
	for _, candidate := range additional {
		if window.windowSeconds != candidate.windowSeconds {
			continue
		}
		compared := false
		for _, pair := range [][2]int64{
			{window.resetAtSeconds, candidate.resetAtSeconds},
			{window.resetAfterSeconds, candidate.resetAfterSeconds},
		} {
			if pair[0] == 0 || pair[1] == 0 {
				continue
			}
			compared = true
			delta := pair[0] - pair[1]
			// Skip if either shared basis identifies a copy; contradictory decisions from the two bases cannot write to the account either.
			if delta >= -1 && delta <= 1 {
				return false
			}
		}
		if !compared {
			return false
		}
	}
	return true
}

// normalizeWebsocketQuotaRate separately retains window anchors needed for source comparison and quota values that can be written.
// Corrupt usage for one window neither erases its period identity nor blocks other valid windows from the same event.
func normalizeWebsocketQuotaRate(raw any, observedAt time.Time) websocketQuotaRateObservation {
	var result websocketQuotaRateObservation
	if raw == nil {
		return result
	}
	rate, valid := object(raw)
	if !valid {
		result.comparisonIncomplete = true
		return result
	}
	allowed, hasAllowed := rate["allowed"].(bool)
	limitReached, hasLimitReached := rate["limit_reached"].(bool)
	result.comparisonWindows = make(map[string]websocketQuotaWindowAnchor, 2)
	for _, slot := range []string{"primary", "secondary"} {
		rawWindow, exists := rate[slot]
		if !exists || rawWindow == nil {
			continue
		}
		window, valid := object(rawWindow)
		if !valid {
			result.comparisonIncomplete = true
			continue
		}
		minutes, ok := integer(window["window_minutes"])
		if !ok || minutes <= 0 || minutes > passiveQuotaMaxResetAtSeconds/60 {
			result.comparisonIncomplete = true
			continue
		}
		seconds := minutes * 60
		anchor := websocketQuotaWindowAnchor{windowSeconds: seconds}
		if absolute, ok := integer(window["reset_at"]); ok && absolute > 0 && absolute <= passiveQuotaMaxResetAtSeconds {
			anchor.resetAtSeconds = absolute
		}
		if relative, ok := integer(window["reset_after_seconds"]); ok && relative > 0 && relative <= passiveQuotaMaxResetAtSeconds {
			anchor.resetAfterSeconds = relative
		}
		result.comparisonWindows[slot] = anchor
		resetAtMS := websocketQuotaResetAtMS(anchor, observedAt)

		used, ok := number(window["used_percent"])
		if !ok || math.IsNaN(used) || math.IsInf(used, 0) || used < 0 || used > 100 {
			continue
		}
		limit, remaining, utilization := 100.0, 100-used, used/100
		state := "available"
		if used >= 100 || (hasAllowed && !allowed) || (hasLimitReached && limitReached) {
			state = "exhausted"
		}
		result.windows = append(result.windows, quotaWindow{
			ID:            slot,
			Used:          &used,
			Limit:         &limit,
			Remaining:     &remaining,
			Utilization:   &utilization,
			ResetAtMS:     resetAtMS,
			WindowSeconds: &seconds,
			State:         state,
		})
	}
	return result
}

func websocketQuotaResetAtMS(anchor websocketQuotaWindowAnchor, observedAt time.Time) *int64 {
	if anchor.resetAtSeconds > 0 {
		value := anchor.resetAtSeconds * 1000
		return &value
	}
	relative := anchor.resetAfterSeconds
	if relative == 0 {
		return nil
	}
	base := observedAt.Unix()
	if base < 0 || relative > passiveQuotaMaxResetAtSeconds-base {
		return nil
	}
	value := (base + relative) * 1000
	return &value
}
