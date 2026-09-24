package webui

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInstallerCannotUninstallDataOrIdentityInsideBinaryPrefix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX installer")
	}
	for _, nested := range []string{"data", "identity"} {
		t.Run(nested, func(t *testing.T) {
			root := t.TempDir()
			prefix := filepath.Join(root, "binary-prefix")
			dataDir := filepath.Join(root, "state", "demerzel")
			configDir := filepath.Join(root, "config", "demerzel")
			if nested == "data" {
				dataDir = filepath.Join(prefix, "demerzel")
			} else {
				configDir = filepath.Join(prefix, "config")
			}
			for _, dir := range []string{prefix, dataDir, configDir} {
				if err := os.MkdirAll(dir, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			state := filepath.Join(dataDir, "gpt-load.db")
			identity := filepath.Join(configDir, "identity.txt")
			for _, path := range []string{state, identity} {
				if err := os.WriteFile(path, []byte("synthetic irreplaceable state"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			command := exec.Command("bash", filepath.Join("..", "..", "packaging", "install.sh"),
				"uninstall", "--prefix", prefix, "--data-dir", dataDir, "--config-dir", configDir)
			command.Env = []string{"HOME=" + root, "PATH=" + os.Getenv("PATH")}
			if output, err := command.CombinedOutput(); err == nil {
				t.Fatalf("uninstall accepted runtime %s inside binary prefix: %s", nested, output)
			}
			for _, path := range []string{state, identity} {
				if _, err := os.Stat(path); err != nil {
					t.Fatalf("uninstall deleted custodied state %s: %v", path, err)
				}
			}
		})
	}
}

func TestInstallerRefusesToUninstallAnUnownedPrefix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX installer")
	}
	home := t.TempDir()
	prefix := filepath.Join(home, ".local", "share")
	if err := os.MkdirAll(prefix, 0o700); err != nil {
		t.Fatal(err)
	}
	otherApp := filepath.Join(prefix, "other-application-data")
	if err := os.WriteFile(otherApp, []byte("preserve"), 0o600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("bash", filepath.Join("..", "..", "packaging", "install.sh"),
		"uninstall", "--prefix", prefix)
	command.Env = []string{"HOME=" + home, "PATH=" + os.Getenv("PATH")}
	if output, err := command.CombinedOutput(); err == nil {
		t.Fatalf("uninstall accepted an unrelated prefix: %s", output)
	}
	if contents, err := os.ReadFile(otherApp); err != nil || string(contents) != "preserve" {
		t.Fatalf("uninstall deleted unrelated app data: %v", err)
	}
}

func TestTrustedInstallerNeverExecutesUnverifiedArtifactVerifier(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX installer")
	}
	root := t.TempDir()
	trustedDir := filepath.Join(root, "trusted")
	artifactDir := filepath.Join(root, "artifacts")
	for _, dir := range []string{trustedDir, artifactDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"install.sh", "verify-release.sh"} {
		if err := os.WriteFile(filepath.Join(trustedDir, name),
			[]byte(readRepositoryFile(t, filepath.Join("packaging", name))), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	marker := filepath.Join(root, "untrusted-verifier-executed")
	malicious := "#!/bin/sh\nprintf 'untrusted code ran' >" + marker + "\nprintf '2.0.1\\n'\n"
	for name, contents := range map[string]string{
		"install.sh":        "#!/bin/sh\nexit 0\n",
		"verify-release.sh": malicious,
	} {
		if err := os.WriteFile(filepath.Join(artifactDir, name), []byte(contents), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	command := exec.Command("bash", filepath.Join(trustedDir, "install.sh"), "install",
		"--artifacts", artifactDir, "--prefix", filepath.Join(root, "installed"))
	command.Env = []string{"HOME=" + root, "PATH=" + os.Getenv("PATH")}
	if output, err := command.CombinedOutput(); err == nil {
		t.Fatalf("unsigned artifact verifier was accepted: %s", strings.TrimSpace(string(output)))
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("untrusted artifact verifier ran before authentication: %v", err)
	}
}

func TestTrustedArtifactSmokeRejectsUnverifiedInstallersBeforeExecution(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX artifact smoke")
	}
	root := t.TempDir()
	trustedDir := filepath.Join(root, "trusted")
	oldDir := filepath.Join(root, "old")
	newDir := filepath.Join(root, "new")
	for _, dir := range []string{trustedDir, oldDir, newDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"local-smoke.sh", "verify-release.sh"} {
		if err := os.WriteFile(filepath.Join(trustedDir, name),
			[]byte(readRepositoryFile(t, filepath.Join("packaging", name))), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	marker := filepath.Join(root, "untrusted-code-executed")
	malicious := "#!/bin/sh\nprintf 'untrusted code ran' >\"" + marker + "\"\n"
	for _, dir := range []string{oldDir, newDir} {
		for _, name := range []string{"install.sh", "verify-release.sh"} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(malicious), 0o700); err != nil {
				t.Fatal(err)
			}
		}
	}
	command := exec.Command("bash", filepath.Join(trustedDir, "local-smoke.sh"), oldDir, newDir)
	if output, err := command.CombinedOutput(); err == nil {
		t.Fatalf("artifact smoke accepted unverified installers: %s", strings.TrimSpace(string(output)))
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("artifact smoke ran untrusted shell code before verification: %v", err)
	}
}

func TestInstallerPreservesPinnedCustomDataRootThroughUpgradeAndReinstall(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("POSIX installer")
	}
	if runtime.GOARCH != "arm64" && runtime.GOARCH != "amd64" {
		t.Skip("installer supports amd64 and arm64")
	}
	if _, err := exec.LookPath("age-keygen"); err != nil {
		t.Skip("age-keygen is required by the real installer")
	}
	root := t.TempDir()
	home := filepath.Join(root, "home")
	trustedDir := filepath.Join(root, "trusted")
	artifactDir := filepath.Join(root, "artifacts")
	for _, dir := range []string{home, trustedDir, artifactDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	installer := []byte(readRepositoryFile(t, "packaging/install.sh"))
	verifier := []byte("#!/bin/sh\nif [ \"${3-}\" = --print-version ]; then printf '%s\\n' \"$RELEASE_TEST_VERSION\"; fi\n")
	for path, contents := range map[string][]byte{
		filepath.Join(trustedDir, "install.sh"):         installer,
		filepath.Join(trustedDir, "verify-release.sh"):  verifier,
		filepath.Join(artifactDir, "install.sh"):        installer,
		filepath.Join(artifactDir, "verify-release.sh"): verifier,
	} {
		if err := os.WriteFile(path, contents, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	osName := runtime.GOOS
	if osName == "darwin" {
		osName = "macos"
	}
	asset := filepath.Join(artifactDir, "demerzel-"+osName+"-"+runtime.GOARCH)
	if err := os.WriteFile(asset, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	prefix := filepath.Join(home, ".local", "opt", "demerzel")
	dataDir := filepath.Join(root, "state", "demerzel")
	configDir := filepath.Join(home, ".config", "demerzel")
	run := func(script, version, command string, args ...string) {
		t.Helper()
		request := exec.Command("bash", append([]string{script, command}, args...)...)
		request.Env = []string{"HOME=" + home, "PATH=" + os.Getenv("PATH"), "RELEASE_TEST_VERSION=" + version}
		if output, err := request.CombinedOutput(); err != nil {
			t.Fatalf("%s %s failed: %v: %s", command, version, err, output)
		}
	}
	run(filepath.Join(trustedDir, "install.sh"), "2.0.1", "install",
		"--artifacts", artifactDir, "--prefix", prefix, "--data-dir", dataDir)
	original, err := filepath.EvalSymlinks(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	pin := filepath.Join(configDir, "data-dir")
	assertPinned := func() {
		t.Helper()
		recorded, err := os.ReadFile(pin)
		if err != nil || strings.TrimSpace(string(recorded)) != original {
			t.Fatalf("custom DATA_DIR pin was lost: %q, %v", recorded, err)
		}
		if _, err := os.Stat(filepath.Join(home, ".demerzel")); !os.IsNotExist(err) {
			t.Fatalf("installer silently created a second default data root: %v", err)
		}
	}
	assertPinned()
	state := filepath.Join(dataDir, "gpt-load.db")
	if err := os.WriteFile(state, []byte("encrypted-account-state"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, override := range [][]string{
		{"--data-dir", filepath.Join(root, "unrelated", "demerzel")},
		{"--config-dir", filepath.Join(root, "unrelated", "config")},
	} {
		script := filepath.Join(prefix, "install.sh")
		request := exec.Command("bash", script, "install",
			"--artifacts", artifactDir, "--prefix", prefix, override[0], override[1])
		request.Env = []string{"HOME=" + home, "PATH=" + os.Getenv("PATH"), "RELEASE_TEST_VERSION=2.0.2"}
		if output, err := request.CombinedOutput(); err == nil {
			t.Fatalf("upgrade changed pinned custody path %q: %s", override[0], output)
		}
		assertPinned()
	}
	run(filepath.Join(prefix, "install.sh"), "2.0.2", "install",
		"--artifacts", artifactDir, "--prefix", prefix)
	assertPinned()
	run(filepath.Join(prefix, "install.sh"), "2.0.2", "uninstall", "--prefix", prefix)
	assertPinned()
	if _, err := os.Stat(state); err != nil {
		t.Fatalf("uninstall deleted the original account database: %v", err)
	}
	run(filepath.Join(trustedDir, "install.sh"), "2.0.3", "install",
		"--artifacts", artifactDir, "--prefix", prefix)
	assertPinned()
	if _, err := os.Stat(state); err != nil {
		t.Fatalf("reinstall lost the original account database: %v", err)
	}
	if _, err := os.Stat(filepath.Join(configDir, "identity.txt")); err != nil {
		t.Fatalf("upgrade lost the external age identity: %v", err)
	}
}
