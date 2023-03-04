package migrate

import (
	"path/filepath"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/ledger/storage"
	"github.com/onflow/flow-go/ledger/storage/importer"
)

var (
	flagCheckpoint string
)

var Cmd = &cobra.Command{
	Use:   "migrate-checkpoint-payload",
	Short: "read the checkpoint file and store all payloads to storage",
	Run:   run,
}

func init() {
	Cmd.Flags().StringVar(&flagCheckpoint, "checkpoint", "",
		"checkpoint file to read")
	_ = Cmd.MarkFlagRequired("checkpoint")
}

func run(*cobra.Command, []string) {
	dir, file := filepath.Split(flagCheckpoint)
	log.Info().Msgf("importing payloads from checkpoint file: %v", flagCheckpoint)

	payloadStorage := storage.CreatePayloadStorage()

	err := importer.ImportLeafNodesFromCheckpoint(dir, file, &log.Logger, payloadStorage)
	if err != nil {
		log.Fatal().Err(err).Msg("could not import leaf nodes from checkpoint file")
	}

	log.Info().Msg("all payloads from checkpoint file has been successfully imported")
}
