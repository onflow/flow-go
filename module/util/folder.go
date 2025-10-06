package util

import (
	"fmt"
	"os"
)

// IsEmptyOrNotExists returns true if the directory does not exist or is empty.
// It returns an error if there's an issue accessing the directory.
func IsEmptyOrNotExists(path string) (bool, error) {
	// Check if the path exists
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		// Directory does not exist
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("error stating path %s: %w", path, err)
	}
	if !info.IsDir() {
		return false, fmt.Errorf("path %s exists but is not a directory", path)
	}

	// Read directory contents
	files, err := os.ReadDir(path)
	if err != nil {
		return false, fmt.Errorf("error reading directory %s: %w", path, err)
	}

	// If the directory has no entries, it's empty
	return len(files) == 0, nil
}
