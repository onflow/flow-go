package wal

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/rs/zerolog"
)

type WriterSeekerCloser interface {
	io.Writer
	io.Seeker
	io.Closer
}

// SyncOnCloseRenameFile is a composite of buffered writer over a given file
// which flushes/sync on closing and renames to `targetName` as a last step.
// Typical usecase is to write data to a temporary file and only rename it
// to target one as the last step. This help avoid situation when writing is
// interrupted  and unusable file but with target name exists.
type SyncOnCloseRenameFile struct {
	logger     zerolog.Logger
	file       *os.File
	targetName string
	savedError error // savedError is the first error returned from Write.  Close() renames temp file to target file only if savedError is nil.
	*bufio.Writer
}

func (s *SyncOnCloseRenameFile) Sync() error {
	err := s.Flush()
	if err != nil {
		return fmt.Errorf("cannot flush buffer: %w", err)
	}
	return s.file.Sync()

}

func (s *SyncOnCloseRenameFile) Close() error {
	if s.savedError != nil {
		// If there is any error saved from previous op, close temp file without renaming to target file.
		return s.closeOnError()
	}

	err := s.Flush()
	if err != nil {
		return fmt.Errorf("cannot flush buffer: %w", err)
	}

	err = s.file.Sync()
	if err != nil {
		return fmt.Errorf("cannot sync file %s: %w", s.file.Name(), err)
	}

	// s.file.Sync() was already called, so we pass fsync=false
	err = evictFileFromLinuxPageCache(s.file, false, s.logger)
	if err != nil {
		s.logger.Warn().Msgf("failed to evict file %s from Linux page cache: %s", s.targetName, err)
		// No need to return this error because we're only "advising" Linux to evict a file from cache.
	}

	err = s.file.Close()
	if err != nil {
		return fmt.Errorf("error while closing file %s: %w", s.file.Name(), err)
	}

	err = os.Rename(s.file.Name(), s.targetName)
	if err != nil {
		return fmt.Errorf("error while renaming from %s to %s: %w", s.file.Name(), s.targetName, err)
	}

	return nil
}

func (s *SyncOnCloseRenameFile) Write(b []byte) (int, error) {
	n, err := s.Writer.Write(b)
	if err != nil && s.savedError == nil {
		s.savedError = err
	}
	return n, err
}

// closeOnError closes and deletes temp file.
func (s *SyncOnCloseRenameFile) closeOnError() error {
	fileName := s.file.Name()

	// Close temp file
	err := s.file.Close()
	if err != nil {
		return err
	}

	// Remove temp file because it is incomplete/invalid.
	return os.Remove(fileName)
}
