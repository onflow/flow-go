package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/consensus/hotstuff/persister"
)

var GetterCmd = &cobra.Command{
	Use:   "get",
	Short: "get hotstuff view",
	Run:   runGet,
}

func init() {
	rootCmd.AddCommand(GetterCmd)
}

func runGet(*cobra.Command, []string) {
	db := common.InitStorage(flagDatadir)
	defer db.Close()

	storages := common.InitStorages(db)
	state, err := common.InitProtocolState(db, storages)
	if err != nil {
		log.Fatal().Err(err).Msg("could not init protocol state")
	}

	rootBlock, err := state.Params().Root()
	if err != nil {
		log.Fatal().Err(err).Msgf("could not get root block")
	}

	pers := persister.New(db, rootBlock.ChainID)

	reader := NewReader(pers)

	log.Info().Msg("getting hotstuff view")

	view, err := reader.GetHotstuffView()
	if err != nil {
		log.Fatal().Err(err).Msg("could not get hotstuff view")
	}

	log.Info().Msgf("successfully get hotstuff view: %v", view)
}
