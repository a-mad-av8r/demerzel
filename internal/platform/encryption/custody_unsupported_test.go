//go:build !darwin && !linux

package encryption

import (
	"strings"
	"testing"
)

func TestUnsupportedNativeCustodyNamesTheOnlyAvailableOverride(t *testing.T) {
	_, err := NewServiceWithCustody("", t.TempDir(), true)
	if err == nil || !strings.Contains(err.Error(), "set ENCRYPTION_KEY explicitly") {
		t.Fatalf("unsupported platform error = %v, want explicit recovery guidance", err)
	}
}
