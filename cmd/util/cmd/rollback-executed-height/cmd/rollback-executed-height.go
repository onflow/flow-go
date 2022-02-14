package cmd

import (
	"errors"
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

var (
	flagHeight  uint64
	flagDataDir string
)

var Cmd = &cobra.Command{
	Use:   "rollback-executed-height",
	Short: "Rollback the executed height",
	Run:   run,
}

func init() {

	Cmd.Flags().Uint64Var(&flagHeight, "height", 0,
		"the height of the block to update the highest executed height")
	_ = Cmd.MarkFlagRequired("from-height")

	Cmd.Flags().StringVar(&flagDataDir, "datadir", "",
		"directory that stores the protocol state")
	_ = Cmd.MarkFlagRequired("datadir")
}

func run(*cobra.Command, []string) {
	log.Info().
		Str("datadir", flagDataDir).
		Uint64("height", flagHeight).
		Msg("flags")

	db := common.InitStorage(flagDataDir)
	storages := common.InitStorages(db)
	state, err := common.InitProtocolState(db, storages)

	if err != nil {
		log.Fatal().Err(err).Msg("could not init states")
	}

	err = removeExecutionResultsFromHeight(
		state,
		storages.TransactionResults,
		storages.Commits,
		storages.ChunkDataPacks,
		storages.Results,
		flagHeight+1)

	if err != nil {
		log.Fatal().Err(err).Msgf("could not remove result from height %v", flagHeight)
	}

	header, err := state.AtHeight(flagHeight).Head()

	err = storages.Headers.RollbackExecutedBlock(header)
	if err != nil {
		log.Fatal().Err(err).Msgf("could not roll back executed block at height %v", flagHeight)
	}

	log.Info().Msgf("executed height rolled back to %v", flagHeight)

}

func removeExecutionResultsFromHeight(
	protoState protocol.State,
	transactionResults storage.TransactionResults,
	commits storage.Commits,
	chunkDataPacks storage.ChunkDataPacks,
	results storage.ExecutionResults,
	fromHeight uint64) error {
	log.Info().Msgf("removing results for blocks from height: %v", flagHeight)

	root, err := protoState.Params().Root()
	if err != nil {
		return fmt.Errorf("could not get root: %w", err)
	}

	if fromHeight <= root.Height {
		return fmt.Errorf("can only remove results for block above root block. fromHeight: %v, rootHeight: %v", fromHeight, root.Height)
	}

	final, err := protoState.Final().Head()
	if err != nil {
		return fmt.Errorf("could get not finalized height: %w", err)
	}

	if fromHeight > final.Height {
		return fmt.Errorf("could not remove results for unfinalized height: %v, finalized height: %v", fromHeight, final.Height)
	}

	count := 0
	total := int(final.Height-fromHeight) + 1

	// removing for finalized blocks
	for height := fromHeight; height <= final.Height; height++ {
		head, err := protoState.AtHeight(height).Head()
		if err != nil {
			return fmt.Errorf("could not get header at height: %w", err)
		}

		blockID := head.ID()

		err = removeForBlockID(commits, transactionResults, results, chunkDataPacks, blockID)
		if err != nil {
			return fmt.Errorf("could not remove result for finalized block: %v, %w", blockID, err)
		}

		count++
		log.Info().Msgf("result at height :%v has been removed. progress (%v/%v)", height, count, total)
	}

	// removing for pending blocks
	pendings, err := protoState.Final().Descendants()
	if err != nil {
		return fmt.Errorf("could not get pending block: %w", err)
	}

	count = 0
	total = len(pendings)

	for _, pending := range pendings {
		err = removeForBlockID(commits, transactionResults, results, chunkDataPacks, pending)
		if err != nil {
			return fmt.Errorf("could not remove result for pending block: %v, %w", pending, err)
		}

		count++
		log.Info().Msgf("result for pending block :%v has been removed. progress (%v/%v) ", pending, count, total)
	}

	return nil
}

func removeForBlockID(
	commits storage.Commits,
	transactionResults storage.TransactionResults,
	results storage.ExecutionResults,
	chunks storage.ChunkDataPacks,
	blockID flow.Identifier) error {
	result, err := results.ByBlockID(blockID)
	if errors.Is(err, storage.ErrNotFound) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("could not find result for block %v: %w", blockID, err)
	}

	for _, chunk := range result.Chunks {
		chunkID := chunk.ID()
		err := chunks.Remove(chunkID)
		if err != nil {
			return fmt.Errorf("could not remove chunk id %v for block id %v: %w", chunkID, blockID, err)
		}
	}

	err = commits.RemoveByBlockID(blockID)
	if err != nil {
		return fmt.Errorf("could not remove by block ID: %v, %w", blockID, err)
	}

	err = transactionResults.RemoveByBlockID(blockID)
	if err != nil {
		return fmt.Errorf("could not remove transaction results by BlockID: %v, %w", blockID, err)
	}

	return nil
}
