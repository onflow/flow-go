package cmd

import (
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/state/protocol"
)

var SnapshotCmd = &cobra.Command{
	Use:   "snapshot",
	Short: "Read snapshot from protocol state",
	Run:   run,
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
		log.Info().Msgf("get block by height: %v", flagHeight)
		snapshot = state.AtHeight(flagHeight)
	}

	if flagFinal {
		log.Info().Msgf("get last finalized block")
		snapshot = state.Final()
	}

	if flagSealed {
		log.Info().Msgf("get last sealed block")
		snapshot = state.Sealed()
	}

	data, err := convert.SnapshotToBytes(snapshot)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to convert snapshot to bytes: %v")
	}

	fmt.Println(data)
}
