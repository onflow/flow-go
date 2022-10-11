//go:build !windows

package io

import (
	"fmt"
	"io/fs"
	"os"
	"syscall"
)

func chown(path string, fileInfo fs.FileInfo) error {
	stat, ok := fileInfo.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("failed to get raw syscall.Stat_t data for '%s'", path)
	}

	return os.Lchown(path, int(stat.Uid), int(stat.Gid))
}
