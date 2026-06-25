package checkpoint_iterate_nodes

import (
	"errors"
	"fmt"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/ledger/complete/wal"
)

var (
	flagCheckpointDir string
	flagCheckpoint    string
)

// Cmd streams every node of a checkpoint (V6 or V7) in descendants-first (DFS)
// order without loading the whole checkpoint into memory, reports node-type
// counts and total payload size, and verifies the trie structural integrity.
var Cmd = &cobra.Command{
	Use:   "checkpoint-iterate-nodes",
	Short: "Stream a checkpoint node-by-node, report node-type counts, and verify trie integrity.",
	Long: `Stream a checkpoint (V6 or V7) node-by-node in depth-first order without loading
the whole checkpoint into memory.

It reports:
  - the number of leaf nodes and interim nodes,
  - the number of interim nodes that have a single (non-nil) child,
  - the total payload size across leaf nodes (V6 only; V7 stores no payloads).

While streaming it verifies trie structural integrity: every interim node must
reference only already-seen, non-default children, and every node must be
referenced by some parent or trie root. On any integrity violation the command
exits fatally.`,
	Run: run,
}

func init() {
	Cmd.Flags().StringVar(&flagCheckpointDir, "checkpoint-dir", "",
		"directory containing the checkpoint files (required)")
	_ = Cmd.MarkFlagRequired("checkpoint-dir")

	Cmd.Flags().StringVar(&flagCheckpoint, "checkpoint", "",
		"checkpoint header filename, e.g. \"checkpoint.00000100\" or \"checkpoint.00000100.v7\" (required)")
	_ = Cmd.MarkFlagRequired("checkpoint")
}

func run(*cobra.Command, []string) {
	log.Info().
		Str("checkpoint_dir", flagCheckpointDir).
		Str("checkpoint", flagCheckpoint).
		Msg("iterating checkpoint nodes")

	res, err := iterateCheckpoint(flagCheckpointDir, flagCheckpoint, log.Logger)
	if err != nil {
		// An integrity violation (or any read error) is fatal: the checkpoint
		// cannot be trusted.
		if errors.Is(err, wal.ErrCheckpointIntegrity) {
			log.Fatal().Err(err).Msg("checkpoint failed integrity verification")
		}
		log.Fatal().Err(err).Msg("fail to iterate checkpoint nodes")
	}

	log.Info().
		Uint64("TotalNodes", res.totalNodes).
		Uint64("LeafNodes", res.leafNodes).
		Uint64("InterimNodes", res.interimNodes).
		Uint64("InterimWithSingleChild", res.interimSingleChild).
		Uint64("LeavesWithPayload", res.leavesWithPayload).
		Uint64("TotalPayloadSize", res.totalPayloadSize).
		Msgf("successfully iterated checkpoint %v", flagCheckpoint)
}

// result accumulates the statistics reported over the whole checkpoint forest.
type result struct {
	totalNodes   uint64
	leafNodes    uint64
	interimNodes uint64
	// interimSingleChild counts interim nodes with exactly one non-nil child
	// (the other child index is 0).
	interimSingleChild uint64
	// leavesWithPayload counts leaf nodes carrying a non-empty payload (V6).
	leavesWithPayload uint64
	// totalPayloadSize is the sum of encoded payload sizes across leaf nodes (V6).
	totalPayloadSize uint64
}

func iterateCheckpoint(dir string, fileName string, logger zerolog.Logger) (result, error) {
	var res result

	err := wal.IterateCheckpointNodes(logger, dir, fileName, func(n *wal.CheckpointNode) error {
		res.totalNodes++

		if n.IsLeaf {
			res.leafNodes++
			if n.PayloadSize > 0 {
				res.leavesWithPayload++
				res.totalPayloadSize += uint64(n.PayloadSize)
			}
			return nil
		}

		res.interimNodes++

		// An interim node with exactly one nil child is legitimate in a compactified
		// trie (the present child is itself an interim node). Both-nil cannot occur,
		// and a non-nil default child is rejected as an integrity violation by the
		// iterator, so the only remaining case to count here is the single-child one.
		leftNil := n.LeftChildIndex == 0
		rightNil := n.RightChildIndex == 0
		if leftNil != rightNil {
			res.interimSingleChild++
		}

		return nil
	})
	if err != nil {
		return result{}, fmt.Errorf("error while iterating checkpoint: %w", err)
	}

	return res, nil
}
