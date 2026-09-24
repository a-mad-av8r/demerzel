//go:build !windows

package accountscli

import "os"

func isOwnerOnlyFile(_ string, info os.FileInfo) bool {
	return info.Mode().Perm()&0o177 == 0
}
