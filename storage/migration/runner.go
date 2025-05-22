package migration

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/dgraph-io/badger/v2"
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
	if err := validateBadgerFolderExistPebbleFolderEmpty(badgerDir, pebbleDir); err != nil {
		return fmt.Errorf("directory validation failed: %w", err)
	}

	// Step 2: Open Badger and Pebble DBs
	badgerDB, err := badger.Open(badger.DefaultOptions(badgerDir).WithLogger(nil))
	if err != nil {
		return fmt.Errorf("failed to open BadgerDB: %w", err)
	}
	defer badgerDB.Close()

	pebbleDB, err := pebble.Open(pebbleDir, &pebble.Options{
		EventListener: &pebble.EventListener{
			DiskSlow: func(info pebble.DiskSlowInfo) {
				log.Info().Msgf("Disk slow: %s", info.String())
			},
			FlushBegin: func(info pebble.FlushInfo) {
				log.Info().Msgf("Flush started: %s", info.String())
			},
			FlushEnd: func(info pebble.FlushInfo) {
				log.Info().Msgf("Flush ended: %s", info.String())
			},
			ManifestCreated: func(info pebble.ManifestCreateInfo) {
				log.Info().Msgf("Manifest created: %s", info.String())
			},
			ManifestDeleted: func(info pebble.ManifestDeleteInfo) {
				log.Info().Msgf("Manifest deleted: %s", info.String())
			},
			CompactionBegin: func(info pebble.CompactionInfo) {
				log.Info().Msgf("Compaction started: %s", info.String())
			},
			CompactionEnd: func(info pebble.CompactionInfo) {
				log.Info().Msgf("Compaction ended: %s", info.String())
			},
			TableCreated: func(info pebble.TableCreateInfo) {
				log.Info().Msgf("Table created: %s", info.String())
			},
			TableDeleted: func(info pebble.TableDeleteInfo) {
				log.Info().Msgf("Table deleted: %s", info.String())
			},
			TableIngested: func(info pebble.TableIngestInfo) {
				log.Info().Msgf("Table ingested: %s", info.String())
			},
			TableValidated: func(info pebble.TableValidatedInfo) {
				log.Info().Msgf("Table validated: %s", info.String())
			},
			WALCreated: func(info pebble.WALCreateInfo) {
				log.Info().Msgf("WAL created: %s", info.String())
			},
			WALDeleted: func(info pebble.WALDeleteInfo) {
				log.Info().Msgf("WAL deleted: %s", info.String())
			},
			WriteStallBegin: func(info pebble.WriteStallBeginInfo) {
			},
			WriteStallEnd: func() {
				log.Info().Msgf("Write stall ended")
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

	lg.Info().Msgf("Migration started. created mark file: %s", startMarkerPath)

	// Step 4: Migrate data
	if err := CopyFromBadgerToPebble(badgerDB, pebbleDB, cfg); err != nil {
		return fmt.Errorf("failed to migrate data from Badger to Pebble: %w", err)
	}

	validatingPrefixBytesCount := 2

	lg.Info().Msgf("Migration from BadgerDB to PebbleDB completed successfully. "+
		"Validating key consistency with %v prefix bytes...", validatingPrefixBytesCount)

	// Step 5: Validate data
	if err := validateMinMaxKeyConsistency(badgerDB, pebbleDB, validatingPrefixBytesCount); err != nil {
		return fmt.Errorf("data validation failed: %w", err)
	}

	log.Info().Msgf("Data validation between BadgerDB and PebbleDB completed successfully.")

	// Step 6: Write MIGRATION_COMPLETED file with timestamp
	endTime := time.Now().Format(time.RFC3339)
	completeMarkerPath := filepath.Join(pebbleDir, "MIGRATION_COMPLETED")
	completeContent := fmt.Sprintf("migration completed at %s\n", endTime)
	if err := os.WriteFile(completeMarkerPath, []byte(completeContent), 0644); err != nil {
		return fmt.Errorf("failed to write MIGRATION_COMPLETED file: %w", err)
	}

	lg.Info().Str("file", completeMarkerPath).Msgf("Migration marker file written successfully. start compaction...")

	// Step 7: Compact the PebbleDB to optimize storage
	if err := pebbleDB.Compact(nil, nil, true); err != nil {
		return fmt.Errorf("failed to compact PebbleDB: %w", err)
	}

	lg.Info().Msgf("PebbleDB compaction completed successfully.")

	return nil
}
