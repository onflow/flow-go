package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
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
	db := common.InitStorage(flagDatadir)
	defer db.Close()

	storages := common.InitStorages(db)
	state, err := common.InitProtocolState(db, storages)
	if err != nil {
		log.Fatal().Err(err).Msg("could not init protocol state")
	}

	rootBlock := state.Params().FinalizedRoot()
	if err != nil {
		log.Fatal().Err(err).Msgf("could not get root block")
	}

	reader := NewHotstuffReader(db, rootBlock.ChainID)

	log.Info().Msg("getting hotstuff liveness data")

	livenessData, err := reader.GetLivenessData()
	if err != nil {
		log.Fatal().Err(err).Msg("could not get hotstuff liveness data")
	}

	log.Info().Msgf("successfully get hotstuff liveness data")
	common.PrettyPrint(livenessData)
}
