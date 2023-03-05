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
	flagStorageDir string
	flagValidate   bool
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

	Cmd.Flags().StringVar(&flagStorageDir, "storagedir", "/var/flow/data/payloads",
		"storage directory to store payloads")
	_ = Cmd.MarkFlagRequired("storagedir")

	Cmd.Flags().BoolVar(&flagValidate, "validate", false,
		"validate the imported payloads")
}

func run(*cobra.Command, []string) {
	dir, file := filepath.Split(flagCheckpoint)
	log.Info().Msgf("importing payloads from checkpoint file: %v, to storage dir: %v",
		flagCheckpoint,
		flagStorageDir)

	payloadStorage := storage.CreatePayloadStorageWithDir(flagStorageDir)

	if !flagValidate {
		err := importer.ImportLeafNodesFromCheckpoint(dir, file, &log.Logger, payloadStorage)
		if err != nil {
			log.Fatal().Err(err).Msg("could not import leaf nodes from checkpoint file")
		}
	}

	log.Info().Msg("all payloads from checkpoint file has been successfully imported")

	if flagValidate {
		log.Info().Msgf("validating imported payloads from checkpoint file")
		err := importer.ValidateFromCheckpoint(dir, file, &log.Logger, payloadStorage)
		if err != nil {
			log.Fatal().Err(err).Msgf("imported checkpoint payload data is invalid")
		}
	}

	log.Info().Msgf("finish importing all payloads")
}
