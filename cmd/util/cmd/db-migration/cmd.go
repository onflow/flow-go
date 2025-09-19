package db

import (
	"fmt"
	"time"

	"github.com/docker/go-units"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/storage/migration"
)

var (
	flagBadgerDBdir            string
	flagPebbleDBdir            string
	flagBatchByteSize          int
	flagReaderCount            int
	flagWriterCount            int
	flagReaderShardPrefixBytes int
	flagValidationMode         string
	flagValidationOnly         bool
)

var Cmd = &cobra.Command{
	Use:   "db-migration",
	Short: "copy badger db to pebble db",
	RunE:  run,
}

func init() {
	Cmd.Flags().StringVar(&flagBadgerDBdir, "badgerdir", "", "BadgerDB Dir to copy data from")
	_ = Cmd.MarkFlagRequired("badgerdir")

	Cmd.Flags().StringVar(&flagPebbleDBdir, "pebbledir", "", "PebbleDB Dir to copy data to")
	_ = Cmd.MarkFlagRequired("pebbledir")

	Cmd.Flags().IntVar(&flagBatchByteSize, "batch_byte_size", migration.DefaultMigrationConfig.BatchByteSize,
		"the batch size in bytes to use for migration (32MB by default)")

	Cmd.Flags().IntVar(&flagReaderCount, "reader_count", migration.DefaultMigrationConfig.ReaderWorkerCount,
		"the number of reader workers to use for migration")

	Cmd.Flags().IntVar(&flagWriterCount, "writer_count", migration.DefaultMigrationConfig.WriterWorkerCount,
		"the number of writer workers to use for migration")

	Cmd.Flags().IntVar(&flagReaderShardPrefixBytes, "reader_shard_prefix_bytes", migration.DefaultMigrationConfig.ReaderShardPrefixBytes,
		"the number of prefix bytes used to assign iterator workload")

	Cmd.Flags().StringVar(&flagValidationMode, "validation_mode", string(migration.DefaultMigrationConfig.ValidationMode),
		"the validation mode to use for migration (partial or full, default is partial)")

	Cmd.Flags().BoolVar(&flagValidationOnly, "validation_only", false,
		"if set, only validate the data in the badger db without copying it to pebble db. "+
			"Note: this will not copy any data to pebble db, and will not create any pebble db files.")
}

func run(*cobra.Command, []string) error {
	lg := log.With().
		Str("badger_db_dir", flagBadgerDBdir).
		Str("pebble_db_dir", flagPebbleDBdir).
		Str("batch_byte_size", units.HumanSize(float64(flagBatchByteSize))).
		Int("reader_count", flagReaderCount).
		Int("writer_count", flagWriterCount).
		Int("reader_shard_prefix_bytes", flagReaderShardPrefixBytes).
		Str("validation_mode", flagValidationMode).
		Bool("validation_only", flagValidationOnly).
		Logger()

	validationMode, err := migration.ParseValidationModeValid(flagValidationMode)
	if err != nil {
		return fmt.Errorf("invalid validation mode: %w", err)
	}

	lg.Info().Msgf("starting migration from badger db to pebble db")
	start := time.Now()
	err = migration.RunMigrationAndCompaction(flagBadgerDBdir, flagPebbleDBdir, migration.MigrationConfig{
		BatchByteSize:          flagBatchByteSize,
		ReaderWorkerCount:      flagReaderCount,
		WriterWorkerCount:      flagWriterCount,
		ReaderShardPrefixBytes: flagReaderShardPrefixBytes,
		ValidationMode:         validationMode,
		ValidationOnly:         flagValidationOnly,
	})

	if err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	lg.Info().Msgf("migration completed successfully in %s", time.Since(start).String())

	return nil
}
