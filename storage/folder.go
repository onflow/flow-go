package storage

import (
	"os"
	"strings"
)

// CheckFolder checks the given folder path and returns whether it is a Pebble folder, a Badger folder, if it is empty, and any error encountered.
func CheckFolder(folderPath string) (isPebbleFolder, isBadgerFolder, isEmpty bool, err error) {
	files, err := os.ReadDir(folderPath)
	if err != nil {
		return false, false, false, err
	}

	if len(files) == 0 {
		return false, false, true, nil
	}

	isPebbleFolder = false
	isBadgerFolder = false

	hasManifest := false
	hasCurrent := false
	hasLog := false
	hasSst := false
	hasVlog := false

	for _, file := range files {
		switch {
		case strings.HasPrefix(file.Name(), "MANIFEST-"):
			hasManifest = true
		case file.Name() == "CURRENT":
			hasCurrent = true
		case file.Name() == "LOG":
			hasLog = true
		case strings.HasSuffix(file.Name(), ".sst"):
			hasSst = true
		case strings.HasSuffix(file.Name(), ".vlog"):
			hasVlog = true
		}
	}

	// Determine if the folder contains Pebble data
	if hasManifest && hasCurrent && hasLog && hasSst {
		isPebbleFolder = true
	}

	// Determine if the folder contains Badger data
	if hasManifest && hasSst && hasVlog {
		isBadgerFolder = true
	}

	return isPebbleFolder, isBadgerFolder, false, nil
}
