package p2pbuilder

import (
	"fmt"
	"github.com/pbnjay/memory"
	"golang.org/x/sys/unix"
	"math"
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
		return 0, fmt.Errorf("scale factor must be greater than 0 and less than or equal to 1: %f", scaleFactor)
	}
	return int64(math.Floor(float64(memory.TotalMemory()) * scaleFactor)), nil
}

func allowedFileDescriptors(scaleFactor float64) (int, error) {
	if scaleFactor < 0 || scaleFactor > 1 {
		return 0, fmt.Errorf("scale factor must be between 0 and 1: %f", scaleFactor)
	}
	fd, err := getNumFDs()
	if err != nil {
		return 0, fmt.Errorf("failed to get file descriptor limit: %w", err)
	}
	return int(math.Ceil(float64(fd) * scaleFactor)), nil
}
