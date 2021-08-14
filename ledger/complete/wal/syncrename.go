package wal

import (
	"bufio"
	"fmt"
	"io"
	"os"
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
	file       *os.File
	targetName string
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
	err := s.Flush()
	if err != nil {
		return fmt.Errorf("cannot flush buffer: %w", err)
	}

	err = s.file.Sync()
	if err != nil {
		return fmt.Errorf("cannot sync file %s: %w", s.file.Name(), err)
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
