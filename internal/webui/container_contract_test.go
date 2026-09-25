package webui

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func requireDockerCompose(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Docker Compose CLI is unavailable; CI exercises this contract")
	}
}

func writeComposeAgeFixture(t *testing.T, projectDir string) string {
	t.Helper()
	path := filepath.Join(projectDir, "age-identity.txt")
	if err := os.WriteFile(path, []byte("identity fixture"), 0o600); err != nil {
		t.Fatalf("write temporary age identity: %v", err)
	}
	return path
}

func composeAgeEnvironment(identityPath string) string {
	return "DEMERZEL_ENCRYPTION_KEY_AGE_IDENTITY_FILE=" + identityPath +
		"\nDEMERZEL_ENCRYPTION_KEY_AGE_RECIPIENT=age1fixture\n"
}

func TestComposeShellPortOverridesDotEnvEverywhere(t *testing.T) {
	requireDockerCompose(t)
	t.Setenv("HOST", "")
	t.Setenv("BIND_ADDRESS", "")
	t.Setenv("OAUTH_CALLBACK_BIND_ADDRESS", "")

	projectDir := t.TempDir()
	identityPath := writeComposeAgeFixture(t, projectDir)
	if err := os.WriteFile(
		filepath.Join(projectDir, "docker-compose.yml"),
		[]byte(readRepositoryFile(t, "docker-compose.yml")),
		0o600,
	); err != nil {
		t.Fatalf("write temporary Compose file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, ".env"), []byte("PORT=3001\n"+composeAgeEnvironment(identityPath)), 0o600); err != nil {
		t.Fatalf("write temporary .env: %v", err)
	}

	command := exec.Command(
		"docker", "compose", "config", "--no-env-resolution", "--format", "json",
	)
	command.Dir = projectDir
	command.Env = append(os.Environ(), "PORT=41234")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("docker compose config: %v\n%s", err, output)
	}

	var resolved struct {
		Services map[string]struct {
			Environment map[string]string `json:"environment"`
			Healthcheck struct {
				Test []string `json:"test"`
			} `json:"healthcheck"`
			Ports []struct {
				Target    int    `json:"target"`
				Published string `json:"published"`
				HostIP    string `json:"host_ip"`
			} `json:"ports"`
		} `json:"services"`
	}
	if err := json.Unmarshal(output, &resolved); err != nil {
		t.Fatalf("decode docker compose config: %v\n%s", err, output)
	}

	service := resolved.Services["demerzel"]
	if service.Environment["PORT"] != "41234" {
		t.Fatalf("resolved application PORT = %q, want shell override 41234", service.Environment["PORT"])
	}
	if service.Environment["HOST"] != "0.0.0.0" {
		t.Fatalf("resolved application HOST = %q, want 0.0.0.0", service.Environment["HOST"])
	}
	if len(service.Ports) != 4 ||
		service.Ports[0].Target != 41234 ||
		service.Ports[0].Published != "41234" ||
		service.Ports[0].HostIP != "127.0.0.1" ||
		service.Ports[1].Target != 1455 ||
		service.Ports[1].Published != "1455" ||
		service.Ports[1].HostIP != "127.0.0.1" ||
		service.Ports[2].Target != 54545 ||
		service.Ports[2].Published != "54545" ||
		service.Ports[2].HostIP != "127.0.0.1" ||
		service.Ports[3].Target != 51121 ||
		service.Ports[3].Published != "51121" ||
		service.Ports[3].HostIP != "127.0.0.1" {
		t.Fatalf("resolved ports = %#v, want application 41234 and loopback OAuth callbacks 1455/54545/51121", service.Ports)
	}
	if len(service.Healthcheck.Test) != 2 ||
		!strings.Contains(service.Healthcheck.Test[1], "localhost:41234/health") {
		t.Fatalf("resolved healthcheck = %#v, want container PORT 41234", service.Healthcheck.Test)
	}
}

func TestComposeHostBindingsInheritHostAndAllowIndependentOverrides(t *testing.T) {
	requireDockerCompose(t)
	projectDir := t.TempDir()
	identityPath := writeComposeAgeFixture(t, projectDir)
	if err := os.WriteFile(
		filepath.Join(projectDir, "docker-compose.yml"),
		[]byte(readRepositoryFile(t, "docker-compose.yml")),
		0o600,
	); err != nil {
		t.Fatalf("write temporary Compose file: %v", err)
	}

	commandEnvironment := make([]string, 0, len(os.Environ()))
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if key != "HOST" && key != "BIND_ADDRESS" && key != "OAUTH_CALLBACK_BIND_ADDRESS" &&
			!strings.HasPrefix(key, "DEMERZEL_ENCRYPTION_KEY_AGE_") && key != "DEMERZEL_VERSION" {
			commandEnvironment = append(commandEnvironment, entry)
		}
	}

	for _, testCase := range []struct {
		name             string
		bindAddress      string
		oauthBindAddress string
		wantMainHost     string
		wantOAuthHost    string
	}{
		{
			name:          "host_default",
			wantMainHost:  "192.0.2.10",
			wantOAuthHost: "192.0.2.10",
		},
		{
			name:          "main_service_override",
			bindAddress:   "127.0.0.2",
			wantMainHost:  "127.0.0.2",
			wantOAuthHost: "192.0.2.10",
		},
		{
			name:             "oauth_callback_override",
			oauthBindAddress: "127.0.0.3",
			wantMainHost:     "192.0.2.10",
			wantOAuthHost:    "127.0.0.3",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			environmentLines := []string{
				"HOST=192.0.2.10",
				"DEMERZEL_ENCRYPTION_KEY_AGE_IDENTITY_FILE=" + identityPath,
				"DEMERZEL_ENCRYPTION_KEY_AGE_RECIPIENT=age1fixture",
			}
			if testCase.bindAddress != "" {
				environmentLines = append(environmentLines, "BIND_ADDRESS="+testCase.bindAddress)
			}
			if testCase.oauthBindAddress != "" {
				environmentLines = append(
					environmentLines,
					"OAUTH_CALLBACK_BIND_ADDRESS="+testCase.oauthBindAddress,
				)
			}
			if err := os.WriteFile(
				filepath.Join(projectDir, ".env"),
				[]byte(strings.Join(environmentLines, "\n")+"\n"),
				0o600,
			); err != nil {
				t.Fatalf("write temporary .env: %v", err)
			}

			command := exec.Command(
				"docker", "compose", "config", "--no-env-resolution", "--format", "json",
			)
			command.Dir = projectDir
			command.Env = commandEnvironment
			output, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("docker compose config: %v\n%s", err, output)
			}

			var resolved struct {
				Services map[string]struct {
					Environment map[string]string `json:"environment"`
					Ports       []struct {
						HostIP string `json:"host_ip"`
					} `json:"ports"`
				} `json:"services"`
			}
			if err := json.Unmarshal(output, &resolved); err != nil {
				t.Fatalf("decode docker compose config: %v\n%s", err, output)
			}

			service := resolved.Services["demerzel"]
			if service.Environment["HOST"] != "0.0.0.0" {
				t.Fatalf("resolved application HOST = %q, want 0.0.0.0", service.Environment["HOST"])
			}
			if len(service.Ports) != 4 {
				t.Fatalf("resolved ports = %#v, want application and three OAuth callback ports", service.Ports)
			}
			if service.Ports[0].HostIP != testCase.wantMainHost {
				t.Fatalf("main service host IP = %q, want %q", service.Ports[0].HostIP, testCase.wantMainHost)
			}
			for index, port := range service.Ports[1:] {
				if port.HostIP != testCase.wantOAuthHost {
					t.Fatalf("OAuth callback %d host IP = %q, want %q", index, port.HostIP, testCase.wantOAuthHost)
				}
			}
		})
	}
}

func TestDockerfileFinalStageDeclaresNonRootPersistentRuntime(t *testing.T) {
	content := readRepositoryFile(t, "Dockerfile")
	// runtime is the only runtime definition; both source-contained builds and release prebuilds inherit it.
	// Every publishable stage must be based on it, or a packaging path would bypass all runtime assertions below.
	runtimeIndex := strings.Index(content, "\nFROM alpine:")
	if runtimeIndex < 0 {
		t.Fatal("Dockerfile does not contain the shared runtime stage")
	}
	if !strings.Contains(content[runtimeIndex:], "AS runtime\n") {
		t.Fatal("Dockerfile runtime stage is not named runtime")
	}
	nextStageIndex := strings.Index(content[runtimeIndex+1:], "\nFROM ")
	if nextStageIndex < 0 {
		t.Fatal("Dockerfile does not derive any stage from the shared runtime stage")
	}
	finalStage := content[runtimeIndex : runtimeIndex+1+nextStageIndex]
	for _, derived := range []string{"FROM runtime AS prebuilt", "FROM runtime AS source-build"} {
		if !strings.Contains(content, derived) {
			t.Fatalf("Dockerfile does not derive a publishable stage via %q", derived)
		}
	}
	// Apart from runtime itself, no publishable stage may be based directly on alpine.
	if strings.Count(content, "\nFROM alpine:") != 1 {
		t.Fatal("Dockerfile declares a publishable stage that bypasses the shared runtime stage")
	}
	if !strings.Contains(finalStage, "EXPOSE 3001 1455 54545 51121") {
		t.Fatal("Dockerfile runtime stage does not expose the application and all fixed OAuth callback ports")
	}

	orderedBeforeUser := []string{
		"addgroup -S -g 10001 demerzel",
		"adduser -S -D -H -u 10001 -G demerzel demerzel",
		"mkdir -p /app/data",
		"chown 10001:10001 /app/data",
		"chmod 0700 /app/data",
		"ENV HOST=0.0.0.0",
		"ENV DATA_DIR=/app/data",
		"USER 10001:10001",
	}
	previousIndex := -1
	for _, required := range orderedBeforeUser {
		index := strings.Index(finalStage, required)
		if index < 0 {
			t.Fatalf("Dockerfile final stage does not contain %q", required)
		}
		if index <= previousIndex {
			t.Fatalf("Dockerfile final stage places %q out of order", required)
		}
		previousIndex = index
	}

	linesByMutation := map[string][]string{
		"USER":  {},
		"chown": {},
		"chmod": {},
	}
	for _, line := range strings.Split(finalStage, "\n") {
		trimmed := strings.TrimSpace(line)
		fields := strings.Fields(trimmed)
		if len(fields) > 0 && strings.EqualFold(fields[0], "USER") {
			linesByMutation["USER"] = append(linesByMutation["USER"], trimmed)
		}

		command := strings.TrimSpace(strings.TrimSuffix(trimmed, `\`))
		command = strings.TrimSpace(strings.TrimPrefix(command, "RUN "))
		command = strings.TrimSpace(strings.TrimPrefix(command, "&& "))
		for _, mutation := range []string{"chown", "chmod"} {
			if strings.Contains(command, mutation+" ") {
				linesByMutation[mutation] = append(linesByMutation[mutation], command)
			}
		}
	}
	for _, expectation := range []struct {
		label string
		line  string
	}{
		{label: "USER", line: "USER 10001:10001"},
		{label: "chown", line: "chown 10001:10001 /app/data"},
		{label: "chmod", line: "chmod 0700 /app/data"},
	} {
		lines := linesByMutation[expectation.label]
		if len(lines) != 1 || lines[0] != expectation.line {
			t.Fatalf(
				"Dockerfile final stage %s mutations = %q, want exactly [%q]",
				expectation.label,
				lines,
				expectation.line,
			)
		}
	}

	entrypoint := `ENTRYPOINT ["/app/gpt-load"]`
	entrypointIndex := strings.Index(finalStage, entrypoint)
	if entrypointIndex < 0 {
		t.Fatalf("Dockerfile final stage does not contain %q", entrypoint)
	}
	if previousIndex >= entrypointIndex {
		t.Fatal("Dockerfile final stage does not switch users before the direct ENTRYPOINT")
	}
	if strings.Count(finalStage, "ENTRYPOINT") != 1 {
		t.Fatal("Dockerfile final stage must declare exactly one direct ENTRYPOINT")
	}

	afterUser := finalStage[strings.Index(finalStage, "USER 10001:10001")+len("USER 10001:10001"):]
	for _, forbidden := range []string{"USER root", "chown"} {
		if strings.Contains(afterUser, forbidden) {
			t.Fatalf("Dockerfile final stage contains %q after the non-root USER", forbidden)
		}
	}
}

func TestDockerfileCopiesLocalCPAEmbeddedModuleBeforeGoModuleDownload(t *testing.T) {
	content := readRepositoryFile(t, "Dockerfile")
	rootModules := strings.Index(content, "COPY go.mod go.sum ./")
	bridgeModules := strings.Index(content, "COPY third_party/cpaembedded/go.mod third_party/cpaembedded/go.sum ./third_party/cpaembedded/")
	download := strings.Index(content, "RUN go mod download")
	bridgeSource := strings.Index(content, "COPY third_party/cpaembedded ./third_party/cpaembedded")
	build := strings.Index(content, "GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build")
	if rootModules < 0 || bridgeModules < 0 || download < 0 || bridgeSource < 0 || build < 0 {
		t.Fatalf("Dockerfile is missing local CPA embedded module build inputs")
	}
	if rootModules >= bridgeModules || bridgeModules >= download || download >= bridgeSource || bridgeSource >= build {
		t.Fatalf("Dockerfile local CPA module copy order is invalid")
	}
}

func TestDockerfileSourceBuildCopiesAllRootGoSources(t *testing.T) {
	content := readRepositoryFile(t, "Dockerfile")
	if !strings.Contains(content, "COPY *.go ./") {
		t.Fatal("Dockerfile source build does not copy all root Go sources")
	}
	if strings.Contains(content, "COPY main.go ./") {
		t.Fatal("Dockerfile source build still copies only main.go")
	}
}

func TestDockerfileDistributesDeclaredThirdPartyLicenseTexts(t *testing.T) {
	content := readRepositoryFile(t, "Dockerfile")
	for _, required := range []string{
		"COPY LICENSES/Apache-2.0.txt /app/licenses/Apache-2.0.txt",
		"COPY LICENSES/Inno-Setup.txt /app/licenses/Inno-Setup.txt",
		"COPY LICENSES/MIT.txt /app/licenses/MIT.txt",
		"COPY LICENSES/MPL-2.0.txt /app/licenses/MPL-2.0.txt",
	} {
		if !strings.Contains(content, required) {
			t.Fatalf("Dockerfile does not distribute declared license text %q", required)
		}
	}
}

func TestComposeBindsLoopbackAndConfiguresContainerAllInterfaces(t *testing.T) {
	requireDockerCompose(t)
	t.Setenv("HOST", "")
	t.Setenv("BIND_ADDRESS", "")
	t.Setenv("OAUTH_CALLBACK_BIND_ADDRESS", "")
	t.Setenv("PORT", "")

	projectDir := t.TempDir()
	identityPath := writeComposeAgeFixture(t, projectDir)
	if err := os.WriteFile(
		filepath.Join(projectDir, "docker-compose.yml"),
		[]byte(readRepositoryFile(t, "docker-compose.yml")),
		0o600,
	); err != nil {
		t.Fatalf("write temporary Compose file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, ".env"), []byte(composeAgeEnvironment(identityPath)), 0o600); err != nil {
		t.Fatalf("write temporary .env: %v", err)
	}

	command := exec.Command(
		"docker", "compose", "config", "--no-env-resolution", "--format", "json",
	)
	command.Dir = projectDir
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("docker compose config: %v\n%s", err, output)
	}

	var resolved struct {
		Services map[string]struct {
			Environment map[string]string `json:"environment"`
			Ports       []struct {
				Target    int    `json:"target"`
				Published string `json:"published"`
				HostIP    string `json:"host_ip"`
			} `json:"ports"`
		} `json:"services"`
	}
	if err := json.Unmarshal(output, &resolved); err != nil {
		t.Fatalf("decode docker compose config: %v\n%s", err, output)
	}

	service := resolved.Services["demerzel"]
	if service.Environment["HOST"] != "0.0.0.0" {
		t.Fatalf("resolved application HOST = %q, want 0.0.0.0", service.Environment["HOST"])
	}
	if len(service.Ports) != 4 ||
		service.Ports[0].Target != 3001 ||
		service.Ports[0].Published != "3001" ||
		service.Ports[0].HostIP != "127.0.0.1" ||
		service.Ports[1].Target != 1455 ||
		service.Ports[1].Published != "1455" ||
		service.Ports[1].HostIP != "127.0.0.1" ||
		service.Ports[2].Target != 54545 ||
		service.Ports[2].Published != "54545" ||
		service.Ports[2].HostIP != "127.0.0.1" ||
		service.Ports[3].Target != 51121 ||
		service.Ports[3].Published != "51121" ||
		service.Ports[3].HostIP != "127.0.0.1" {
		t.Fatalf("resolved ports = %#v, want application 3001 and loopback OAuth callbacks 1455/54545/51121", service.Ports)
	}
}

func TestComposeProjectsHaveIndependentNamesApplicationPortsAndVolumes(t *testing.T) {
	requireDockerCompose(t)
	t.Setenv("HOST", "")
	t.Setenv("BIND_ADDRESS", "")
	t.Setenv("OAUTH_CALLBACK_BIND_ADDRESS", "")
	t.Setenv("PORT", "")

	projectDir := t.TempDir()
	identityPath := writeComposeAgeFixture(t, projectDir)
	if err := os.WriteFile(
		filepath.Join(projectDir, "docker-compose.yml"),
		[]byte(readRepositoryFile(t, "docker-compose.yml")),
		0o600,
	); err != nil {
		t.Fatalf("write temporary Compose file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, ".env"), []byte(composeAgeEnvironment(identityPath)), 0o600); err != nil {
		t.Fatalf("write temporary .env: %v", err)
	}

	type composeConfig struct {
		Name     string `json:"name"`
		Services map[string]struct {
			ContainerName string `json:"container_name"`
			Ports         []struct {
				Target    int    `json:"target"`
				Published string `json:"published"`
				HostIP    string `json:"host_ip"`
			} `json:"ports"`
		} `json:"services"`
		Volumes map[string]struct {
			Name string `json:"name"`
		} `json:"volumes"`
	}

	render := func(projectName, publishedPort string) composeConfig {
		t.Helper()
		command := exec.Command(
			"docker", "compose", "--project-name", projectName,
			"config", "--no-env-resolution", "--format", "json",
		)
		command.Dir = projectDir
		command.Env = append(os.Environ(), "PORT="+publishedPort)
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("docker compose config for %s: %v\n%s", projectName, err, output)
		}

		var resolved composeConfig
		if err := json.Unmarshal(output, &resolved); err != nil {
			t.Fatalf("decode docker compose config for %s: %v\n%s", projectName, err, output)
		}
		return resolved
	}

	first := render("review-one", "41001")
	second := render("review-two", "41002")
	for _, item := range []struct {
		projectName   string
		publishedPort string
		targetPort    int
		config        composeConfig
	}{
		{projectName: "review-one", publishedPort: "41001", targetPort: 41001, config: first},
		{projectName: "review-two", publishedPort: "41002", targetPort: 41002, config: second},
	} {
		if item.config.Name != item.projectName {
			t.Fatalf("resolved project name = %q, want %q", item.config.Name, item.projectName)
		}
		service := item.config.Services["demerzel"]
		if service.ContainerName != "" {
			t.Fatalf("resolved project %s fixes container_name to %q", item.projectName, service.ContainerName)
		}
		if len(service.Ports) != 4 ||
			service.Ports[0].Target != item.targetPort ||
			service.Ports[0].Published != item.publishedPort ||
			service.Ports[0].HostIP != "127.0.0.1" ||
			service.Ports[1].Target != 1455 ||
			service.Ports[1].Published != "1455" ||
			service.Ports[1].HostIP != "127.0.0.1" ||
			service.Ports[2].Target != 54545 ||
			service.Ports[2].Published != "54545" ||
			service.Ports[2].HostIP != "127.0.0.1" ||
			service.Ports[3].Target != 51121 ||
			service.Ports[3].Published != "51121" ||
			service.Ports[3].HostIP != "127.0.0.1" {
			t.Fatalf("resolved project %s ports = %#v", item.projectName, service.Ports)
		}
		wantVolume := item.projectName + "_demerzel-data"
		if got := item.config.Volumes["demerzel-data"].Name; got != wantVolume {
			t.Fatalf("resolved project %s volume = %q, want %q", item.projectName, got, wantVolume)
		}
	}
	if first.Volumes["demerzel-data"].Name == second.Volumes["demerzel-data"].Name {
		t.Fatal("different Compose projects resolve the same named volume")
	}
}

func TestComposeUsesOwnedImageNamedDataVolumeAndExternalAgeIdentity(t *testing.T) {
	requireDockerCompose(t)
	t.Setenv("DATA_DIR", "/host/path/must-not-reach-container")
	t.Setenv("DATABASE_DSN", "/host/database/must-not-reach-container.db")
	t.Setenv("DEMERZEL_VERSION", "")

	projectDir := t.TempDir()
	identityPath := writeComposeAgeFixture(t, projectDir)
	if err := os.WriteFile(
		filepath.Join(projectDir, "docker-compose.yml"),
		[]byte(readRepositoryFile(t, "docker-compose.yml")),
		0o600,
	); err != nil {
		t.Fatalf("write temporary Compose file: %v", err)
	}
	envFile := composeAgeEnvironment(identityPath)
	if err := os.WriteFile(filepath.Join(projectDir, ".env"), []byte(envFile), 0o600); err != nil {
		t.Fatalf("write temporary .env: %v", err)
	}

	command := exec.Command(
		"docker", "compose", "config", "--no-env-resolution", "--format", "json",
	)
	command.Dir = projectDir
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("docker compose config: %v\n%s", err, output)
	}

	var resolved struct {
		Services map[string]struct {
			Image           string            `json:"image"`
			Environment     map[string]string `json:"environment"`
			User            string            `json:"user"`
			UsernsMode      string            `json:"userns_mode"`
			Privileged      bool              `json:"privileged"`
			StopGracePeriod string            `json:"stop_grace_period"`
			Healthcheck     map[string]any    `json:"healthcheck"`
			Volumes         []struct {
				Type     string `json:"type"`
				Source   string `json:"source"`
				Target   string `json:"target"`
				ReadOnly bool   `json:"read_only"`
			} `json:"volumes"`
		} `json:"services"`
		Volumes map[string]any `json:"volumes"`
	}
	if err := json.Unmarshal(output, &resolved); err != nil {
		t.Fatalf("decode docker compose config: %v\n%s", err, output)
	}

	service, ok := resolved.Services["demerzel"]
	if !ok {
		t.Fatal("resolved Compose lacks Demerzel service")
	}
	if service.Image != "demerzel:local" {
		t.Fatalf("resolved image = %q, want Demerzel-owned local image", service.Image)
	}
	if service.Environment["DATA_DIR"] != "/app/data" {
		t.Fatalf("resolved DATA_DIR = %q, want /app/data", service.Environment["DATA_DIR"])
	}
	if service.Environment["DEMERZEL_ENCRYPTION_KEY_AGE_IDENTITY_FILE"] != "/run/secrets/demerzel-age-identity" ||
		service.Environment["DEMERZEL_ENCRYPTION_KEY_AGE_RECIPIENT"] != "age1fixture" {
		t.Fatalf("resolved age custody configuration = %#v", service.Environment)
	}
	if _, ok := service.Environment["DATABASE_DSN"]; ok {
		t.Fatal("host DATABASE_DSN leaked into the container")
	}
	if service.User != "10001:10001" || service.UsernsMode != "keep-id:uid=10001,gid=10001" {
		t.Fatalf("rootless runtime user mapping = %q/%q", service.User, service.UsernsMode)
	}
	if service.Privileged {
		t.Fatal("resolved Compose enables privileged mode")
	}
	if service.StopGracePeriod != "15s" || len(service.Healthcheck) == 0 {
		t.Fatalf("resolved stop or health contract = %q/%#v", service.StopGracePeriod, service.Healthcheck)
	}
	if len(service.Volumes) != 2 {
		t.Fatalf("resolved volume count = %d, want data volume and external identity mount", len(service.Volumes))
	}
	dataVolume, identityMount := false, false
	for _, volume := range service.Volumes {
		if volume.Type == "volume" && volume.Source == "demerzel-data" &&
			volume.Target == "/app/data" {
			dataVolume = true
		}
		if volume.Type == "bind" && volume.Source == identityPath &&
			volume.Target == "/run/secrets/demerzel-age-identity" && volume.ReadOnly {
			identityMount = true
		}
		if strings.Contains(volume.Source, "docker.sock") || strings.Contains(volume.Target, "docker.sock") {
			t.Fatal("resolved Compose mounts the container engine socket")
		}
	}
	if !dataVolume || !identityMount {
		t.Fatalf("resolved mounts do not separate persistent data from the read-only external age identity: %#v", service.Volumes)
	}
	if _, ok := resolved.Volumes["demerzel-data"]; !ok {
		t.Fatal("resolved Compose lacks the named Demerzel data volume")
	}
}

func TestInstallerUninstallPreservesDataUnlessExactPurgeConfirmation(t *testing.T) {
	installer := filepath.Join(t.TempDir(), "install.sh")
	if err := os.WriteFile(installer, []byte(readRepositoryFile(t, "packaging/install.sh")), 0o700); err != nil {
		t.Fatalf("write temporary installer: %v", err)
	}

	home := t.TempDir()
	prefix := filepath.Join(home, ".local", "opt", "demerzel")
	dataDir := filepath.Join(home, ".demerzel")
	identity := filepath.Join(home, ".config", "demerzel", "identity.txt")
	if err := os.MkdirAll(prefix, 0o700); err != nil {
		t.Fatalf("create install prefix: %v", err)
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatalf("create data directory: %v", err)
	}
	canonicalDataDir, err := filepath.EvalSymlinks(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(identity), 0o700); err != nil {
		t.Fatalf("create external identity directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "gpt-load.db"), []byte("state"), 0o600); err != nil {
		t.Fatalf("write database sentinel: %v", err)
	}
	if err := os.WriteFile(identity, []byte("external identity"), 0o600); err != nil {
		t.Fatalf("write identity sentinel: %v", err)
	}

	configDir, err := filepath.EvalSymlinks(filepath.Dir(identity))
	if err != nil {
		t.Fatal(err)
	}
	seedInstalledPrefix := func() {
		t.Helper()
		versionDir := filepath.Join(prefix, "versions", "2.0.1", "bin")
		binDir := filepath.Join(prefix, "bin")
		for _, dir := range []string{versionDir, binDir} {
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
		}
		for _, path := range []string{
			filepath.Join(prefix, "install.sh"),
			filepath.Join(binDir, "demerzel"),
			filepath.Join(versionDir, "gpt-load"),
		} {
			if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(prefix, "config-dir"), []byte(configDir+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(configDir, "data-dir"), []byte(canonicalDataDir+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("versions/2.0.1", filepath.Join(prefix, "current")); err != nil {
			t.Fatal(err)
		}
	}
	seedInstalledPrefix()

	run := func(input string, args ...string) error {
		t.Helper()
		command := exec.Command("bash", append([]string{installer, "uninstall"}, args...)...)
		command.Env = []string{"HOME=" + home, "PATH=" + os.Getenv("PATH")}
		command.Stdin = strings.NewReader(input)
		output, err := command.CombinedOutput()
		if err != nil && input != "" {
			t.Logf("uninstall rejected confirmation as expected: %s", output)
		}
		return err
	}

	if err := run("", "--prefix", prefix, "--data-dir", dataDir); err != nil {
		t.Fatalf("default uninstall failed: %v", err)
	}
	if _, err := os.Stat(prefix); !os.IsNotExist(err) {
		t.Fatalf("default uninstall kept the binary prefix: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "gpt-load.db")); err != nil {
		t.Fatalf("default uninstall removed runtime data: %v", err)
	}
	if _, err := os.Stat(identity); err != nil {
		t.Fatalf("default uninstall removed the external identity: %v", err)
	}

	seedInstalledPrefix()
	if err := run("PURGE /wrong/path\n", "--prefix", prefix, "--data-dir", dataDir, "--purge"); err == nil {
		t.Fatal("purge accepted a confirmation for another path")
	}
	if _, err := os.Stat(filepath.Join(dataDir, "gpt-load.db")); err != nil {
		t.Fatalf("rejected purge removed runtime data: %v", err)
	}
	if err := run("PURGE "+canonicalDataDir+"\n", "--prefix", prefix, "--data-dir", dataDir, "--purge"); err != nil {
		t.Fatalf("exactly confirmed purge failed: %v", err)
	}
	if _, err := os.Stat(dataDir); !os.IsNotExist(err) {
		t.Fatalf("explicit purge kept the data directory: %v", err)
	}
	if _, err := os.Stat(identity); err != nil {
		t.Fatalf("explicit data purge removed the external age identity: %v", err)
	}
}

func TestPodmanVolumePurgeRequiresOwnedVolumeAndExactConfirmation(t *testing.T) {
	if _, err := exec.LookPath("jq"); err != nil {
		t.Skip("jq is required by the Podman volume ownership gate")
	}
	scratch := t.TempDir()
	fakeBin := filepath.Join(scratch, "bin")
	if err := os.MkdirAll(fakeBin, 0o700); err != nil {
		t.Fatalf("create fake command directory: %v", err)
	}
	calls := filepath.Join(scratch, "podman-calls")
	fakePodman := `#!/usr/bin/env sh
set -eu
case "$1:$2" in
  volume:inspect) printf '%s\n' "$LABEL_JSON" ;;
  volume:rm) printf '%s\n' "$3" >>"$PODMAN_CALLS" ;;
  *) exit 2 ;;
esac
`
	if err := os.WriteFile(filepath.Join(fakeBin, "podman"), []byte(fakePodman), 0o700); err != nil {
		t.Fatalf("write fake Podman command: %v", err)
	}
	script := filepath.Join(scratch, "purge-volume.sh")
	if err := os.WriteFile(script, []byte(readRepositoryFile(t, "packaging/purge-volume.sh")), 0o700); err != nil {
		t.Fatalf("write volume purge script: %v", err)
	}
	run := func(labels, input string, args ...string) error {
		t.Helper()
		command := exec.Command("bash", append([]string{script}, args...)...)
		command.Env = append(os.Environ(),
			"PATH="+fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"),
			"LABEL_JSON="+labels,
			"PODMAN_CALLS="+calls,
		)
		command.Stdin = strings.NewReader(input)
		return command.Run()
	}
	ownedLabels := `{"com.docker.compose.project":"review","io.demerzel.data":"true"}`

	if err := run(ownedLabels, "PURGE review_demerzel-data\n", "--project", "review"); err == nil {
		t.Fatal("volume purge ran without the explicit --purge gate")
	}
	if err := run(`{"com.docker.compose.project":"other","io.demerzel.data":"true"}`, "PURGE review_demerzel-data\n", "--project", "review", "--purge"); err == nil {
		t.Fatal("volume purge accepted a volume owned by another Compose project")
	}
	if err := run(`{"com.docker.compose.project":"review","com.docker.compose.volume":"demerzel-data"}`, "PURGE review_demerzel-data\n", "--project", "review", "--purge"); err == nil {
		t.Fatal("volume purge accepted a project volume without the Demerzel data marker")
	}
	if err := run(ownedLabels, "PURGE wrong-volume\n", "--project", "review", "--purge"); err == nil {
		t.Fatal("volume purge accepted a mismatched typed confirmation")
	}
	if _, err := os.Stat(calls); !os.IsNotExist(err) {
		t.Fatalf("rejected purge invoked Podman volume removal: %v", err)
	}
	if err := run(ownedLabels, "PURGE review_demerzel-data\n", "--project", "review", "--purge"); err != nil {
		t.Fatalf("owner-confirmed Podman volume purge failed: %v", err)
	}
	removed, err := os.ReadFile(calls)
	if err != nil {
		t.Fatalf("read fake Podman removal record: %v", err)
	}
	if string(removed) != "review_demerzel-data\n" {
		t.Fatalf("Podman removed %q, want only the owned data volume", removed)
	}
}
