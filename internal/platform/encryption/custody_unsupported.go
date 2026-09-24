//go:build !darwin && !linux

package encryption

import (
	"errors"
	"os"
)

func lockKeyCustody(string) (func(), error) {
	return nil, errors.New("master-key custody locking is unsupported on this platform")
}

func openCustodyFile(string) (*os.File, error) {
	return nil, errors.New("secure custody files are unsupported on this platform")
}

func syncCustodyDirectory(string) error {
	return errors.New("secure custody files are unsupported on this platform")
}
