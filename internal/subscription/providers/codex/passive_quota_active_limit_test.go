package codex

import (
	"encoding/json"
	"testing"
	"time"
)

// The upstream reports only quota billed to this request in the generic X-Codex-Primary/Secondary-* group.
// For Spark requests, the group contains Spark's window; treating it as ordinary account quota would overwrite ordinary 7d.
func TestPassiveQuotaGenericWindowsFollowActiveLimit(t *testing.T) {
	weekly := map[string]string{
		"X-Codex-Secondary-Used-Percent":   "0",
		"X-Codex-Secondary-Window-Minutes": "10080",
		"X-Codex-Secondary-Reset-At":       "1789222487",
	}
	sparkNamespace := map[string]string{
		"X-Codex-Bengalfox-Primary-Used-Percent":   "20",
		"X-Codex-Bengalfox-Primary-Window-Minutes": "300",
	}
	for _, test := range []struct {
		name        string
		activeLimit string
		namespaced  bool
		wantSources []string
		wantPeriods []int64
	}{
		{"spark request reports only the metered window", "codex_bengalfox", false,
			[]string{"codex_bengalfox"}, []int64{604800}},
		{"account request keeps the account source", "premium", false,
			[]string{"codex"}, []int64{604800}},
		{"account alias keeps the account source", "codex", false,
			[]string{"codex"}, []int64{604800}},
		// Same source but different periods: each window represents distinct quota and both must refresh.
		{"metered copy covers another period", "codex_bengalfox", true,
			[]string{"codex_bengalfox", "codex_bengalfox"}, []int64{604800, 18000}},
		{"legacy response without an active limit", "", false,
			[]string{"codex"}, []int64{604800}},
		{"unidentified copy alongside a metered namespace", "", true,
			[]string{"codex_bengalfox"}, []int64{18000}},
	} {
		t.Run(test.name, func(t *testing.T) {
			signals := map[string]string{}
			for key, value := range weekly {
				signals[key] = value
			}
			if test.activeLimit != "" {
				signals["X-Codex-Active-Limit"] = test.activeLimit
			}
			if test.namespaced {
				for key, value := range sparkNamespace {
					signals[key] = value
				}
			}

			windows := NormalizePassiveQuotaWindows(signals, time.Now())
			if len(windows) != len(test.wantSources) {
				t.Fatalf("windows = %#v, want %d from %v", windows, len(test.wantSources), test.wantSources)
			}
			for index, want := range test.wantSources {
				window := windows[index]
				if window.SourceID != want {
					t.Fatalf("window %d source = %q, want %q (all=%#v)", index, window.SourceID, want, windows)
				}
				if window.WindowSeconds == nil || *window.WindowSeconds != test.wantPeriods[index] {
					t.Fatalf("window %d period = %v, want %d (all=%#v)",
						index, window.WindowSeconds, test.wantPeriods[index], windows)
				}
			}
		})
	}
}

// Only the same source and period is a real duplicate: then the generic copy must yield, otherwise two data points target one
// window and the merge layer finds ambiguity, dropping both.
func TestPassiveQuotaMeteredCopyDoesNotCompeteWithItsNamespace(t *testing.T) {
	windows := NormalizePassiveQuotaWindows(map[string]string{
		"X-Codex-Active-Limit":                       "codex_bengalfox",
		"X-Codex-Primary-Used-Percent":               "20",
		"X-Codex-Primary-Window-Minutes":             "300",
		"X-Codex-Bengalfox-Primary-Used-Percent":     "20",
		"X-Codex-Bengalfox-Primary-Window-Minutes":   "300",
		"X-Codex-Bengalfox-Secondary-Used-Percent":   "40",
		"X-Codex-Bengalfox-Secondary-Window-Minutes": "10080",
	}, time.Now())
	if len(windows) != 2 {
		t.Fatalf("windows = %#v, want only the namespaced pair", windows)
	}
	for _, window := range windows {
		if window.SourceID != "codex_bengalfox" || window.WindowSeconds == nil {
			t.Fatalf("unexpected window: %#v (all=%#v)", window, windows)
		}
	}
	if *windows[0].WindowSeconds != 18000 || *windows[1].WindowSeconds != 604800 {
		t.Fatalf("duplicate or missing period: %#v", windows)
	}
}

// Active-Limit's value is the same source identifier as active-query metered_feature,
// so rebinding the generic group does not derive a response-header namespace prefix.
func TestPassiveQuotaActiveLimitMatchesActiveMeteredFeature(t *testing.T) {
	raw, err := NormalizeQuota([]byte(`{
		"rate_limit":{"primary_window":{"used_percent":100,"limit_window_seconds":604800,"reset_at":1788700000}},
		"additional_rate_limits":[{"metered_feature":"codex_bengalfox","limit_name":"GPT-5.3-Codex-Spark",
			"rate_limit":{"secondary_window":{"used_percent":0,"limit_window_seconds":604800,"reset_at":1789222487}}}]
	}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	var active quotaSnapshot
	if err := json.Unmarshal(raw, &active); err != nil {
		t.Fatal(err)
	}
	windows := NormalizePassiveQuotaWindows(map[string]string{
		"X-Codex-Active-Limit":             "codex_bengalfox",
		"X-Codex-Secondary-Used-Percent":   "0",
		"X-Codex-Secondary-Window-Minutes": "10080",
		"X-Codex-Secondary-Reset-At":       "1789222487",
	}, time.Now())
	if len(windows) != 1 {
		t.Fatalf("windows = %#v, want the rebound Spark window", windows)
	}
	spark := active.QuotaWindows[1]
	if windows[0].SourceID != spark.SourceID {
		t.Fatalf("passive source = %q, want the active metered feature %q", windows[0].SourceID, spark.SourceID)
	}
	if windows[0].WindowSeconds == nil || spark.WindowSeconds == nil ||
		*windows[0].WindowSeconds != *spark.WindowSeconds {
		t.Fatalf("passive period does not align with the active window: %#v", windows[0])
	}
}

// Deduplicate only repeated reports between generic and independent namespaces. The generic group's two slots remain
// distinct data even for the same period; neither may cause the other to disappear as a copy. The merge layer decides usability.
func TestPassiveQuotaDeduplicationIgnoresSiblingGenericWindows(t *testing.T) {
	windows := NormalizePassiveQuotaWindows(map[string]string{
		"X-Codex-Active-Limit":             "premium",
		"X-Codex-Primary-Used-Percent":     "10",
		"X-Codex-Primary-Window-Minutes":   "300",
		"X-Codex-Secondary-Used-Percent":   "20",
		"X-Codex-Secondary-Window-Minutes": "300",
	}, time.Now())
	if len(windows) != 2 {
		t.Fatalf("windows = %#v, want both generic slots preserved", windows)
	}
	for index, window := range windows {
		if window.SourceID != "codex" || window.WindowSeconds == nil || *window.WindowSeconds != 18000 {
			t.Fatalf("window %d = %#v", index, window)
		}
	}
	if windows[0].Used == nil || *windows[0].Used != 10 ||
		windows[1].Used == nil || *windows[1].Used != 20 {
		t.Fatalf("generic slots lost their own values: %#v", windows)
	}
}
