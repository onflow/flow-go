package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/consensus/hotstuff/persister"
)

var (
	flagView uint64
)

// TODO: will be removed after active pacemaker is implemented
var SetterCmd = &cobra.Command{
	Use:   "set",
	Short: "set hotstuff view",
	Run:   runSet,
}

func init() {
	rootCmd.AddCommand(SetterCmd)
	SetterCmd.Flags().Uint64Var(&flagView, "view", 0, "hotstuff view")
}

func runSet(*cobra.Command, []string) {
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

	currentView, err := reader.GetHotstuffView()
	if err != nil {
		log.Fatal().Err(err).Msgf("could not get current view")
	}

	log.Info().Msgf("current view: %v, setting hotstuff view to %v", currentView, flagView)

	err = reader.SetHotstuffView(flagView)
	if err != nil {
		log.Fatal().Err(err).Msgf("could not set hotstuff view to %v", flagView)
	}

	log.Info().Msgf("successfully set hotstuff view to %v", flagView)
}
