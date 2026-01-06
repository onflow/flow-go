package cmd

import (
	"errors"
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	commonFuncs "github.com/onflow/flow-go/cmd/util/common"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	badgerstate "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/storage"
)

var flagCheckpointDir string
var flagCheckpointScanStep uint
var flagCheckpointScanEndHeight int64

var SnapshotCmd = &cobra.Command{
	Use:   "snapshot",
	Short: "Read snapshot from protocol state",
	RunE:  runSnapshotE,
}

func init() {
	rootCmd.AddCommand(SnapshotCmd)

	SnapshotCmd.Flags().Uint64Var(&flagHeight, "height", 0,
		"Block height")

	SnapshotCmd.Flags().StringVar(&flagBlockID, "block-id", "",
		"Block ID (hex-encoded, 64 characters)")

	SnapshotCmd.Flags().BoolVar(&flagFinal, "final", false,
		"get finalized block")

	SnapshotCmd.Flags().BoolVar(&flagSealed, "sealed", false,
		"get sealed block")

	SnapshotCmd.Flags().BoolVar(&flagExecuted, "executed", false,
		"get last executed and sealed block (execution node only)")

	SnapshotCmd.Flags().StringVar(&flagCheckpointDir, "checkpoint-dir", "",
		"(execution node only) get snapshot from the latest checkpoint file in the given checkpoint directory")

	SnapshotCmd.Flags().UintVar(&flagCheckpointScanStep, "checkpoint-scan-step", 0,
		"(execution node only) scan step for finding sealed height by checkpoint (use with --checkpoint-dir flag)")

	SnapshotCmd.Flags().Int64Var(&flagCheckpointScanEndHeight, "checkpoint-scan-end-height", -1,
		"(execution node only) scan end height for finding sealed height by checkpoint (use with --checkpoint-dir flag)")
}

func runSnapshotE(*cobra.Command, []string) error {
	lockManager := storage.MakeSingletonLockManager()
	return common.WithStorage(flagDatadir, func(db storage.DB) error {
		chainID, err := badgerstate.GetChainID(db)
		if err != nil {
			return err
		}
		storages := common.InitStorages(db, chainID)
		state, err := common.OpenProtocolState(lockManager, db, storages)
		if err != nil {
			return fmt.Errorf("could not init protocol state")
		}

		var snapshot protocol.Snapshot

		if flagHeight > 0 {
			log.Info().Msgf("get snapshot by height: %v", flagHeight)
			snapshot = state.AtHeight(flagHeight)
		} else if flagBlockID != "" {
			log.Info().Msgf("get snapshot by block ID: %v", flagBlockID)
			blockID := flow.MustHexStringToIdentifier(flagBlockID)
			snapshot = state.AtBlockID(blockID)
		} else if flagFinal {
			log.Info().Msgf("get last finalized snapshot")
			snapshot = state.Final()
		} else if flagSealed {
			log.Info().Msgf("get last sealed snapshot")
			snapshot = state.Sealed()
		} else if flagExecuted {
			log.Info().Msgf("get last executed and sealed snapshot")
			sealedSnapshot := state.Sealed()
			sealedHead, err := sealedSnapshot.Head()
			if err != nil {
				return fmt.Errorf("could not get sealed block: %w", err)
			}

			root := state.Params().SealedRoot()

			// find the last executed and sealed block
			var executedBlockID flow.Identifier
			found := false
			for h := sealedHead.Height; h >= root.Height; h-- {
				blockHeader, err := state.AtHeight(h).Head()
				if err != nil {
					return fmt.Errorf("could not get block header by height: %v: %w", h, err)
				}

				// block is executed if a commitment to the block's output state has been persisted
				_, err = storages.Commits.ByBlockID(blockHeader.ID())
				if err == nil {
					executedBlockID = blockHeader.ID()
					found = true
					break
				}

				// state commitment not existing means the block hasn't been executed yet,
				// hence `storage.ErrNotFound` is the only error return we expect here
				if !errors.Is(err, storage.ErrNotFound) {
					return fmt.Errorf("could not check block executed or not: %v: %w", h, err)
				}
			}

			if !found {
				return fmt.Errorf("State corrupted: traversed sealed fork backwards down to root height %d, could not find executed block. This should never happen, since the state should be known at least for the root block!", root.Height)
			}

			snapshot = state.AtBlockID(executedBlockID)
		} else if flagCheckpointDir != "" {
			log.Info().Msgf("get snapshot for latest checkpoint in directory %v (step: %v, endHeight: %v)",
				flagCheckpointDir, flagCheckpointScanStep, flagCheckpointScanEndHeight)
			var protocolSnapshot protocol.Snapshot
			var sealedHeight uint64
			var sealedCommit flow.StateCommitment
			var checkpointFile string
			if flagCheckpointScanEndHeight < 0 {
				// using default end height which is the last sealed height
				protocolSnapshot, sealedHeight, sealedCommit, checkpointFile, err = commonFuncs.GenerateProtocolSnapshotForCheckpoint(
					log.Logger, state, storages.Headers, storages.Seals, flagCheckpointDir, flagCheckpointScanStep)
			} else {
				// using customized end height
				protocolSnapshot, sealedHeight, sealedCommit, checkpointFile, err = commonFuncs.GenerateProtocolSnapshotForCheckpointWithHeights(
					log.Logger, state, storages.Headers, storages.Seals, flagCheckpointDir, flagCheckpointScanStep, uint64(flagCheckpointScanEndHeight))
			}
			if err != nil {
				return fmt.Errorf("could not generate protocol snapshot for checkpoint in dir: %v: %w", flagCheckpointDir, err)
			}

			snapshot = protocolSnapshot
			log.Info().Msgf("snapshot found for checkpoint file %v, sealed height %v, commit %x", checkpointFile, sealedHeight, sealedCommit)
		}

		head, err := snapshot.Head()
		if err != nil {
			return fmt.Errorf("fail to get block of snapshot: %w", err)
		}

		log.Info().Msgf("creating snapshot for block height %v, id %v", head.Height, head.ID())

		serializable, err := inmem.FromSnapshot(snapshot)
		if err != nil {
			return fmt.Errorf("fail to serialize snapshot: %w", err)
		}

		sealingSegment, err := serializable.SealingSegment()
		if err != nil {
			return fmt.Errorf("could not get sealing segment: %w", err)
		}

		log.Info().Msgf("snapshot created, sealed height %v, id %v",
			sealingSegment.Sealed().Height, sealingSegment.Sealed().ID())

		log.Info().Msgf("highest finalized height %v, id %v",
			sealingSegment.Highest().Height, sealingSegment.Highest().ID())

		encoded := serializable.Encodable()
		common.PrettyPrint(encoded)
		return nil
	})
}
