package app

import (
	"errors"
	"fmt"
	"net"
	"os"
	"syscall"
	"time"
)

var adminSocketPeer = &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)}

type adminSocketConn struct{ net.Conn }

func (adminSocketConn) RemoteAddr() net.Addr { return adminSocketPeer }

type adminSocketListener struct{ net.Listener }

func (listener adminSocketListener) Accept() (net.Conn, error) {
	conn, err := listener.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return adminSocketConn{Conn: conn}, nil
}

func listenAdminSocket(path string) (net.Listener, error) {
	if path == "" {
		return nil, nil
	}
	if existing, err := os.Lstat(path); err == nil {
		if existing.Mode()&os.ModeSocket == 0 {
			return nil, fmt.Errorf("admin socket path is occupied by a non-socket file")
		}
		conn, dialErr := net.DialTimeout("unix", path, 100*time.Millisecond)
		if dialErr == nil {
			_ = conn.Close()
			return nil, fmt.Errorf("admin socket is already serving")
		}
		if !errors.Is(dialErr, syscall.ECONNREFUSED) {
			return nil, fmt.Errorf("cannot confirm the prior admin socket is stale: %w", dialErr)
		}
		if err := os.Remove(path); err != nil {
			return nil, fmt.Errorf("remove stale admin socket: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("inspect admin socket: %w", err)
	}
	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("bind owner-only admin socket: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = listener.Close()
		return nil, fmt.Errorf("restrict admin socket: %w", err)
	}
	return adminSocketListener{Listener: listener}, nil
}
