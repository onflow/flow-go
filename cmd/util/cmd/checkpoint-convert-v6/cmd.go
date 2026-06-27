package checkpoint_convert_v6

import (
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/ledger/complete/wal"
)

var (
	flagCheckpointDir  string
	flagCheckpoint     string
	flagExecutionDir   string
	flagOutputDir      string
	flagOutput         string
	flagNWorker        uint
	flagPrevCheckpoint int
	flagWALFrom        int
	flagWALTo          int
)

// Cmd reconstructs a full V6 checkpoint from a V7 (payloadless) checkpoint by
// re-sourcing every leaf's payload from a previous full V6 checkpoint plus the
// WAL segments written since.
var Cmd = &cobra.Command{
	Use:   "checkpoint-convert-v6",
	Short: "Reconstruct a V6 checkpoint from a V7 (payloadless) checkpoint.",
	Long: `Reconstruct a full V6 checkpoint from a V7 (payloadless) checkpoint.

A V7 checkpoint stores only a leaf hash per register, not the payload, so it
cannot be turned back into a V6 checkpoint on its own. This command recovers each
payload from two sources in the execution directory:

  - the previous full V6 checkpoint (registers not updated since), and
  - the WAL segments written between that checkpoint and the V7 checkpoint
    (registers that were updated).

For each V7 leaf it finds the payload whose HashLeaf(path, value) matches the
stored leaf hash. By default the previous checkpoint and the WAL range are
auto-discovered: the previous full checkpoint is the latest V6 checkpoint with a
number lower than the V7 checkpoint's number N, and the WAL range is (M, N].
Both can be overridden with flags.

The checkpoint is processed one subtrie partition at a time so only one
partition's payloads are held in memory; --nworker partitions are processed
concurrently (valid range [1, 16]), trading peak memory for speed.

NOTE: the per-trie regSize (AllocatedRegSize) field is metrics-only and is not
stored in a V7 checkpoint; it is written as 0 in the reconstructed checkpoint.
This does not affect trie root hashes.`,
	Run: run,
}

func init() {
	Cmd.Flags().StringVar(&flagCheckpointDir, "checkpoint-dir", "",
		"directory containing the V7 checkpoint files (required)")
	_ = Cmd.MarkFlagRequired("checkpoint-dir")

	Cmd.Flags().StringVar(&flagCheckpoint, "checkpoint", "",
		"V7 checkpoint header filename, e.g. \"checkpoint.00000100.v7\" (required)")
	_ = Cmd.MarkFlagRequired("checkpoint")

	Cmd.Flags().StringVar(&flagExecutionDir, "execution-dir", "",
		"ledger WAL directory holding the previous V6 checkpoint and WAL segments (required)")
	_ = Cmd.MarkFlagRequired("execution-dir")

	Cmd.Flags().StringVar(&flagOutputDir, "output-dir", "",
		"directory to write the reconstructed V6 checkpoint files to (required)")
	_ = Cmd.MarkFlagRequired("output-dir")

	Cmd.Flags().StringVar(&flagOutput, "output", "",
		"V6 output filename. Default: input filename with the \".v7\" suffix removed.")

	Cmd.Flags().UintVar(&flagNWorker, "nworker", 1,
		"number of subtrie partitions to process in parallel (valid range [1, 16])")

	Cmd.Flags().IntVar(&flagPrevCheckpoint, "prev-checkpoint", -1,
		"override the previous full V6 checkpoint number to source payloads from (default: auto-discover)")

	Cmd.Flags().IntVar(&flagWALFrom, "wal-from", -1,
		"override the first WAL segment number to replay (default: previous checkpoint + 1)")

	Cmd.Flags().IntVar(&flagWALTo, "wal-to", -1,
		"override the last WAL segment number to replay (default: V7 checkpoint number)")
}

func run(*cobra.Command, []string) {
	outputFile := flagOutput
	if outputFile == "" {
		outputFile = defaultV6Filename(flagCheckpoint)
	}

	log.Info().
		Str("checkpoint_dir", flagCheckpointDir).
		Str("checkpoint", flagCheckpoint).
		Str("execution_dir", flagExecutionDir).
		Str("output_dir", flagOutputDir).
		Str("output", outputFile).
		Uint("nworker", flagNWorker).
		Int("prev_checkpoint", flagPrevCheckpoint).
		Int("wal_from", flagWALFrom).
		Int("wal_to", flagWALTo).
		Msg("reconstructing V6 checkpoint from V7")

	err := wal.ConvertCheckpointV7ToV6(
		flagCheckpointDir,
		flagCheckpoint,
		flagExecutionDir,
		flagPrevCheckpoint,
		flagWALFrom,
		flagWALTo,
		flagOutputDir,
		outputFile,
		log.Logger,
		flagNWorker,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("checkpoint conversion failed")
	}

	log.Info().
		Str("output", filepath.Join(flagOutputDir, outputFile)).
		Msg("✅ V7→V6 checkpoint reconstruction completed successfully")
}

// defaultV6Filename returns the default V6 output filename for a given V7
// checkpoint filename: strip the ".v7" suffix if present.
func defaultV6Filename(v7Name string) string {
	return strings.TrimSuffix(v7Name, wal.V7FileSuffix)
}
