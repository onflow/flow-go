package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	commonFuncs "github.com/onflow/flow-go/cmd/util/common"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/inmem"
)

var flagCheckpointDir string
var flagCheckpointScanStep uint
var flagCheckpointScanEndHeight int64

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

	SnapshotCmd.Flags().StringVar(&flagCheckpointDir, "checkpoint-dir", "",
		"(execution node only) get snapshot from the latest checkpoint file in the given checkpoint directory")

	SnapshotCmd.Flags().UintVar(&flagCheckpointScanStep, "checkpoint-scan-step", 0,
		"(execution node only) scan step for finding sealed height by checkpoint (use with --checkpoint-dir flag)")

	SnapshotCmd.Flags().Int64Var(&flagCheckpointScanEndHeight, "checkpoint-scan-end-height", -1,
		"(execution node only) scan end height for finding sealed height by checkpoint (use with --checkpoint-dir flag)")
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
	} else if flagCheckpointDir != "" {
		log.Info().Msgf("get snapshot for latest checkpoint in directory %v (step: %v, endHeight: %v)",
			flagCheckpointDir, flagCheckpointScanStep, flagCheckpointScanEndHeight)
		var protocolSnapshot protocol.Snapshot
		var sealedHeight uint64
		var sealedCommit flow.StateCommitment
		if flagCheckpointScanEndHeight < 0 {
			// using default end height which is the last sealed height
			protocolSnapshot, sealedHeight, sealedCommit, err = commonFuncs.GenerateProtocolSnapshotForCheckpoint(
				log.Logger, state, storages.Headers, storages.Seals, flagCheckpointDir, flagCheckpointScanStep)
		} else {
			// using customized end height
			protocolSnapshot, sealedHeight, sealedCommit, err = commonFuncs.GenerateProtocolSnapshotForCheckpointWithHeights(
				log.Logger, state, storages.Headers, storages.Seals, flagCheckpointDir, flagCheckpointScanStep, uint64(flagCheckpointScanEndHeight))
		}

		if err != nil {
			log.Fatal().Err(err).Msgf("could not generate protocol snapshot for checkpoint in dir: %v", flagCheckpointDir)
		}

		snapshot = protocolSnapshot
		log.Info().Msgf("snapshot found, sealed height %v, commit %x", sealedHeight, sealedCommit)
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
