package checkpoint_trie_stats

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/ledger/complete/mtrie/node"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/ledger/complete/wal"
)

var (
	flagCheckpoint string
	flagTrieIndex  int
	flagFast       bool
)

var Cmd = &cobra.Command{
	Use:   "checkpoint-trie-stats",
	Short: "List the trie node count by types in a checkpoint, show total payload size",
	Run:   run,
}

func init() {

	Cmd.Flags().StringVar(&flagCheckpoint, "checkpoint", "",
		"checkpoint file to read")
	_ = Cmd.MarkFlagRequired("checkpoint")
	Cmd.Flags().IntVar(&flagTrieIndex, "trie-index", 0, "trie index to read, 0 being the first trie, -1 is the last trie")
	Cmd.Flags().BoolVar(&flagFast, "fast", false, "fast mode: iterate nodes without loading full trie (requires checkpoint with exactly 1 trie)")

}

func run(*cobra.Command, []string) {
	if flagFast {
		runFast()
		return
	}

	log.Info().Msgf("loading checkpoint %v, reading %v-th trie", flagCheckpoint, flagTrieIndex)
	res, err := scanCheckpoint(flagCheckpoint, flagTrieIndex, log.Logger)
	if err != nil {
		log.Fatal().Err(err).Msg("fail to scan checkpoint")
	}
	log.Info().
		Str("TrieRootHash", res.trieRootHash).
		Int("InterimNodeCount", res.interimNodeCount).
		Int("LeafNodeCount", res.leafNodeCount).
		Int("TotalPayloadSize", res.totalPayloadSize).
		Msgf("successfully scanned checkpoint %v", flagCheckpoint)
}

func runFast() {
	dir := filepath.Dir(flagCheckpoint)
	fileName := filepath.Base(flagCheckpoint)

	log.Info().Msgf("fast mode: scanning checkpoint %v without loading full trie", flagCheckpoint)

	stats, err := wal.ReadCheckpointStats(dir, fileName, log.Logger)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to read checkpoint stats")
	}

	log.Info().
		Str("TrieRootHash", stats.RootHash.String()).
		Uint64("InterimNodeCount", stats.InterimNodeCount).
		Uint64("LeafNodeCount", stats.LeafNodeCount).
		Uint64("TotalPayloadSize", stats.TotalPayloadSize).
		Msgf("successfully scanned checkpoint %v (fast mode)", flagCheckpoint)
}

type result struct {
	trieRootHash     string
	interimNodeCount int
	leafNodeCount    int
	totalPayloadSize int
}

func readTrie(tries []*trie.MTrie, index int) (*trie.MTrie, error) {
	if len(tries) == 0 {
		return nil, errors.New("No tries available")
	}

	if index < -len(tries) || index >= len(tries) {
		return nil, fmt.Errorf("index %d out of range", index)
	}

	if index < 0 {
		return tries[len(tries)+index], nil
	}

	return tries[index], nil
}

func scanCheckpoint(checkpoint string, trieIndex int, log zerolog.Logger) (result, error) {
	tries, err := wal.LoadCheckpoint(flagCheckpoint, log)
	if err != nil {
		return result{}, fmt.Errorf("error while loading checkpoint: %w", err)
	}

	log.Info().
		Int("total_tries", len(tries)).
		Msg("checkpoint loaded")

	t, err := readTrie(tries, trieIndex)
	if err != nil {
		return result{}, fmt.Errorf("error while reading trie: %w", err)
	}

	log.Info().Msgf("trie loaded, root hash: %v", t.RootHash())

	res := &result{
		trieRootHash:     t.RootHash().String(),
		interimNodeCount: 0,
		leafNodeCount:    0,
		totalPayloadSize: 0,
	}
	processNode := func(n *node.Node) error {
		if n.IsLeaf() {
			res.leafNodeCount++
			res.totalPayloadSize += n.Payload().Size()
		} else {
			res.interimNodeCount++
		}
		return nil
	}

	err = trie.TraverseNodes(t, processNode)
	if err != nil {
		return result{}, fmt.Errorf("fail to traverse the trie: %w", err)
	}

	return *res, nil
}
