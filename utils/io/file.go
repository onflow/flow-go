package io

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"go.uber.org/multierr"
)

// ReadFile reads the file from path, if not found, it will print the absolute path, instead of
// relative path.
func ReadFile(path string) ([]byte, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("could not get absolution path: %w", err)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("could not read file: %w", err)
	}

	return data, nil
}

func FileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// CopyDirectory recursively copies a directory.
// From https://stackoverflow.com/questions/51779243/copy-a-folder-in-go
func CopyDirectory(scrDir, dest string) error {
	entries, err := os.ReadDir(scrDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		sourcePath := filepath.Join(scrDir, entry.Name())
		destPath := filepath.Join(dest, entry.Name())

		fileInfo, err := os.Stat(sourcePath)
		if err != nil {
			return err
		}

		switch fileInfo.Mode() & os.ModeType {
		case os.ModeDir:
			if err := CreateIfNotExists(destPath, 0755); err != nil {
				return err
			}
			if err := CopyDirectory(sourcePath, destPath); err != nil {
				return err
			}
		case os.ModeSymlink:
			if err := CopySymLink(sourcePath, destPath); err != nil {
				return err
			}
		default:
			if err := Copy(sourcePath, destPath); err != nil {
				return err
			}
		}

		if err := chown(destPath, fileInfo); err != nil {
			return err
		}

		fInfo, err := entry.Info()
		if err != nil {
			return err
		}

		isSymlink := fInfo.Mode()&os.ModeSymlink != 0
		if !isSymlink {
			if err := os.Chmod(destPath, fInfo.Mode()); err != nil {
				return err
			}
		}
	}
	return nil
}

func Copy(srcFile, dstFile string) (errToReturn error) {
	in, err := os.Open(srcFile)
	if err != nil {
		return fmt.Errorf("can not open file %v to copy from: %w", srcFile, err)
	}

	defer multierr.AppendInvoke(&errToReturn, multierr.Close(in))

	out, err := os.Create(dstFile)
	if err != nil {
		return fmt.Errorf("can not create file %v to copy to: %w", dstFile, err)
	}

	defer multierr.AppendInvoke(&errToReturn, multierr.Close(out))

	_, err = io.Copy(out, in)
	if err != nil {
		return fmt.Errorf("can not copy file: %w", err)
	}

	return nil
}

func Exists(filePath string) bool {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return false
	}

	return true
}

func CreateIfNotExists(dir string, perm os.FileMode) error {
	if Exists(dir) {
		return nil
	}

	if err := os.MkdirAll(dir, perm); err != nil {
		return fmt.Errorf("failed to create directory: '%s', error: '%s'", dir, err.Error())
	}

	return nil
}

func CopySymLink(source, dest string) error {
	link, err := os.Readlink(source)
	if err != nil {
		return err
	}
	return os.Symlink(link, dest)
}

// MoveFile moves src to dst. It first attempts an atomic rename; if src and dst
// are on different file systems, it falls back to copying the file contents to
// dst, syncing dst, and removing src. The destination file inherits the source
// file's permissions.
//
// No error returns are expected during normal operation.
func MoveFile(src, dst string) error {
	return moveFile(src, dst, os.Rename)
}

func moveFile(src, dst string, renameFn func(string, string) error) error {
	err := renameFn(src, dst)
	if err == nil {
		return nil
	}

	if !isCrossDeviceRenameError(err) {
		return fmt.Errorf("cannot rename %s to %s: %w", src, dst, err)
	}

	if err := copyFileAndRemoveSource(src, dst); err != nil {
		return fmt.Errorf("cannot copy %s to %s after cross-device rename failure: %w", src, dst, err)
	}

	return nil
}

func isCrossDeviceRenameError(err error) bool {
	var linkErr *os.LinkError
	if !errors.As(err, &linkErr) {
		return false
	}
	if errors.Is(linkErr.Err, syscall.EXDEV) {
		return true
	}
	// Some platforms may surface cross-device rename errors as generic errors
	// with "cross-device" in the message.
	if strings.Contains(linkErr.Err.Error(), "cross-device") {
		return true
	}
	return false
}

func copyFileAndRemoveSource(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("cannot open source file %s: %w", src, err)
	}
	defer sourceFile.Close()

	sourceInfo, err := sourceFile.Stat()
	if err != nil {
		return fmt.Errorf("cannot stat source file %s: %w", src, err)
	}

	destinationFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, sourceInfo.Mode().Perm())
	if err != nil {
		return fmt.Errorf("cannot create destination file %s: %w", dst, err)
	}

	_, err = io.Copy(destinationFile, sourceFile)
	if err != nil {
		_ = destinationFile.Close()
		_ = os.Remove(dst)
		return fmt.Errorf("cannot copy file from %s to %s: %w", src, dst, err)
	}

	err = destinationFile.Sync()
	if err != nil {
		_ = destinationFile.Close()
		_ = os.Remove(dst)
		return fmt.Errorf("cannot sync destination file %s: %w", dst, err)
	}

	err = destinationFile.Close()
	if err != nil {
		_ = os.Remove(dst)
		return fmt.Errorf("cannot close destination file %s: %w", dst, err)
	}

	err = os.Remove(src)
	if err != nil {
		return fmt.Errorf("cannot remove source file %s after copy: %w", src, err)
	}

	return nil
}

// EnsureEmptyOrCreate checks that dir is either absent or an empty directory.
// If absent it is created; if non-empty it returns an error.
//
// No error returns are expected during normal operation.
func EnsureEmptyOrCreate(dir string) error {
	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return os.MkdirAll(dir, 0o755)
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s exists but is not a directory", dir)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("cannot read directory %s: %w", dir, err)
	}
	if len(entries) > 0 {
		return fmt.Errorf("directory %s must be empty", dir)
	}
	return nil
}
