package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/consensus/hotstuff/persister"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

var GetSafetyCmd = &cobra.Command{
	Use:   "get-safety",
	Short: "get hotstuff safety data (locked view, highest acked view, last timeout)",
	Run:   runGetSafetyData,
}

func init() {
	rootCmd.AddCommand(GetSafetyCmd)
}

func runGetSafetyData(*cobra.Command, []string) {
	err := common.WithStorage(flagDatadir, func(db storage.DB) error {
		lockManager := storage.NewTestingLockManager()

		chainID := flow.ChainID(flagChain)
		reader, err := persister.NewReader(db, chainID, lockManager)
		if err != nil {
			log.Fatal().Err(err).Msg("could not create reader from db")
		}

		log.Info().
			Str("chain", flagChain).
			Msg("getting hotstuff safety data")

		livenessData, err := reader.GetSafetyData()
		if err != nil {
			log.Fatal().Err(err).Msg("could not get hotstuff safety data")
		}

		log.Info().Msgf("successfully get hotstuff safety data")
		common.PrettyPrint(livenessData)
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("could not get hotstuff safety data")
	}
}
