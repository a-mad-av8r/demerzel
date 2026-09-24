//go:build darwin || linux

package encryption

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

func lockKeyCustody(dataDir string) (func(), error) {
	lockPath := filepath.Join(dataDir, keyCustodyLockFile)
	fd, err := unix.Open(lockPath, unix.O_CREAT|unix.O_RDWR|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0o600)
	if err != nil {
		return nil, err
	}
	var status unix.Stat_t
	if err := unix.Fstat(fd, &status); err != nil ||
		status.Mode&unix.S_IFMT != unix.S_IFREG ||
		status.Uid != uint32(os.Geteuid()) {
		_ = unix.Close(fd)
		return nil, os.ErrPermission
	}
	if err := unix.Fchmod(fd, 0o600); err != nil {
		_ = unix.Close(fd)
		return nil, err
	}
	for {
		err = unix.Flock(fd, unix.LOCK_EX)
		if err != unix.EINTR {
			break
		}
	}
	if err != nil {
		_ = unix.Close(fd)
		return nil, err
	}
	return func() {
		_ = unix.Flock(fd, unix.LOCK_UN)
		_ = unix.Close(fd)
	}, nil
}

func openCustodyFile(path string) (*os.File, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	file := os.NewFile(uintptr(fd), path)
	var status unix.Stat_t
	if err := unix.Fstat(fd, &status); err != nil {
		_ = file.Close()
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 ||
		status.Uid != uint32(os.Geteuid()) {
		_ = file.Close()
		return nil, os.ErrPermission
	}
	return file, nil
}

func syncCustodyDirectory(path string) error {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_DIRECTORY, 0)
	if err != nil {
		return err
	}
	defer unix.Close(fd)
	return unix.Fsync(fd)
}
