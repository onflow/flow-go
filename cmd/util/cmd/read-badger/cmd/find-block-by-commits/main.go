package find

import (
	"encoding/hex"
	"fmt"
	"strings"

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
var flagStateCommitments string

var loader func() (*storage.All, *badger.DB) = nil

func Init(f func() (*storage.All, *badger.DB)) *cobra.Command {
	loader = f

	cmd.Flags().Uint64Var(&flagStartHeight, "start-height", 0, "start height to block for commit")
	_ = cmd.MarkFlagRequired("start-height")

	cmd.Flags().Uint64Var(&flagEndHeight, "end-height", 0, "end height to block for commit")
	_ = cmd.MarkFlagRequired("end-height")

	cmd.Flags().StringVar(&flagStateCommitments, "state-commitments", "",
		"Comma separated list of state commitments (each must be 64 chars, hex-encoded)")
	_ = cmd.MarkFlagRequired("state-commitments")

	return cmd
}

func FindBlockIDByCommits(
	log zerolog.Logger,
	headers storage.Headers,
	commits storage.Commits,
	stateCommitments []flow.StateCommitment,
	startHeight uint64,
	endHeight uint64,
) (flow.Identifier, error) {
	commitMap := make(map[flow.StateCommitment]struct{}, len(stateCommitments))
	for _, commit := range stateCommitments {
		commitMap[commit] = struct{}{}
	}

	for height := startHeight; height <= endHeight; height++ {
		log.Info().Msgf("finding for height %v for height range: [%v, %v]", height, startHeight, endHeight)
		blockID, err := headers.BlockIDByHeight(height)
		if err != nil {
			return flow.ZeroID, fmt.Errorf("could not find block by height %v: %w", height, err)
		}

		commit, err := commits.ByBlockID(blockID)
		if err != nil {
			return flow.ZeroID, fmt.Errorf("could not find commitment at height %v: %w", height, err)
		}

		_, ok := commitMap[commit]
		if ok {
			log.Info().Msgf("successfully found block %v at height %v for commit %v",
				blockID, height, commit)
			return blockID, nil
		}
	}

	return flow.ZeroID, fmt.Errorf("could not find commit within height range [%v,%v]", startHeight, endHeight)
}

func toStateCommitments(commitsStr string) ([]flow.StateCommitment, error) {
	commitSlice := strings.Split(commitsStr, ",")
	commits := make([]flow.StateCommitment, len(commitSlice))
	for _, c := range commitSlice {
		commit, err := toStateCommitment(c)
		if err != nil {
			return nil, err
		}

		commits = append(commits, commit)
	}
	return commits, nil

}

func toStateCommitment(commit string) (flow.StateCommitment, error) {
	stateCommitmentBytes, err := hex.DecodeString(commit)
	if err != nil {
		return flow.DummyStateCommitment, fmt.Errorf("invalid commit string %v, cannot decode", commit)
	}

	stateCommitment, err := flow.ToStateCommitment(stateCommitmentBytes)
	if err != nil {
		return flow.DummyStateCommitment, fmt.Errorf("invalid number of bytes, got %d expected %d, %v", len(stateCommitmentBytes), len(stateCommitment), commit)
	}
	return stateCommitment, nil
}

func run(*cobra.Command, []string) {
	log.Info().Msgf("looking up block in height range [%v, %v] for commits %v",
		flagStartHeight, flagEndHeight, flagStateCommitments)

	stateCommitments, err := toStateCommitments(flagStateCommitments)
	if err != nil {
		log.Fatal().Err(err).Msgf("fail to convert commitment")
	}

	storage, db := loader()
	defer func () { 
		err := db.Close()
		if err != nil {
			log.Warn().Err(err).Msg("error closing db")
		}
	}()

	_, err = FindBlockIDByCommits(
		log.Logger,
		storage.Headers,
		storage.Commits,
		stateCommitments,
		flagStartHeight,
		flagEndHeight,
	)

	if err != nil {
		log.Fatal().Err(err).Msgf("fail to find block id by commit")
	}
}
