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

	db, err := badger.Open(opts)
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

	db, err := badger.Open(opts)
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
