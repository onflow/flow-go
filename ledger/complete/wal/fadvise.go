package wal

import (
	"runtime"

	"golang.org/x/sys/unix"
)

// fadviseNoLinuxPageCache advises Linux to evict
// a file from Linux page cache.  This calls unix.Fadvise which
// in turn calls posix_fadvise with POSIX_FADV_DONTNEED.
// If GOOS != "linux" then this function does nothing.
// CAUTION: If fsync=true, this will call unix.Fsync which
// can cause performance hit especially when used inside a loop.
func fadviseNoLinuxPageCache(fd uintptr, fsync bool) error {
	return fadviseSegmentNoLinuxPageCache(fd, 0, 0, fsync)
}

// fadviseSegmentNoLinuxPageCache advises Linux to evict the
// file segment from Linux page cache.  This calls unix.Fadvise
// which in turn calls posix_fadvise with POSIX_FADV_DONTNEED.
// If GOOS != "linux" then this function does nothing.
// CAUTION: If fsync=true, this will call unix.Fsync which
// can cause performance hit especially when used inside a loop.
func fadviseSegmentNoLinuxPageCache(fd uintptr, off, len int64, fsync bool) error {
	if runtime.GOOS != "linux" {
		return nil
	}

	// Optionally call fsync because dirty pages won't be evicted.
	if fsync {
		_ = unix.Fsync(int(fd)) // ignore error because this is optional
	}

	// Fadvise under the hood calls posix_fadvise.
	// posix_fadvise doc from man page says:
	// "posix_fadvise - predeclare an access pattern for file data"
	// "The advice applies to a (not necessarily existent) region
	// starting at offset and extending for len bytes (or until
	// the end of the file if len is 0) within the file referred
	// to by fd.  The advice is not binding; it merely constitutes
	// an expectation on behalf of the application."
	return unix.Fadvise(int(fd), off, len, unix.FADV_DONTNEED)
}
