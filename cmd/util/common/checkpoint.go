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
	"github.com/onflow/flow-go/state/protocol/snapshots"
	"github.com/onflow/flow-go/storage"
)

// FindHeightByCheckpoints finds the height of the sealed block that produces one of the state commitments
// in the given checkpoint file.
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
	// It is safe to skip blocks in this linear search because we expect `stateCommitments` to hold commits
	// for a contiguous range of blocks (for correct operation we assume `blocksToSkip` is smaller than this range).
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
			log.Info().Msgf("successfully found block %v at height %v for commit %x",
				blockID, height, commit)
			return height, commit, nil
		}

		if height < uint64(step) {
			break
		}
	}

	return 0, flow.DummyStateCommitment, fmt.Errorf("could not find commit within height range [%v,%v]", startHeight, endHeight)
}

// GenerateProtocolSnapshotForCheckpoint finds a sealed block that produces the state commitment contained in the latest
// checkpoint file, and return a protocol snapshot for the finalized block that seals the sealed block.
// The returned protocol snapshot can be used for dynamic bootstrapping an execution node along with the latest checkpoint file.
//
// When finding a sealed block it iterates backwards through each sealed height from the last sealed height, and see
// if the state commitment matches with one of the state commitments contained in the checkpoint file.
// However, the iteration could be slow, in order to speed up the iteration, we can skip some blocks each time.
// Since a checkpoint file usually contains 500 tries, which might cover around 250 blocks (assuming 2 tries per block),
// then skipping 10 blocks each time will still allow us to find the sealed block while not missing the height contained
// by the checkpoint file.
// So the blocksToSkip parameter is used to skip some blocks each time when iterating the sealed heights.
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

// findLatestCheckpointFilePath finds the latest checkpoint file in the given directory
// it returns the header file name of the latest checkpoint file
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

// GenerateProtocolSnapshotForCheckpointWithHeights does the same thing as GenerateProtocolSnapshotForCheckpoint
// except that it allows the caller to specify the end height of the sealed block that we iterate backwards from.
func GenerateProtocolSnapshotForCheckpointWithHeights(
	logger zerolog.Logger,
	state protocol.State,
	headers storage.Headers,
	seals storage.Seals,
	checkpointDir string,
	blocksToSkip uint,
	endHeight uint64,
) (protocol.Snapshot, uint64, flow.StateCommitment, error) {
	// Stop searching after 10,000 iterations or upon reaching the minimum height, whichever comes first.
	startHeight := uint64(0)
	// preventing startHeight from being negative
	length := uint64(blocksToSkip+1) * 10000
	if endHeight > length {
		startHeight = endHeight - length
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

	snapshot := state.AtHeight(finalizedHeight)
	validSnapshot, err := snapshots.GetDynamicBootstrapSnapshot(state, snapshot)
	if err != nil {
		return nil, 0, flow.DummyStateCommitment, fmt.Errorf("could not get dynamic bootstrap snapshot: %w", err)
	}

	return validSnapshot, sealedHeight, commit, nil
}

// hashesToCommits converts a list of ledger.RootHash to a list of flow.StateCommitment
func hashesToCommits(hashes []ledger.RootHash) []flow.StateCommitment {
	commits := make([]flow.StateCommitment, len(hashes))
	for i, h := range hashes {
		commits[i] = flow.StateCommitment(h)
	}
	return commits
}

// findFinalizedHeightBySealedHeight finds the finalized height that seals the block with the given sealed height
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
