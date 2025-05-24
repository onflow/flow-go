package migration

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/dgraph-io/badger/v2"
	"github.com/onflow/flow-go/storage/util"
	"github.com/rs/zerolog/log"
)

var DefaultMigrationConfig = MigrationConfig{
	BatchByteSize:          32_000_000, // 32 MB
	ReaderWorkerCount:      2,
	WriterWorkerCount:      2,
	ReaderShardPrefixBytes: 2, // better to keep it as 2.
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
//     - Generating a list of prefix shards (based on 2-byte prefixes)
//     - Finding the min and max keys for each prefix group
//     - Comparing the values of those keys between Badger and Pebble
//  5. Writes a "MIGRATION_COMPLETED" marker file with a timestamp to signal successful completion.
//
// This function returns an error if any part of the process fails, including directory checks,
// database operations, or validation mismatches.
func RunMigration(badgerDir string, pebbleDir string, cfg MigrationConfig) error {
	// Step 1: Validate directories
	log.Info().Msgf("Step 1/7 Validating directories...")
	if err := validateBadgerFolderExistPebbleFolderEmpty(badgerDir, pebbleDir); err != nil {
		return fmt.Errorf("directory validation failed: %w", err)
	}

	// Step 2: Open Badger and Pebble DBs
	log.Info().Msgf("Step 2/7 Opening BadgerDB and PebbleDB...")
	badgerOptions := badger.DefaultOptions(badgerDir).
		WithLogger(util.NewLogger(log.Logger.With().Str("db", "badger").Logger())).
		WithReadOnly(true).
		WithNumCompactors(0).
		WithNumLevelZeroTables(0)
	badgerDB, err := badger.Open(badgerOptions)
	if err != nil {
		return fmt.Errorf("failed to open BadgerDB: %w", err)
	}
	defer badgerDB.Close()

	pebbleDB, err := pebble.Open(pebbleDir, &pebble.Options{
		DisableAutomaticCompactions: true, // compaction will be done at the end
		EventListener: &pebble.EventListener{
			CompactionEnd: func(info pebble.CompactionInfo) {
				log.Info().Msgf("Compaction ended: %s", info.String())
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to open PebbleDB: %w", err)
	}
	defer pebbleDB.Close()

	// Step 3: Write MIGRATION_STARTED file with timestamp
	startTime := time.Now().Format(time.RFC3339)
	startMarkerPath := filepath.Join(pebbleDir, "MIGRATION_STARTED")
	startContent := fmt.Sprintf("migration started at %s\n", startTime)
	if err := os.WriteFile(startMarkerPath, []byte(startContent), 0644); err != nil {
		return fmt.Errorf("failed to write MIGRATION_STARTED file: %w", err)
	}

	lg := log.With().
		Str("from-badger-dir", badgerDir).
		Str("to-pebble-dir", pebbleDir).
		Logger()

	lg.Info().Msgf("Step 3/7 Migration started. created mark file: %s", startMarkerPath)

	// Step 4: Migrate data
	if err := CopyFromBadgerToPebble(badgerDB, pebbleDB, cfg); err != nil {
		return fmt.Errorf("failed to migrate data from Badger to Pebble: %w", err)
	}

	validatingPrefixBytesCount := 2

	lg.Info().Msgf("Step 4/7 Migration from BadgerDB to PebbleDB completed successfully. "+
		"Validating key consistency with %v prefix bytes...", validatingPrefixBytesCount)

	// Step 5: Validate data
	if err := validateMinMaxKeyConsistency(badgerDB, pebbleDB, validatingPrefixBytesCount); err != nil {
		return fmt.Errorf("data validation failed: %w", err)
	}

	log.Info().
		Msgf("Step 5/7 Data validation between BadgerDB and PebbleDB completed successfully.")

	// Step 6: Write MIGRATION_COMPLETED file with timestamp
	endTime := time.Now().Format(time.RFC3339)
	completeMarkerPath := filepath.Join(pebbleDir, "MIGRATION_COMPLETED")
	completeContent := fmt.Sprintf("migration completed at %s\n", endTime)
	if err := os.WriteFile(completeMarkerPath, []byte(completeContent), 0644); err != nil {
		return fmt.Errorf("failed to write MIGRATION_COMPLETED file: %w", err)
	}

	lg.Info().Str("file", completeMarkerPath).
		Msgf("Step 6/7 Migration marker file written successfully. start compaction...")

	start, end := []byte{0x00}, []byte{0xff}
	// Step 7: Compact the PebbleDB to optimize storage
	if err := pebbleDB.Compact(start, end, true); err != nil {
		return fmt.Errorf("failed to compact PebbleDB: %w", err)
	}

	lg.Info().Msgf("Step 7/7 PebbleDB compaction completed successfully.")

	return nil
}
