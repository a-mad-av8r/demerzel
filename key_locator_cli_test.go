package main

import (
	"bytes"
	"os"
	"runtime"
	"strings"
	"testing"

	"gpt-load/internal/platform/config"
	"gpt-load/internal/platform/encryption"
)

func TestKeyLocatorUsesServerDefaultDataRootWithoutOpeningCustody(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("native data-root contract targets macOS and Linux")
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DATA_DIR", "")
	t.Setenv("XDG_DATA_HOME", "")
	dataDir, err := config.DefaultDataDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	want, err := encryption.InstallationIdentifier(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		var stdout, stderr bytes.Buffer
		if code := dispatchKeyLocator(nil, &stdout, &stderr); code != 0 ||
			strings.TrimSpace(stdout.String()) != want || stderr.Len() != 0 {
			t.Fatalf("offline locator did not resolve the server root: code=%d, stderr=%s", code, stderr.String())
		}
		t.Chdir(t.TempDir())
	}
	entries, err := os.ReadDir(dataDir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("locator created custody or application state: %v, entries=%v", err, entries)
	}
}
