package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/persister"
)

var (
	flagView uint64
)

var SetterCmd = &cobra.Command{
	Use:   "set-unsafe",
	Short: "set hotstuff data (unsafe and may cause data corruption if used incorrectly)",
	Run:   runSetUnsafe,
}

func init() {
	rootCmd.AddCommand(SetterCmd)
	SetterCmd.Flags().Uint64Var(&flagView, "view", 0, "hotstuff view")
}

// set:
//
//	LastViewTC=nil
//	CurrentView=flagView
//	HighestAcknowledgedView=flagView-1
//	LastTimeout=nil
//
// CAUTION: does not implement safety checks!!!
// This function if improperly used can cause data corruption. Be careful!!!
func runSetUnsafe(*cobra.Command, []string) {
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
	reader := NewHotstuffReader(db, rootBlock.ChainID)

	initialSafetyData, err := reader.GetSafetyData()
	if err != nil {
		log.Fatal().Err(err).Msgf("could not get hotstuff safety data")
	}
	initialLivenessData, err := reader.GetLivenessData()
	if err != nil {
		log.Fatal().Err(err).Msgf("could not get hotstuff liveness data")
	}

	modifiedSafetyData := &hotstuff.SafetyData{
		LockedOneChainView:      initialSafetyData.LockedOneChainView,
		HighestAcknowledgedView: flagView - 1,
		LastTimeout:             nil,
	}
	modifiedLivenessData := &hotstuff.LivenessData{
		CurrentView: flagView,
		NewestQC:    initialLivenessData.NewestQC,
		LastViewTC:  nil,
	}

	log.Info().
		Uint64("safety.LockedOneChainView", initialSafetyData.LockedOneChainView).
		Uint64("safety.HighestAcknowledgedView", initialSafetyData.HighestAcknowledgedView).
		Uint64("liveness.CurrentView", initialLivenessData.CurrentView).
		Uint64("liveness.NewestQC.View", initialLivenessData.NewestQC.View).
		Msg("initial state hotstuff data")

	log.Info().
		Uint64("safety.LockedOneChainView", modifiedSafetyData.LockedOneChainView).
		Uint64("safety.HighestAcknowledgedView", modifiedSafetyData.HighestAcknowledgedView).
		Uint64("liveness.CurrentView", modifiedLivenessData.CurrentView).
		Uint64("liveness.NewestQC.View", modifiedLivenessData.NewestQC.View).
		Msg("setting new hotstuff data")

	err = pers.PutSafetyData(modifiedSafetyData)
	if err != nil {
		log.Fatal().Err(err).Msgf("could not set hotstuff safety data")
	}
	err = pers.PutLivenessData(modifiedLivenessData)
	if err != nil {
		log.Fatal().Err(err).Msgf("could not set hotstuff liveness data")
	}

	log.Info().Msg("successfully set hotstuff data")
}
