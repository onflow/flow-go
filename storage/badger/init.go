package badger

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/storage/badger/operation"
)

// InitPublic initializes a public database by checking and setting the database
// type marker. If an existing, inconsistent type marker is set, this method will
// return an error. Once a database type marker has been set using these methods,
// the type cannot be changed.
func InitPublic(opts badger.Options) (*badger.DB, error) {
	db, err := SafeOpen(opts)
	if err != nil {
		return nil, fmt.Errorf("could not open db: %w", err)
	}
	err = db.Update(operation.InsertPublicDBMarker)
	if err != nil {
		// Close db before returning error.
		db.Close()

		return nil, fmt.Errorf("could not assert db type: %w", err)
	}

	return db, nil
}

// InitSecret initializes a secrets database by checking and setting the database
// type marker. If an existing, inconsistent type marker is set, this method will
// return an error. Once a database type marker has been set using these methods,
// the type cannot be changed.
func InitSecret(opts badger.Options) (*badger.DB, error) {
	db, err := SafeOpen(opts)
	if err != nil {
		return nil, fmt.Errorf("could not open db: %w", err)
	}
	err = db.Update(operation.InsertSecretDBMarker)
	if err != nil {
		// Close db before returning error.
		db.Close()

		return nil, fmt.Errorf("could not assert db type: %w", err)
	}

	return db, nil
}

func IsBadgerFolder(dataDir string) (bool, error) {
	// Check if the directory exists
	info, err := os.Stat(dataDir)
	if err != nil {
		return false, err
	}
	if !info.IsDir() {
		return false, errors.New("provided path is not a directory")
	}

	// Flags to indicate presence of key BadgerDB files
	var hasKeyRegistry, hasVLOG, hasManifest bool

	err = filepath.WalkDir(dataDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		name := d.Name()
		switch {
		case strings.HasSuffix(name, ".vlog"):
			hasVLOG = true
		case name == "KEYREGISTRY":
			hasKeyRegistry = true
		case name == "MANIFEST":
			hasManifest = true
		}

		// Short-circuit once we know it's a Badger folder
		if hasKeyRegistry && hasVLOG && hasManifest {
			return fs.SkipDir
		}
		return nil
	})

	if err != nil && !errors.Is(err, fs.SkipDir) {
		return false, err
	}

	isBadger := hasKeyRegistry && hasVLOG && hasManifest
	return isBadger, nil
}

func MustBeBadgerFolder(dataDir string) error {
	ok, err := isEmptyOrNotExists(dataDir)
	if err != nil {
		return fmt.Errorf("error checking if folder is empty or does not exist: %w", err)
	}

	// if the folder is empty or does not exist, then it can be used as a Badger folder
	if ok {
		return nil
	}

	isBadger, err := IsBadgerFolder(dataDir)
	if err != nil {
		return fmt.Errorf("error checking if folder is a Badger folder: %w", err)
	}
	if !isBadger {
		return fmt.Errorf("folder %s is not a Badger folder", dataDir)
	}
	return nil
}

// isEmptyOrNotExists returns true if the directory does not exist or is empty.
// It returns an error if there's an issue accessing the directory.
func isEmptyOrNotExists(path string) (bool, error) {
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

// SafeOpen opens a Badger database with the provided options, ensuring that the
// directory is a valid Badger folder. If the directory is not valid, it returns an error.
// This is useful to prevent accidental opening of a non-Badger (pebble) directory as a Badger database,
// which could wipe out the existing data.
func SafeOpen(opts badger.Options) (*badger.DB, error) {
	// Check if the directory is a Badger folder
	err := MustBeBadgerFolder(opts.Dir)
	if err != nil {
		return nil, fmt.Errorf("could not assert badger folder: %w", err)
	}

	// Open the database
	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("could not open db: %w", err)
	}

	return db, nil
}
