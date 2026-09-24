package webui

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestReleaseImageRevisionVerifierRejectsHostileRegistryLabels(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX release verifier")
	}
	if _, err := exec.LookPath("jq"); err != nil {
		t.Skip("revision verifier requires jq")
	}
	const expected = "0123456789abcdef0123456789abcdef01234567"
	binDir := t.TempDir()
	fakeDocker := "#!/bin/sh\nprintf '%s' \"$REGISTRY_INSPECTION\"\n"
	if err := os.WriteFile(filepath.Join(binDir, "docker"), []byte(fakeDocker), 0o700); err != nil {
		t.Fatal(err)
	}
	inspection := func(amd64, arm64 any) string {
		value := map[string]any{
			"image": map[string]any{
				"linux/amd64": map[string]any{"config": map[string]any{
					"Labels": map[string]any{"org.opencontainers.image.revision": amd64},
				}},
				"linux/arm64": map[string]any{"config": map[string]any{
					"Labels": map[string]any{"org.opencontainers.image.revision": arm64},
				}},
			},
		}
		contents, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return string(contents)
	}
	for _, test := range []struct {
		name       string
		inspection string
		accepted   bool
	}{
		{"matching", inspection(expected, expected), true},
		{"newline", inspection(expected+"\ninjected", expected), false},
		{"shell metacharacter", inspection(expected+"|injected", expected), false},
		{"embedded NUL", inspection(expected+"\x00injected", expected), false},
		{"other architecture", inspection("different", expected), false},
		{"missing fields", `{}`, false},
		{"non-string", inspection(123, expected), false},
		{"malformed", `{`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			command := exec.Command("bash", filepath.Join("..", "..", ".github", "scripts", "release-verify-image-revision.sh"),
				"ghcr.io/a-mad-av8r/demerzel:2.0.1", expected)
			command.Env = []string{"PATH=" + binDir + string(os.PathListSeparator) + os.Getenv("PATH"),
				"REGISTRY_INSPECTION=" + test.inspection}
			output, err := command.Output()
			if test.accepted {
				if err != nil || string(output) != expected {
					t.Fatalf("matching revision was rejected: %v, stdout=%q", err, output)
				}
				return
			}
			if err == nil || len(strings.TrimSpace(string(output))) != 0 {
				t.Fatalf("untrusted registry label reached success output: %q, error=%v", output, err)
			}
		})
	}
}
