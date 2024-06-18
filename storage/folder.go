package storage

import (
	"fmt"
	"os"
	"strings"
)

// CheckFolder checks the given folder path and returns whether it is a Pebble folder, a Badger folder, if it is empty, and any error encountered.
func CheckFolder(folderPath string) (isPebbleFolder, isBadgerFolder, isEmpty bool, err error) {
	// Check if the folder exists
	info, err := os.Stat(folderPath)
	if os.IsNotExist(err) {
		// if folder does not exist, consider it as empty, since
		// the database module is able to create it.
		return false, false, true, nil
	}
	if err != nil {
		return false, false, false, err
	}

	// Check if the path is a directory
	if !info.IsDir() {
		return false, false, false, fmt.Errorf("provided path is not a directory")
	}

	files, err := os.ReadDir(folderPath)
	if err != nil {
		return false, false, false, err
	}

	if len(files) == 0 {
		return false, false, true, nil
	}

	isPebbleFolder = false
	isBadgerFolder = false

	hasPebbleManifest := false
	hasBadgerManifest := false
	hasKeyRegistery := false
	hasCurrent := false
	hasLog := false
	hasVlog := false

	for _, file := range files {
		fileName := file.Name()
		switch {
		case strings.HasPrefix(file.Name(), "MANIFEST-"):
			hasPebbleManifest = true
		case fileName == "MANIFEST":
			hasBadgerManifest = true
		case fileName == "CURRENT":
			hasCurrent = true
		case fileName == "KEYREGISTRY":
			hasKeyRegistery = true
		case strings.HasSuffix(fileName, ".log"):
			hasLog = true
		case strings.HasSuffix(fileName, ".vlog"):
			hasVlog = true
		}
	}

	// Determine if the folder contains Pebble data
	if hasPebbleManifest && hasCurrent && hasLog {
		isPebbleFolder = true
	}

	// Determine if the folder contains Badger data
	if hasBadgerManifest && hasKeyRegistery && hasVlog {
		isBadgerFolder = true
	}

	return isPebbleFolder, isBadgerFolder, false, nil
}
