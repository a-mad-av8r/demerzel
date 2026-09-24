//go:build darwin || linux

package accountscli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"gpt-load/internal/platform/config"
)

func secureAdminSocket(dataDir string) (string, error) {
	dir, err := filepath.Abs(dataDir)
	if err != nil {
		return "", errors.New("invalid application data directory")
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || !ownerOnly(info, 0o700) {
		return "", errors.New("application data directory must be an owner-only directory")
	}
	path := filepath.Join(dir, config.ControlSocketFileName)
	info, err = os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSocket == 0 || !ownerOnly(info, 0o600) {
		return "", fmt.Errorf("local admin socket must be an owner-only socket in the application data directory")
	}
	return path, nil
}

func ownerOnly(info os.FileInfo, allowed os.FileMode) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Uid == uint32(os.Geteuid()) && info.Mode().Perm()&^allowed == 0
}
