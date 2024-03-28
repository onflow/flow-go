package common

import (
	"fmt"
	"path/filepath"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

func FindHeightByCheckpoints(
	logger zerolog.Logger,
	headers storage.Headers,
	seals storage.Seals,
	checkpointFilePath string,
	blocksToSkip uint,
	startHeight uint64,
	endHeight uint64,
) (uint64, flow.StateCommitment, error) {

	// find all trie root hashes in the checkpoint file
	dir, fileName := filepath.Split(checkpointFilePath)
	hashes, err := wal.ReadTriesRootHash(logger, dir, fileName)
	if err != nil {
		return 0, flow.DummyStateCommitment,
			fmt.Errorf("could not read trie root hashes from checkpoint file %v: %w",
				checkpointFilePath, err)
	}

	// convert all trie root hashes to state commitments
	commitments := hashesToCommits(hashes)

	// find a finalized block that produces one of the state commitment
	// in the given list of state commitments
	height, commit, err := findSealedHeightForCommits(
		headers,
		seals,
		commitments,
		blocksToSkip,
		startHeight,
		endHeight,
	)
	if err != nil {
		return 0, flow.DummyStateCommitment, fmt.Errorf("could not find sealed height by checkpoints: %w", err)
	}

	return height, commit, nil
}

func findSealedHeightForCommits(
	headers storage.Headers,
	seals storage.Seals,
	stateCommitments []flow.StateCommitment,
	blocksToSkip uint,
	startHeight uint64,
	endHeight uint64,
) (uint64, flow.StateCommitment, error) {
	commitMap := make(map[flow.StateCommitment]struct{}, len(stateCommitments))
	for _, commit := range stateCommitments {
		commitMap[commit] = struct{}{}
	}

	// iterate backwards from the end height to the start height
	// to find the block that produces a state commitment in the given list
	// end height must be a sealed block
	step := blocksToSkip + 1
	for height := endHeight; height >= startHeight; height -= uint64(step) {
		blockID, err := headers.BlockIDByHeight(height)
		if err != nil {
			return 0, flow.DummyStateCommitment, fmt.Errorf("could not find block by height %v: %w", height, err)
		}

		// since height is a sealed block height, then we must be able to find the seal for this block
		seal, err := seals.FinalizedSealForBlock(blockID)
		if err != nil {
			return 0, flow.DummyStateCommitment, fmt.Errorf("could not find seal for block %v at height %v: %w", blockID, height, err)
		}

		commit := seal.FinalState

		_, ok := commitMap[commit]
		if ok {
			log.Info().Msgf("successfully found block %v at height %v for commit %v",
				blockID, height, commit)
			return height, commit, nil
		}

		if height < uint64(blocksToSkip) {
			break
		}
	}

	return 0, flow.DummyStateCommitment, fmt.Errorf("could not find commit within height range [%v,%v]", startHeight, endHeight)
}

func GenerateProtocolSnapshotForCheckpoint(
	logger zerolog.Logger,
	state protocol.State,
	headers storage.Headers,
	seals storage.Seals,
	checkpointDir string,
	blocksToSkip uint,
) (protocol.Snapshot, uint64, flow.StateCommitment, error) {
	// skip X blocks (i.e. 10) each time to find the block that produces the state commitment in the checkpoint file
	// since a checkpoint file contains 500 tries, this allows us to find the block more efficiently
	sealed, err := state.Sealed().Head()
	if err != nil {
		return nil, 0, flow.DummyStateCommitment, err
	}
	endHeight := sealed.Height

	return GenerateProtocolSnapshotForCheckpointWithHeights(logger, state, headers, seals,
		checkpointDir,
		blocksToSkip,
		endHeight,
	)
}

func findLatestCheckpointFilePath(checkpointDir string) (string, error) {
	_, last, err := wal.ListCheckpoints(checkpointDir)
	if err != nil {
		return "", fmt.Errorf("could not list checkpoints in directory %v: %w", checkpointDir, err)
	}

	fileName := wal.NumberToFilename(last)
	if last < 0 {
		fileName = "root.checkpoint"
	}

	checkpointFilePath := filepath.Join(checkpointDir, fileName)
	return checkpointFilePath, nil
}

func GenerateProtocolSnapshotForCheckpointWithHeights(
	logger zerolog.Logger,
	state protocol.State,
	headers storage.Headers,
	seals storage.Seals,
	checkpointDir string,
	blocksToSkip uint,
	endHeight uint64,
) (protocol.Snapshot, uint64, flow.StateCommitment, error) {
	startHeight := uint64(0)
	// preventing startHeight from being negative
	if endHeight > uint64(blocksToSkip+1)*10000 {
		startHeight = endHeight - uint64(blocksToSkip+1)*10000
	}

	checkpointFilePath, err := findLatestCheckpointFilePath(checkpointDir)
	if err != nil {
		return nil, 0, flow.DummyStateCommitment, fmt.Errorf("could not find latest checkpoint file in directory %v: %w", checkpointDir, err)
	}

	log.Info().
		Uint64("start_height", startHeight).
		Uint64("end_height", endHeight).
		Uint("blocksToSkip", blocksToSkip).
		Msgf("generating protocol snapshot for checkpoint file %v", checkpointFilePath)
	// find the height of the finalized block that produces the state commitment contained in the checkpoint file
	sealedHeight, commit, err := FindHeightByCheckpoints(logger, headers, seals, checkpointFilePath, blocksToSkip, startHeight, endHeight)
	if err != nil {
		return nil, 0, flow.DummyStateCommitment, fmt.Errorf("could not find sealed height in range [%v:%v] (blocksToSkip: %v) by checkpoints: %w",
			startHeight, endHeight, blocksToSkip,
			err)
	}

	// find which finalized height seals the block with the given sealed height
	finalizedHeight, err := findFinalizedHeightBySealedHeight(state, sealedHeight)
	if err != nil {
		return nil, 0, flow.DummyStateCommitment, fmt.Errorf("could not find finalized height for sealed height %v: %w", sealedHeight, err)
	}

	return state.AtHeight(finalizedHeight), sealedHeight, commit, nil
}

// hashesToCommits converts a list of ledger.RootHash to a list of flow.StateCommitment
func hashesToCommits(hashes []ledger.RootHash) []flow.StateCommitment {
	commits := make([]flow.StateCommitment, len(hashes))
	for i, h := range hashes {
		commits[i] = flow.StateCommitment(h)
	}
	return commits
}

func findFinalizedHeightBySealedHeight(state protocol.State, sealedHeight uint64) (uint64, error) {
	sealed, err := state.AtHeight(sealedHeight).Head()
	if err != nil {
		return 0, err
	}
	sealedID := sealed.ID()

	lastFinalized, err := state.Final().Head()
	if err != nil {
		return 0, fmt.Errorf("could not get last finalized block: %w", err)
	}
	lastFinalizedHeight := lastFinalized.Height

	// the finalized height that seals the given sealed height must be above the sealed height
	// so if we iterate through each height, we should eventually find the finalized height
	for height := sealedHeight; height <= lastFinalizedHeight; height++ {
		_, seal, err := state.AtHeight(height).SealedResult()
		if err != nil {
			return 0, fmt.Errorf("could not get sealed result at height %v: %w", height, err)
		}

		// if the block contains a seal that seals the block with the given sealed height
		// then it's the finalized height that we are looking for
		if seal.BlockID == sealedID {
			return height, nil
		}
	}

	return 0, fmt.Errorf("could not find finalized height for sealed height %v", sealedHeight)
}
