package accountscli

import (
	"strings"
	"testing"
)

func TestAccountHealthOnlyReportsObservedFailures(t *testing.T) {
	fresh := healthDisplay(credentialItem{AuthState: "ready", LastFailureCategory: "ambiguous"})
	if strings.Contains(fresh, "last_failure=") || !strings.Contains(fresh, "auth=ready") {
		t.Fatalf("new account was shown as failed: %q", fresh)
	}
	failed := healthDisplay(credentialItem{AuthState: "ready", LastFailureCategory: "rate_limited", RecentFailureCount: 1})
	if !strings.Contains(failed, "last_failure=rate_limited") {
		t.Fatalf("real recent failure was hidden: %q", failed)
	}
}
