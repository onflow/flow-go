package truncate_database

import (
	"github.com/rs/zerolog/log"

	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
)

var (
	flagDatadir          string
	flagChunkDataPackDir string
)

var Cmd = &cobra.Command{
	Use:   "truncate-database",
	Short: "Truncates protocol state database (Possible data loss!)",
	Run:   run,
}

func init() {

	Cmd.Flags().StringVar(&flagDatadir, "datadir", "",
		"directory that stores the protocol state")
	_ = Cmd.MarkFlagRequired("datadir")

	Cmd.Flags().StringVar(&flagChunkDataPackDir, "chunk-data-pack-dir", "",
		"directory that stores the chunk data pack")
}

func run(*cobra.Command, []string) {

	log.Info().Msg("Opening protocol database with truncate")

	db := common.InitStorageWithTruncate(flagDatadir, true)
	defer db.Close()

	log.Info().Msg("ProtocolDB Truncated")

	if flagChunkDataPackDir != "" {
		chunkdb := common.InitStorageWithTruncate(flagChunkDataPackDir, true)
		defer chunkdb.Close()

		log.Info().Msg("Chunk Data Pack database Truncated")
	}
}
