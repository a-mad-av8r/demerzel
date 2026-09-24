//go:build !darwin && !linux

package encryption

import "errors"

func newSystemKeyStore() (masterKeyStore, error) {
	return nil, errors.New("OS key custody is unsupported on this platform")
}
