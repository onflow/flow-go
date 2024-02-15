package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
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
	db := common.InitStorage(flagDatadir)
	defer db.Close()

	storages := common.InitStorages(db)
	state, err := common.InitProtocolState(db, storages)
	if err != nil {
		log.Fatal().Err(err).Msg("could not init protocol state")
	}

	rootBlock := state.Params().FinalizedRoot()

	reader := NewHotstuffReader(db, rootBlock.ChainID)

	log.Info().Msg("getting hotstuff safety data")

	livenessData, err := reader.GetSafetyData()
	if err != nil {
		log.Fatal().Err(err).Msg("could not get hotstuff safety data")
	}

	log.Info().Msgf("successfully get hotstuff safety data")
	common.PrettyPrint(livenessData)
}
