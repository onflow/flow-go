package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/consensus/hotstuff/persister"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

var GetLivenessCmd = &cobra.Command{
	Use:   "get-liveness",
	Short: "get hotstuff liveness data (current view, newest QC, last view TC)",
	Run:   runGetLivenessData,
}

func init() {
	rootCmd.AddCommand(GetLivenessCmd)
}

func runGetLivenessData(*cobra.Command, []string) {
	err := common.WithStorage(common.DBDirs{
		Datadir:   flagDatadir,
		Pebbledir: flagPebbleDir,
	}, func(db storage.DB) error {

		chainID := flow.ChainID(flagChain)
		reader, err := persister.NewReader(db, chainID)
		if err != nil {
			log.Fatal().Err(err).Msg("could not create reader from db")
		}

		log.Info().
			Str("chain", flagChain).
			Str("datadir", flagDatadir).
			Str("pebbledir", flagPebbleDir).
			Msg("getting hotstuff liveness data")

		livenessData, err := reader.GetLivenessData()
		if err != nil {
			log.Fatal().Err(err).Msg("could not get hotstuff liveness data")
		}

		log.Info().Msgf("successfully get hotstuff liveness data")
		common.PrettyPrint(livenessData)
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("could not get hotstuff liveness data")
	}

}
