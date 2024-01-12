package find

import (
	"encoding/hex"
	"fmt"

	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

var cmd = &cobra.Command{
	Use:   "find-block-id-commit",
	Short: "find block ID by commit",
	Run:   run,
}

var flagStartHeight uint64
var flagEndHeight uint64
var flagStateCommitment string

var loader func() (*storage.All, *badger.DB) = nil

func Init(f func() (*storage.All, *badger.DB)) *cobra.Command {
	loader = f

	cmd.Flags().Uint64Var(&flagStartHeight, "start-height", 0, "start height to block for commit")
	_ = cmd.MarkFlagRequired("start-height")

	cmd.Flags().Uint64Var(&flagEndHeight, "end-height", 0, "end height to block for commit")
	_ = cmd.MarkFlagRequired("end-height")

	cmd.Flags().StringVar(&flagStateCommitment, "state-commitment", "",
		"State commitment (64 chars, hex-encoded)")
	_ = cmd.MarkFlagRequired("state-commitment")

	return cmd
}

func FindBlockIDByCommit(
	log zerolog.Logger,
	headers storage.Headers,
	commits storage.Commits,
	commit flow.StateCommitment,
	startHeight uint64,
	endHeight uint64,
) (flow.Identifier, error) {
	for height := startHeight; height <= endHeight; height++ {
		log.Info().Msgf("finding for height %v for height range: [%v, %v]", height, startHeight, endHeight)
		blockID, err := headers.BlockIDByHeight(height)
		if err != nil {
			return flow.ZeroID, fmt.Errorf("could not find block by height %v: %w", height, err)
		}

		executedCommit, err := commits.ByBlockID(blockID)
		if err != nil {
			return flow.ZeroID, fmt.Errorf("could not find commitment at height %v: %w", height, err)
		}

		if commit == executedCommit {
			log.Info().Msgf("successfully found block %v at height %v for commit %v",
				blockID, height, commit)
			return blockID, nil
		}
	}

	return flow.ZeroID, fmt.Errorf("could not find commit within height range [%v,%v]", startHeight, endHeight)
}

func run(*cobra.Command, []string) {
	stateCommitmentBytes, err := hex.DecodeString(flagStateCommitment)
	if err != nil {
		log.Fatal().Err(err).Msg("invalid flag, cannot decode")
	}

	stateCommitment, err := flow.ToStateCommitment(stateCommitmentBytes)
	if err != nil {
		log.Fatal().Err(err).Msgf("invalid number of bytes, got %d expected %d", len(stateCommitmentBytes), len(stateCommitment))
	}

	storage, db := loader()
	defer db.Close()

	_, err = FindBlockIDByCommit(
		log.Logger,
		storage.Headers,
		storage.Commits,
		stateCommitment,
		flagStartHeight,
		flagEndHeight,
	)

	if err != nil {
		log.Fatal().Err(err).Msgf("fail to find block id by commit")
	}

}
