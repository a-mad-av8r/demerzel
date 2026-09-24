//go:build darwin || linux

package app

import (
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"

	"gpt-load/internal/platform/utils"
	"gpt-load/internal/storage"
)

func TestAppServesLocalAdminSocketOnlyFromProtectedDataRoot(t *testing.T) {
	dataDir, err := os.MkdirTemp("/tmp", "dmz-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dataDir) })
	path := filepath.Join(dataDir, "control.sock")
	cfg := testConfig(t)
	cfg.DataDir = dataDir
	cfg.AdminSocketPath = path
	db, err := storage.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	engine := mustNewEngine(t)
	engine.GET("/socket-canary", func(c *gin.Context) {
		peer, err := utils.NormalizePeerIP(c.Request.RemoteAddr)
		if err != nil || peer != "127.0.0.1" {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusNoContent)
	})
	application := NewApp(AppParams{
		Engine:           engine,
		Config:           cfg,
		DB:               db,
		StartupBootstrap: startupBootstrapFunc(noopStartupBootstrap),
		RuntimeState:     runtimeStateLoaderFunc(func(context.Context) error { return nil }),
		ControlRuntime:   newControlRuntimeFake(nil, false),
		RequestLogs:      newRequestLogRuntimeFake(nil, nil),
	})
	if err := application.Start(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSocket == 0 || info.Mode().Perm() != 0o600 {
		t.Fatalf("admin socket is not owner-only: %v, info=%v", err, info)
	}
	client := &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", path)
		},
	}}
	response, err := client.Get("http://unix/socket-canary")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("owner-local request status = %d", response.StatusCode)
	}
	if err := application.Stop(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("admin socket remained after shutdown: %v", err)
	}
}

func TestAdminSocketRefusesAnExistingNonSocketWithoutDeletingIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.sock")
	if err := os.WriteFile(path, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if listener, err := listenAdminSocket(path); err == nil {
		_ = listener.Close()
		t.Fatal("non-socket admin path was overwritten")
	}
	contents, err := os.ReadFile(path)
	if err != nil || string(contents) != "keep" {
		t.Fatalf("existing non-socket file was changed: %v", err)
	}
}
