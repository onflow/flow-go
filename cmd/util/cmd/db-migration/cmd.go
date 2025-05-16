package db

import (
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
)

var Cmd = &cobra.Command{
	Use:   "db-migration",
	Short: "copy badger db to pebble db",
	Run:   run,
}

func init() {
	Cmd.Flags().StringVar(&flagBadgerDBdir, "datadir", "", "BadgerDB Dir to copy data from")
	_ = Cmd.MarkFlagRequired("datadir")

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
}

func run(*cobra.Command, []string) {
	lg := log.With().
		Str("badger_db_dir", flagBadgerDBdir).
		Str("pebble_db_dir", flagPebbleDBdir).
		Str("batch_byte_size", units.HumanSize(float64(flagBatchByteSize))).
		Int("reader_count", flagReaderCount).
		Int("writer_count", flagWriterCount).
		Int("reader_shard_prefix_bytes", flagReaderShardPrefixBytes).
		Logger()

	lg.Info().Msgf("starting migration from badger db to pebble db")

	err := migration.RunMigration(flagBadgerDBdir, flagPebbleDBdir, migration.MigrationConfig{
		BatchByteSize:          flagBatchByteSize,
		ReaderWorkerCount:      flagReaderCount,
		WriterWorkerCount:      flagWriterCount,
		ReaderShardPrefixBytes: flagReaderShardPrefixBytes,
	})

	if err != nil {
		lg.Error().Err(err).Msg("migration failed")
		return
	}

	lg.Info().Msgf("migration completed")
}
