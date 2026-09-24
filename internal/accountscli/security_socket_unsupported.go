//go:build !darwin && !linux

package accountscli

import "errors"

func secureAdminSocket(string) (string, error) {
	return "", errors.New("account management requires an owner-only Unix socket on macOS or Linux")
}
