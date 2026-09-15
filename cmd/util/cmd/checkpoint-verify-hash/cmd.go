package checkpoint_verify_hash

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/ledger/complete/wal"
)

var (
	flagCheckpointDir string
	flagCheckpoint    string
	flagNWorker       uint
)

// Cmd verifies the cryptographic integrity of a checkpoint (V6 or V7) by
// recomputing every node's hash and comparing it against the hash stored with the
// node, without loading the whole checkpoint into memory.
var Cmd = &cobra.Command{
	Use:   "checkpoint-verify-hash",
	Short: "Verify every node hash in a checkpoint (V6 or V7) by streaming nodes in DFS order.",
	Long: `Verify the cryptographic integrity of a checkpoint (V6 or V7).

Each node is streamed in depth-first order (without loading the whole checkpoint
into memory) and its stored hash is recomputed and compared:
  - leaf nodes are verified from their content (V6 payload value, V7 leaf hash),
  - interim nodes are verified as HashInterNode of their children's hashes.

The 16 subtrie files are verified concurrently using --n-worker goroutines (1-16);
the top trie is then verified using the subtrie node hashes. On any hash mismatch
or integrity violation the command exits fatally.`,
	Run: run,
}

func init() {
	Cmd.Flags().StringVar(&flagCheckpointDir, "checkpoint-dir", "",
		"directory containing the checkpoint files (required)")
	_ = Cmd.MarkFlagRequired("checkpoint-dir")

	Cmd.Flags().StringVar(&flagCheckpoint, "checkpoint", "",
		"checkpoint header filename, e.g. \"checkpoint.00000100\" or \"checkpoint.00000100.v7\" (required)")
	_ = Cmd.MarkFlagRequired("checkpoint")

	Cmd.Flags().UintVar(&flagNWorker, "n-worker", 1,
		"number of subtrie files to verify concurrently (1-16)")
}

func run(*cobra.Command, []string) {
	log.Info().
		Str("checkpoint_dir", flagCheckpointDir).
		Str("checkpoint", flagCheckpoint).
		Uint("n_worker", flagNWorker).
		Msg("verifying checkpoint hashes")

	err := wal.VerifyCheckpointHashes(log.Logger, flagCheckpointDir, flagCheckpoint, flagNWorker)
	if err != nil {
		// A hash mismatch or integrity violation (or any read error) is fatal: the
		// checkpoint cannot be trusted.
		log.Fatal().Err(err).Msg("checkpoint failed hash verification")
	}

	log.Info().Msgf("successfully verified all node hashes in checkpoint %v", flagCheckpoint)
}
