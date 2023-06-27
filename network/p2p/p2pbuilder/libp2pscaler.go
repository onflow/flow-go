package p2pbuilder

import (
	"fmt"
	"math"

	"github.com/pbnjay/memory"
	"golang.org/x/sys/unix"
)

func getNumFDs() (int, error) {
	var l unix.Rlimit
	if err := unix.Getrlimit(unix.RLIMIT_NOFILE, &l); err != nil {
		return 0, err
	}
	return int(l.Cur), nil
}

func allowedMemory(scaleFactor float64) (int64, error) {
	if scaleFactor <= 0 || scaleFactor > 1 {
		return 0, fmt.Errorf("memory scale factor must be greater than 0 and less than or equal to 1: %f", scaleFactor)
	}
	return int64(math.Floor(float64(memory.TotalMemory()) * scaleFactor)), nil
}

func allowedFileDescriptors(scaleFactor float64) (int, error) {
	if scaleFactor <= 0 || scaleFactor > 1 {
		return 0, fmt.Errorf("fd scale factor must be greater than 0 and less than or equal to 1: %f", scaleFactor)
	}
	fd, err := getNumFDs()
	if err != nil {
		return 0, fmt.Errorf("failed to get file descriptor limit: %w", err)
	}
	return int(math.Floor(float64(fd) * scaleFactor)), nil
}
