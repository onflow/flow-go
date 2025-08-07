package migration

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cockroachdb/pebble/v2"
	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog/log"

	badgerstorage "github.com/onflow/flow-go/storage/badger"
	pebblestorage "github.com/onflow/flow-go/storage/pebble"
	"github.com/onflow/flow-go/storage/util"
)

var DefaultMigrationConfig = MigrationConfig{
	BatchByteSize:          32_000_000, // 32 MB
	ReaderWorkerCount:      2,
	WriterWorkerCount:      2,
	ReaderShardPrefixBytes: 2,                 // better to keep it as 2.
	ValidationMode:         PartialValidation, // Default to partial validation
}

func RunMigrationAndCompaction(badgerDir string, pebbleDir string, cfg MigrationConfig) error {
	err := RunMigration(badgerDir, pebbleDir, cfg)
	if err != nil {
		return err
	}

	err = ForceCompactPebbleDB(pebbleDir)
	if err != nil {
		return fmt.Errorf("failed to compact PebbleDB: %w", err)
	}

	return nil
}

// RunMigration performs a complete migration of key-value data from a BadgerDB directory
// to a PebbleDB directory and verifies the integrity of the migrated data.
//
// It executes the following steps:
//
//  1. Validates that the Badger directory exists and is non-empty.
//     Ensures that the Pebble directory does not already contain data.
//  2. Opens both databases and runs the migration using CopyFromBadgerToPebble with the given config.
//  3. Writes a "MIGRATION_STARTED" marker file with a timestamp in the Pebble directory.
//  4. After migration, performs validation by:
//     - For PartialValidation: Generates a list of prefix shards (based on 2-byte prefixes)
//     and finds the min and max keys for each prefix group
//     - For FullValidation: Validates all keys in the database
//  5. Writes a "MIGRATION_COMPLETED" marker file with a timestamp to signal successful completion.
//
// This function returns an error if any part of the process fails, including directory checks,
// database operations, or validation mismatches.
func RunMigration(badgerDir string, pebbleDir string, cfg MigrationConfig) error {
	lg := log.With().
		Str("from-badger-dir", badgerDir).
		Str("to-pebble-dir", pebbleDir).
		Logger()

	// Step 1: Validate directories
	lg.Info().Msg("Step 1/6: Starting directory validation...")
	startTime := time.Now()
	if !cfg.ValidationOnly { // when ValidationOnly is true, database folders can be not empty
		if err := validateBadgerFolderExistPebbleFolderEmpty(badgerDir, pebbleDir); err != nil {
			return fmt.Errorf("directory validation failed: %w", err)
		}
	}
	lg.Info().Dur("duration", time.Since(startTime)).Msg("Step 1/6: Directory validation completed successfully")

	// Step 2: Open Badger and Pebble DBs
	lg.Info().Msg("Step 2/6: Opening BadgerDB and PebbleDB...")
	startTime = time.Now()
	badgerOptions := badger.DefaultOptions(badgerDir).
		WithLogger(util.NewLogger(log.Logger.With().Str("db", "badger").Logger()))
	badgerDB, err := badgerstorage.SafeOpen(badgerOptions)
	if err != nil {
		return fmt.Errorf("failed to open BadgerDB: %w", err)
	}
	defer badgerDB.Close()

	cache := pebble.NewCache(pebblestorage.DefaultPebbleCacheSize)
	defer cache.Unref()
	pebbleDBOpts := pebblestorage.DefaultPebbleOptions(log.Logger, cache, pebble.DefaultComparer)
	pebbleDBOpts.DisableAutomaticCompactions = true

	pebbleDB, err := pebble.Open(pebbleDir, pebbleDBOpts)
	if err != nil {
		return fmt.Errorf("failed to open PebbleDB: %w", err)
	}
	defer pebbleDB.Close()
	lg.Info().Dur("duration", time.Since(startTime)).Msg("Step 2/6: BadgerDB and PebbleDB opened successfully")

	if cfg.ValidationOnly {
		lg.Info().Str("mode", string(cfg.ValidationMode)).Msg("Step 6/6 Validation only mode enabled, skipping migration steps, Starting data validation...")
		startTime = time.Now()
		if err := validateData(badgerDB, pebbleDB, cfg); err != nil {
			return fmt.Errorf("data validation failed: %w", err)
		}
		lg.Info().Dur("duration", time.Since(startTime)).Msg("Step 6/6: Data validation completed successfully")

		return nil
	}
	// Step 3: Write MIGRATION_STARTED file
	lg.Info().Msg("Step 3/6: Writing migration start marker...")
	startTime = time.Now()
	startTimeStr := time.Now().Format(time.RFC3339)
	startMarkerPath := filepath.Join(pebbleDir, "MIGRATION_STARTED")
	startContent := fmt.Sprintf("migration started at %s\n", startTimeStr)
	if err := os.WriteFile(startMarkerPath, []byte(startContent), 0644); err != nil {
		return fmt.Errorf("failed to write MIGRATION_STARTED file: %w", err)
	}
	lg.Info().Dur("duration", time.Since(startTime)).Str("file", startMarkerPath).Msg("Step 3/6: Migration start marker written successfully")

	// Step 4: Migrate data
	lg.Info().Msg("Step 4/6: Starting data migration...")
	startTime = time.Now()
	cfg.PebbleDir = pebbleDir
	if err := CopyFromBadgerToPebbleSSTables(badgerDB, pebbleDB, cfg); err != nil {
		return fmt.Errorf("failed to migrate data from Badger to Pebble: %w", err)
	}
	lg.Info().Dur("duration", time.Since(startTime)).Msg("Step 4/6: Data migration completed successfully")

	// Step 5: Validate data
	lg.Info().Str("mode", string(cfg.ValidationMode)).Msg("Step 5/6: Starting data validation...")
	startTime = time.Now()
	if err := validateData(badgerDB, pebbleDB, cfg); err != nil {
		return fmt.Errorf("data validation failed: %w", err)
	}
	lg.Info().Dur("duration", time.Since(startTime)).Msg("Step 5/6: Data validation completed successfully")

	// Step 6: Write MIGRATION_COMPLETED file
	lg.Info().Msg("Step 6/6: Writing migration completion marker...")
	startTime = time.Now()
	endTime := time.Now().Format(time.RFC3339)
	completeMarkerPath := filepath.Join(pebbleDir, "MIGRATION_COMPLETED")
	completeContent := fmt.Sprintf("migration completed at %s\n", endTime)
	if err := os.WriteFile(completeMarkerPath, []byte(completeContent), 0644); err != nil {
		return fmt.Errorf("failed to write MIGRATION_COMPLETED file: %w", err)
	}
	lg.Info().Dur("duration", time.Since(startTime)).Str("file", completeMarkerPath).Msg("Step 6/6: Migration completion marker written successfully")

	return nil
}
