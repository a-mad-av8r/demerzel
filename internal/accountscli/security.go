package accountscli

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"gpt-load/internal/platform/config"
)

const maxCredentialInputBytes = 1 << 20

func loadAuthKey(dataDir, authKeyPath string) (string, error) {
	if value, exists := os.LookupEnv("AUTH_KEY"); exists {
		key := strings.TrimSpace(value)
		if key != "" {
			if strings.ContainsAny(key, "\r\n") {
				return "", errors.New("AUTH_KEY environment variable is invalid")
			}
			return key, nil
		}
	}
	if authKeyPath == "" {
		if dataDir == "" {
			var err error
			dataDir, err = config.DefaultDataDir()
			if err != nil {
				return "", err
			}
		}
		authKeyPath = filepath.Join(dataDir, "auth.key")
	}
	contents, err := readOwnerOnlyFile(authKeyPath, 4096)
	if err != nil {
		return "", errors.New("could not read an owner-only management key file")
	}
	key := strings.TrimSpace(string(contents))
	clear(contents)
	if key == "" || strings.ContainsAny(key, "\r\n") {
		return "", errors.New("management key file is empty or invalid")
	}
	return key, nil
}

func readSingleCredential(input io.Reader) ([]byte, error) {
	if input == nil {
		return nil, errors.New("credential input is unavailable")
	}
	contents, err := io.ReadAll(io.LimitReader(input, maxCredentialInputBytes+1))
	if err != nil || len(contents) > maxCredentialInputBytes {
		clear(contents)
		return nil, errors.New("could not read credential input")
	}
	if !utf8.Valid(contents) {
		clear(contents)
		return nil, errors.New("credential input must be a single UTF-8 API key")
	}
	contents = bytes.TrimSuffix(contents, []byte("\n"))
	contents = bytes.TrimSuffix(contents, []byte("\r"))
	if len(bytes.TrimSpace(contents)) == 0 || bytes.ContainsAny(contents, "\r\n") {
		clear(contents)
		return nil, errors.New("credential input must contain exactly one API key")
	}
	return contents, nil
}

func readCredentialFile(path string) ([]byte, error) {
	contents, err := readOwnerOnlyFile(path, maxCredentialInputBytes)
	if err != nil {
		return nil, errors.New("credential file must be a restrictive regular file")
	}
	credential, err := readSingleCredential(bytes.NewReader(contents))
	clear(contents)
	return credential, err
}

func readOwnerOnlyFile(path string, maxBytes int64) ([]byte, error) {
	before, err := os.Lstat(path)
	if err != nil || !before.Mode().IsRegular() || !isOwnerOnlyFile(path, before) {
		return nil, errors.New("file is not owner-only")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, errors.New("file is unavailable")
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !isOwnerOnlyFile(path, opened) {
		return nil, errors.New("file is not owner-only")
	}
	after, err := os.Lstat(path)
	if err != nil || !os.SameFile(before, opened) || !os.SameFile(opened, after) {
		return nil, errors.New("file changed while opening")
	}
	if opened.Size() > maxBytes {
		return nil, errors.New("file exceeds the allowed size")
	}
	contents, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil || int64(len(contents)) > maxBytes {
		clear(contents)
		return nil, errors.New("file could not be read")
	}
	return contents, nil
}
