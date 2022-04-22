//go:build !linux
// +build !linux

package wal

// fadviseNoLinuxPageCache does nothing if GOOS != "linux".
// Otherwise, fadviseNoLinuxPageCache advises Linux to evict
// a file from Linux page cache.  This calls unix.Fadvise which
// in turn calls posix_fadvise with POSIX_FADV_DONTNEED.
// CAUTION: If fsync=true, this will call unix.Fsync which
// can cause performance hit especially when used inside a loop.
func fadviseNoLinuxPageCache(fd uintptr, fsync bool) error {
	return nil
}

// fadviseNoLinuxPageCache does nothing if GOOS != "linux".
// Otherwise, fadviseSegmentNoLinuxPageCache advises Linux to evict
// file segment from Linux page cache.  This calls unix.Fadvise
// which in turn calls posix_fadvise with POSIX_FADV_DONTNEED.
// If GOOS != "linux" then this function does nothing.
// CAUTION: If fsync=true, this will call unix.Fsync which
// can cause performance hit especially when used inside a loop.
func fadviseSegmentNoLinuxPageCache(fd uintptr, off, len int64, fsync bool) error {
	return nil
}
