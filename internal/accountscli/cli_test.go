package accountscli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"gpt-load/internal/platform/securefile"
)

const testManagementKey = "synthetic-management-key"
const testAccountKey = "synthetic-upstream-key-never-print"
const testResponseSecret = "synthetic-response-secret-never-print"

type fakeCredential struct {
	id     uint
	label  string
	status string
}

type fakeManagementAPI struct {
	mu                    sync.Mutex
	server                *httptest.Server
	credential            *fakeCredential
	importedSecret        string
	importIdempotencyKey  string
	lastBatchAction       string
	rejectAuthentication bool
}

func newFakeManagementAPI() *fakeManagementAPI {
	fixture := &fakeManagementAPI{}
	fixture.server = httptest.NewServer(http.HandlerFunc(fixture.serveHTTP))
	return fixture
}

func (fixture *fakeManagementAPI) close() {
	fixture.server.Close()
}

func (fixture *fakeManagementAPI) serveHTTP(w http.ResponseWriter, request *http.Request) {
	fixture.mu.Lock()
	rejectAuthentication := fixture.rejectAuthentication
	fixture.mu.Unlock()
	if rejectAuthentication || request.Header.Get("Authorization") != "Bearer "+testManagementKey {
		w.WriteHeader(http.StatusUnauthorized)
		writeJSON(w, map[string]any{
			"code":    "AUTH_FAILED",
			"message": testResponseSecret,
			"data":    nil,
		})
		return
	}

	switch {
	case request.Method == http.MethodGet && request.URL.Path == "/api/modern/groups":
		writeSuccess(w, map[string]any{
			"items": []any{
				map[string]any{
					"id": 7, "name": "Primary group", "channel_id": "openai",
					"channel_name": "OpenAI", "connection_type": "api_key",
				},
			},
		})
		fixture.listCredentials(w, request)
	case request.Method == http.MethodPost && request.URL.Path == "/api/groups/7/credentials/import":
		fixture.importCredential(w, request)
	case request.Method == http.MethodPost && request.URL.Path == "/api/groups/7/credentials/batch":
		fixture.updateStatus(w, request)
	case request.Method == http.MethodPut && request.URL.Path == "/api/groups/7/credentials/41":
		fixture.updateLabel(w, request)
	case request.Method == http.MethodDelete && request.URL.Path == "/api/groups/7/credentials/41":
		fixture.removeCredential(w, request)
	default:
		http.NotFound(w, request)
	}
}

func (fixture *fakeManagementAPI) listCredentials(w http.ResponseWriter, request *http.Request) {
	if request.URL.Query().Get("page") != "1" || request.URL.Query().Get("page_size") != "100" {
		http.Error(w, "invalid pagination", http.StatusBadRequest)
		return
	}
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	items := []any{}
	totalPages := 0
	if fixture.credential != nil {
		items = append(items, fixture.credentialResponse())
		totalPages = 1
	}
	writeSuccess(w, map[string]any{
		"items": items,
		"pagination": map[string]any{
			"page": 1, "page_size": 100, "total_items": len(items), "total_pages": totalPages,
		},
	})
}

func (fixture *fakeManagementAPI) importCredential(w http.ResponseWriter, request *http.Request) {
	var body struct {
		Credentials string `json:"credentials"`
	}
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil || body.Credentials == "" {
		http.Error(w, "invalid credential import", http.StatusBadRequest)
		return
	}
	idempotencyKey := request.Header.Get("Idempotency-Key")
	if len(idempotencyKey) != 36 || strings.Count(idempotencyKey, "-") != 4 {
		http.Error(w, "missing idempotency key", http.StatusBadRequest)
		return
	}
	fixture.mu.Lock()
	fixture.importedSecret = body.Credentials
	fixture.importIdempotencyKey = idempotencyKey
	fixture.credential = &fakeCredential{id: 41, status: "active"}
	fixture.mu.Unlock()
	writeSuccess(w, map[string]any{"group_id": 7, "credentials_added": 1, "credentials_duplicated": 0})
}

func (fixture *fakeManagementAPI) updateStatus(w http.ResponseWriter, request *http.Request) {
	var body struct {
		Action        string `json:"action"`
		CredentialIDs []uint `json:"credential_ids"`
		Scope         string `json:"scope"`
	}
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil || len(body.CredentialIDs) != 1 || body.CredentialIDs[0] != 41 || body.Scope != "" {
		http.Error(w, "expected one explicit credential ID", http.StatusBadRequest)
		return
	}
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	if fixture.credential == nil {
		http.NotFound(w, request)
		return
	}
	switch body.Action {
	case "disable":
		fixture.credential.status = "disabled"
	case "enable":
		fixture.credential.status = "active"
	default:
		http.Error(w, "unexpected status action", http.StatusBadRequest)
		return
	}
	fixture.lastBatchAction = body.Action
	writeSuccess(w, map[string]any{
		"affected_credential_ids": []uint{41},
		"summary":                 map[string]any{"total": 1, "available": 1, "disabled": 0},
	})
}

func (fixture *fakeManagementAPI) updateLabel(w http.ResponseWriter, request *http.Request) {
	var body struct {
		Label string `json:"label"`
	}
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		http.Error(w, "invalid label update", http.StatusBadRequest)
		return
	}
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	if fixture.credential == nil {
		http.NotFound(w, request)
		return
	}
	fixture.credential.label = body.Label
	writeSuccess(w, fixture.credentialResponse())
}

func (fixture *fakeManagementAPI) removeCredential(w http.ResponseWriter, request *http.Request) {
	var body map[string]any
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil || len(body) != 0 {
		http.Error(w, "expected an empty object", http.StatusBadRequest)
		return
	}
	fixture.mu.Lock()
	fixture.credential = nil
	fixture.mu.Unlock()
	writeSuccess(w, nil)
}

func (fixture *fakeManagementAPI) credentialResponse() map[string]any {
	item := fixture.credential
	resetAt := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC).UnixMilli()
	return map[string]any{
		"credential_id": item.id,
		"label":         item.label,
		"mask":          "sk-...cafe",
		"account":       map[string]any{"email": "owner@example.test", "email_mask": "o***@example.test"},
		"auth_state":    "ready",
		"configured_status": item.status,
		"effective_status":  item.status,
		"recent_success_count": 8,
		"recent_failure_count": 1,
		"consecutive_failure_count": 0,
		"last_failure_category": "rate_limited",
		"cooldown_until_ms": resetAt,
		"model_cooldowns": []any{},
		"observation": map[string]any{
			"state": "ready",
			"snapshot": map[string]any{
				"account_summary": map[string]any{"display_name": "Owner", "email": "owner@example.test"},
				"quota_windows": []any{map[string]any{
					"id": "daily", "label": "Daily", "unit": "requests",
					"remaining": 73.0, "limit": 100.0, "state": "available", "reset_at_ms": resetAt,
				}},
			},
		},
	}
}

func TestAccountLifecycleUsesAuthenticatedControlAPIAndRedactsSecrets(t *testing.T) {
	t.Setenv("AUTH_KEY", "")
	fixture := newFakeManagementAPI()
	defer fixture.close()
	authFile := writeOwnerOnlyTestFile(t, "auth.key", []byte(testManagementKey))
	connection := []string{"--url", fixture.server.URL, "--auth-key-file", authFile}

	stdout, stderr, code := invoke(t, []string{"add", "--group", "7", "--stdin"}, connection, testAccountKey+"\n")
	if code != 0 || stderr != "" || stdout != "Added one account to group 7.\n" {
		t.Fatal("add did not report a successful account import")
	}
	assertRedacted(t, stdout, stderr)
	fixture.mu.Lock()
	if fixture.importedSecret != testAccountKey || fixture.importIdempotencyKey == "" {
		fixture.mu.Unlock()
		t.Fatal("add did not send the stdin secret through the import API")
	}
	fixture.mu.Unlock()

	stdout, stderr, code = invoke(t, []string{"list", "--group", "7"}, connection, "")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "GROUP_ID") ||
		!strings.Contains(stdout, "owner@example.test") || !strings.Contains(stdout, "active/active") ||
		!strings.Contains(stdout, "Daily{remaining=73,limit=100,unit=requests,state=available") ||
		!strings.Contains(stdout, "2026-10-01T12:00:00Z") || !strings.Contains(stdout, "auth=ready") {
		t.Fatal("list did not render account identity, status, quota, cooldown, and health")
	}
	assertRedacted(t, stdout, stderr)

	stdout, stderr, code = invoke(t, []string{"label", "--group", "7", "--credential", "41", "--label", "Codex operator"}, connection, "")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "Updated label") {
		t.Fatal("label did not report a confirmed update")
	}
	fixture.mu.Lock()
	labelUpdated := fixture.credential != nil && fixture.credential.label == "Codex operator"
	fixture.mu.Unlock()
	if !labelUpdated {
		t.Fatal("label update was not persisted by the control API")
	}
	assertRedacted(t, stdout, stderr)

	stdout, stderr, code = invoke(t, []string{"label", "--group", "7", "--credential", "41", "--label", ""}, connection, "")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "Cleared label") {
		t.Fatal("empty label did not clear the credential label")
	}
	fixture.mu.Lock()
	labelCleared := fixture.credential != nil && fixture.credential.label == ""
	fixture.mu.Unlock()
	if !labelCleared {
		t.Fatal("empty label was not persisted by the control API")
	}
	assertRedacted(t, stdout, stderr)

	stdout, stderr, code = invoke(t, []string{"disable", "--group", "7", "--credential", "41"}, connection, "")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "Disabled credential 41") {
		t.Fatal("disable did not report a confirmed status change")
	}
	fixture.mu.Lock()
	disabled := fixture.credential != nil && fixture.credential.status == "disabled" && fixture.lastBatchAction == "disable"
	fixture.mu.Unlock()
	if !disabled {
		t.Fatal("disable did not mutate the explicit credential through the batch API")
	}
	assertRedacted(t, stdout, stderr)

	stdout, stderr, code = invoke(t, []string{"restore", "--group", "7", "--credential", "41"}, connection, "")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "Restored credential 41") {
		t.Fatal("restore did not report a confirmed status change")
	}
	fixture.mu.Lock()
	restored := fixture.credential != nil && fixture.credential.status == "active" && fixture.lastBatchAction == "enable"
	fixture.mu.Unlock()
	if !restored {
		t.Fatal("restore did not re-enable the explicit credential")
	}
	assertRedacted(t, stdout, stderr)

	stdout, stderr, code = invoke(t, []string{"remove", "--group", "7", "--credential", "41"}, connection, "")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "Removed credential 41") {
		t.Fatal("remove did not report a successful deletion")
	}
	fixture.mu.Lock()
	removed := fixture.credential == nil
	fixture.mu.Unlock()
	if !removed {
		t.Fatal("remove did not delete the explicit credential")
	}
	assertRedacted(t, stdout, stderr)

	keyFile := writeOwnerOnlyTestFile(t, "account.key", []byte(testAccountKey+"\n"))
	stdout, stderr, code = invoke(t, []string{"add", "--group", "7", "--key-file", keyFile}, connection, "")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "Added one account") {
		t.Fatal("add did not accept a restrictive credential file")
	}
	assertRedacted(t, stdout, stderr)
}

func TestAccountCLIRejectsInsecureCredentialFileAndRemoteControlURL(t *testing.T) {
	t.Setenv("AUTH_KEY", "")
	fixture := newFakeManagementAPI()
	defer fixture.close()
	authFile := writeOwnerOnlyTestFile(t, "auth.key", []byte(testManagementKey))
	if runtime.GOOS != "windows" {
		keyFile := filepath.Join(t.TempDir(), "credential.key")
		if err := os.WriteFile(keyFile, []byte(testAccountKey), 0o644); err != nil {
			t.Fatal("could not create credential test file")
		}
		if err := os.Chmod(keyFile, 0o644); err != nil {
			t.Fatal("could not make the credential test file readable by other users")
		}

		stdout, stderr, code := invoke(t, []string{"add", "--group", "7", "--key-file", keyFile},
			[]string{"--url", fixture.server.URL, "--auth-key-file", authFile}, "")
		if code == 0 || !strings.Contains(stderr, "restrictive regular file") {
			t.Fatal("add accepted a credential file readable by other users")
		}
		assertRedacted(t, stdout, stderr)
	}

	stdout, stderr, code := invoke(t, []string{"list"},
		[]string{"--url", "http://example.com", "--auth-key-file", authFile}, "")
	if code == 0 || !strings.Contains(stderr, "loopback") {
		t.Fatal("CLI accepted a non-loopback control API")
	}
	assertRedacted(t, stdout, stderr)
}

func TestAccountCLIHidesControlAPIErrorBodies(t *testing.T) {
	t.Setenv("AUTH_KEY", "")
	fixture := newFakeManagementAPI()
	defer fixture.close()
	fixture.mu.Lock()
	fixture.rejectAuthentication = true
	fixture.mu.Unlock()
	authFile := writeOwnerOnlyTestFile(t, "auth.key", []byte(testManagementKey))

	stdout, stderr, code := invoke(t, []string{"add", "--group", "7", "--stdin"},
		[]string{"--url", fixture.server.URL, "--auth-key-file", authFile}, testAccountKey)
	if code == 0 || !strings.Contains(stderr, "HTTP 401") {
		t.Fatal("unauthorized control API response did not produce a nonzero safe error")
	}
	assertRedacted(t, stdout, stderr)
}

func TestAccountAddRejectsCredentialArguments(t *testing.T) {
	t.Setenv("AUTH_KEY", "")
	var stdout, stderr strings.Builder
	code := Run([]string{"add", "--group", "7", testAccountKey}, strings.NewReader(""), &stdout, &stderr)
	if code == 0 {
		t.Fatal("add accepted a credential supplied as a command-line argument")
	}
	assertRedacted(t, stdout.String(), stderr.String())
}

func invoke(t *testing.T, command, connection []string, stdin string) (string, string, int) {
	t.Helper()
	args := append(append([]string{}, command...), connection...)
	var stdout, stderr strings.Builder
	code := Run(args, strings.NewReader(stdin), &stdout, &stderr)
	return stdout.String(), stderr.String(), code
}

func writeOwnerOnlyTestFile(t *testing.T, name string, contents []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal("could not create owner-only test file")
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal("could not restrict test file permissions")
	}
	if err := securefile.HardenManagedFileIfExists(path); err != nil {
		t.Fatal("could not configure an owner-only test file")
	}
	return path
}

func TestDefaultDataDirUsesPlatformInstallPathWithoutCreatingIt(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	var paths []string
	switch runtime.GOOS {
	case "darwin":
		t.Setenv("HOME", home)
		path, err := defaultDataDir()
		if err != nil {
			t.Fatal("could not resolve macOS data root")
		}
		want := filepath.Join(home, ".demerzel")
		if path != want {
			t.Fatal("macOS data root did not match the product install path")
		}
		paths = append(paths, path)
	case "linux":
		t.Setenv("HOME", home)
		t.Setenv("XDG_DATA_HOME", "")
		path, err := defaultDataDir()
		if err != nil {
			t.Fatal("could not resolve default Linux data root")
		}
		want := filepath.Join(home, ".local", "share", "demerzel")
		if path != want {
			t.Fatal("Linux data root did not use the XDG default")
		}
		paths = append(paths, path)

		xdgRoot := filepath.Join(t.TempDir(), "xdg")
		t.Setenv("XDG_DATA_HOME", xdgRoot)
		path, err = defaultDataDir()
		if err != nil || path != filepath.Join(xdgRoot, "demerzel") {
			t.Fatal("Linux data root did not honor XDG_DATA_HOME")
		}
		paths = append(paths, path)

		t.Setenv("XDG_DATA_HOME", "relative-data")
		if _, err := defaultDataDir(); err == nil {
			t.Fatal("relative XDG_DATA_HOME was accepted")
		}
	default:
		t.Skip("platform-specific install root")
	}
	for _, path := range paths {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("resolving the default data root created or exposed a path")
		}
	}
}

func assertRedacted(t *testing.T, outputs ...string) {
	t.Helper()
	for _, output := range outputs {
		if strings.Contains(output, testAccountKey) || strings.Contains(output, testManagementKey) || strings.Contains(output, testResponseSecret) {
			t.Fatal("CLI output exposed a credential")
		}
	}
}

func writeSuccess(w http.ResponseWriter, data any) {
	writeJSON(w, map[string]any{"code": "0", "message": "Success", "data": data})
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		fmt.Fprint(w, "{}")
	}
}
