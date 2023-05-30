package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/inmem"
)

var SnapshotCmd = &cobra.Command{
	Use:   "snapshot",
	Short: "Read snapshot from protocol state",
	Run:   runSnapshot,
}

func init() {
	rootCmd.AddCommand(SnapshotCmd)

	SnapshotCmd.Flags().Uint64Var(&flagHeight, "height", 0,
		"Block height")

	SnapshotCmd.Flags().BoolVar(&flagFinal, "final", false,
		"get finalized block")

	SnapshotCmd.Flags().BoolVar(&flagSealed, "sealed", false,
		"get sealed block")
}

func runSnapshot(*cobra.Command, []string) {
	db := common.InitStorage(flagDatadir)
	defer db.Close()

	storages := common.InitStorages(db)
	state, err := common.InitProtocolState(db, storages)
	if err != nil {
		log.Fatal().Err(err).Msg("could not init protocol state")
	}

	var snapshot protocol.Snapshot

	if flagHeight > 0 {
		log.Info().Msgf("get snapshot by height: %v", flagHeight)
		snapshot = state.AtHeight(flagHeight)
	} else if flagFinal {
		log.Info().Msgf("get last finalized snapshot")
		snapshot = state.Final()
	} else if flagSealed {
		log.Info().Msgf("get last sealed snapshot")
		snapshot = state.Sealed()
	}

	head, err := snapshot.Head()
	if err != nil {
		log.Fatal().Err(err).Msg("fail to get block of snapshot")
	}

	log.Info().Msgf("creating snapshot for block height %v, id %v", head.Height, head.ID())

	serializable, err := inmem.FromSnapshot(snapshot)
	if err != nil {
		log.Fatal().Err(err).Msg("fail to serialize snapshot")
	}

	sealingSegment, err := serializable.SealingSegment()
	if err != nil {
		log.Fatal().Err(err).Msg("could not get sealing segment")
	}

	log.Info().Msgf("snapshot created, sealed height %v, id %v",
		sealingSegment.Sealed().Header.Height, sealingSegment.Sealed().Header.ID())

	log.Info().Msgf("highest finalized height %v, id %v",
		sealingSegment.Highest().Header.Height, sealingSegment.Highest().Header.ID())

	encoded := serializable.Encodable()
	common.PrettyPrint(encoded)
}
