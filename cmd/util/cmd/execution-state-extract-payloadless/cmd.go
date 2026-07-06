package extractpayloadless

import (
	"encoding/hex"
	"fmt"
	"os"
	"path"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/complete/payloadless"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
)

var (
	flagExecutionStateDir string
	flagOutputDir         string
	flagStateCommitment   string
	flagNWorker           uint
	flagMTrieCacheSize    uint32
)

// Cmd extracts the payloadless (V7) trie at a given state commitment from a WAL directory and writes
// it as a single-trie V7 root checkpoint. It is the payloadless counterpart of
// execution-state-extract: no migration is performed and no payloads are read, because a payloadless
// trie stores only leaf hashes.
var Cmd = &cobra.Command{
	Use:   "execution-state-extract-payloadless",
	Short: "Extract a payloadless (V7) trie at a state commitment into a V7 root checkpoint",
	Long: `Extract the payloadless (V7) trie at a given state commitment and write it as a V7 root checkpoint.

The trie is loaded from the WAL directory (--execution-state-dir), recovering in-memory state from the
latest V7 checkpoint plus any newer WAL segments, exactly like the node does at startup. The trie whose
root hash matches --state-commitment is written to --output-dir as a single-trie V7 root checkpoint
("` + bootstrap.FilenameWALRootCheckpoint + wal.V7FileSuffix + `").

Because a payloadless trie carries only leaf hashes and no payloads, no migration is possible or needed;
this command only re-checkpoints the selected trie. It acquires an exclusive lock on the WAL directory,
so it must be run against a stopped node's data directory.`,
	RunE: runE,
}

func init() {
	Cmd.Flags().StringVar(&flagExecutionStateDir, "execution-state-dir", "",
		"Execution Node state dir (where the V7 checkpoint and WAL logs are written)")
	_ = Cmd.MarkFlagRequired("execution-state-dir")

	Cmd.Flags().StringVar(&flagOutputDir, "output-dir", "",
		"Directory to write the V7 root checkpoint to")
	_ = Cmd.MarkFlagRequired("output-dir")

	Cmd.Flags().StringVar(&flagStateCommitment, "state-commitment", "",
		"state commitment of the trie to extract (hex-encoded, 64 characters)")
	_ = Cmd.MarkFlagRequired("state-commitment")

	Cmd.Flags().UintVar(&flagNWorker, "nworker", 16,
		"number of subtrie files to encode in parallel (valid range [1, 16])")

	Cmd.Flags().Uint32Var(&flagMTrieCacheSize, "mtrie-cache-size", ledger.DefaultMTrieCacheSize,
		"number of tries retained in the forest during WAL replay; match the node's --mtrie-cache-size. "+
			"This is the main driver of peak memory; lower it to reduce memory (at the risk of failing to "+
			"resolve tries across WAL forks)")
}

func runE(*cobra.Command, []string) error {
	stateCommitmentBytes, err := hex.DecodeString(flagStateCommitment)
	if err != nil {
		return fmt.Errorf("cannot decode state commitment: %w", err)
	}
	stateCommitment, err := flow.ToStateCommitment(stateCommitmentBytes)
	if err != nil {
		return fmt.Errorf("invalid state commitment length: %w", err)
	}

	outputFile := bootstrap.FilenameWALRootCheckpoint + wal.V7FileSuffix

	log.Info().
		Str("execution-state-dir", flagExecutionStateDir).
		Str("output-dir", flagOutputDir).
		Str("state-commitment", stateCommitment.String()).
		Str("output", path.Join(flagOutputDir, outputFile)).
		Msg("extracting payloadless (V7) trie at state commitment")

	if err := os.MkdirAll(flagOutputDir, 0755); err != nil {
		return fmt.Errorf("cannot create output directory %s: %w", flagOutputDir, err)
	}

	trie, err := util.ReadPayloadlessTrie(flagExecutionStateDir, stateCommitment, int(flagMTrieCacheSize))
	if err != nil {
		return fmt.Errorf("cannot read payloadless trie for state commitment %s: %w", stateCommitment, err)
	}

	log.Info().
		Str("root_hash", trie.RootHash().String()).
		Uint64("allocated_reg_count", trie.AllocatedRegCount()).
		Msg("loaded payloadless trie, storing V7 root checkpoint")

	err = wal.StoreCheckpointV7(
		[]*payloadless.MTrie{trie},
		flagOutputDir,
		outputFile,
		log.Logger,
		flagNWorker,
	)
	if err != nil {
		return fmt.Errorf("cannot store V7 root checkpoint: %w", err)
	}

	log.Info().
		Str("state-commitment", ledger.State(trie.RootHash()).String()).
		Str("output", path.Join(flagOutputDir, outputFile)).
		Msg("✅ payloadless (V7) state extraction completed successfully")
	return nil
}
