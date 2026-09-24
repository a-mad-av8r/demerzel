package webui

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestReleaseManifestSignsEveryDistributedArtifact(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("release manifest is generated on Linux")
	}
	for _, program := range []string{"bash", "jq", "sha256sum"} {
		if _, err := exec.LookPath(program); err != nil {
			t.Skipf("release generator requires %s", program)
		}
	}
	assets := strings.Fields(readRepositoryFile(t, ".github/release-assets.txt"))
	dir := t.TempDir()
	want := make(map[string]string, len(assets)-3)
	for _, name := range assets {
		switch name {
		case "manifest.json", "manifest.sigstore.json", "SHA256SUMS":
			continue
		}
		contents := []byte("synthetic artifact: " + name)
		if err := os.WriteFile(filepath.Join(dir, name), contents, 0o600); err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(contents)
		want[name] = hex.EncodeToString(digest[:])
	}
	command := exec.Command("bash", ".github/scripts/release-create-manifest.sh", dir, "a-mad-av8r/demerzel", "v2.0.1")
	command.Dir = filepath.Join("..", "..")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generate release manifest: %v: %s", err, output)
	}
	contents, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Assets []struct {
			Name   string `json:"name"`
			SHA256 string `json:"sha256"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(contents, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Assets) != len(want) {
		t.Fatalf("signed assets = %d, release assets = %d", len(manifest.Assets), len(want))
	}
	for _, asset := range manifest.Assets {
		if digest, exists := want[asset.Name]; !exists || asset.SHA256 != digest {
			t.Fatalf("asset %q is not bound to its distributed bytes", asset.Name)
		}
		delete(want, asset.Name)
	}
	if len(want) != 0 {
		t.Fatalf("release artifacts absent from signed manifest: %v", want)
	}
}
