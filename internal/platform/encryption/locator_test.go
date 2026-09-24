package encryption

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallationIdentifierUsesCanonicalDataDirectory(t *testing.T) {
	dataDir := t.TempDir()
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(dataDir, alias); err != nil {
		t.Skipf("create canonical-path alias: %v", err)
	}
	canonical, err := InstallationIdentifier(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	throughAlias, err := InstallationIdentifier(alias)
	if err != nil || throughAlias != canonical {
		t.Fatalf("vault account differs for the same directory: %v", err)
	}
	other, err := InstallationIdentifier(t.TempDir())
	if err != nil || other == canonical {
		t.Fatalf("different installation reused vault account: %v", err)
	}
	if _, err := InstallationIdentifier(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("nonexistent DATA_DIR received a vault account locator")
	}
}
