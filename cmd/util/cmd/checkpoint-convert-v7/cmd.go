package checkpoint_convert_v7

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
	flagOutputDir      string
	flagOutput         string
	flagNWorker        uint
	flagStream         bool
	flagVerifyLeafHash bool
)

// Cmd converts a V6 checkpoint to a V7 (payloadless) checkpoint by reading
// the V6 part files, projecting every leaf into a payload-hash leaf, and
// re-encoding with the V7 (payloadless) writer.
var Cmd = &cobra.Command{
	Use:   "checkpoint-convert-v7",
	Short: "Convert a V6 checkpoint to a V7 (payloadless) checkpoint.",
	Long: `Convert a V6 checkpoint to a V7 (payloadless) checkpoint.

The V6 checkpoint header file and its 17 part files (subtrie + top-trie) must
all be present. The V7 output uses the same checkpoint number with the
".v7" suffix (e.g. "checkpoint.00000100" -> "checkpoint.00000100.v7") so the
two formats can coexist in the same directory.

Conversion preserves trie root hashes: every V7 trie produced has the same
root hash as the corresponding V6 trie. The 16 V7 subtrie part files are
encoded in parallel using --nworker goroutines.`,
	Run: run,
}

func init() {
	Cmd.Flags().StringVar(&flagCheckpointDir, "checkpoint-dir", "",
		"directory containing the V6 checkpoint files (required)")
	_ = Cmd.MarkFlagRequired("checkpoint-dir")

	Cmd.Flags().StringVar(&flagCheckpoint, "checkpoint", "",
		"V6 checkpoint header filename, e.g. \"checkpoint.00000100\" (required)")
	_ = Cmd.MarkFlagRequired("checkpoint")

	Cmd.Flags().StringVar(&flagOutputDir, "output-dir", "",
		"directory to write the V7 checkpoint files to (default: --checkpoint-dir)")

	Cmd.Flags().StringVar(&flagOutput, "output", "",
		"V7 output filename. Default: input filename + \".v7\".")

	Cmd.Flags().UintVar(&flagNWorker, "nworker", 16,
		"number of subtrie files to encode in parallel (valid range [1, 16])")

	Cmd.Flags().BoolVar(&flagStream, "stream", false,
		"stream part files node-by-node instead of loading the full trie forest into memory "+
			"(constant memory, preserves node hashes without re-deriving root hashes)")

	Cmd.Flags().BoolVar(&flagVerifyLeafHash, "verify-leaf-hash", false,
		"in --stream mode, verify every allocated leaf by comparing the derived V7 node hash "+
			"against the V6 node hash on disk (ignored without --stream, which already cross-checks "+
			"root hashes)")
}

func run(*cobra.Command, []string) {
	outputDir := flagOutputDir
	if outputDir == "" {
		outputDir = flagCheckpointDir
	}

	outputFile := flagOutput
	if outputFile == "" {
		outputFile = defaultV7Filename(flagCheckpoint)
	}

	log.Info().
		Str("checkpoint_dir", flagCheckpointDir).
		Str("checkpoint", flagCheckpoint).
		Str("output_dir", outputDir).
		Str("output", outputFile).
		Uint("nworker", flagNWorker).
		Bool("stream", flagStream).
		Bool("verify_leaf_hash", flagVerifyLeafHash).
		Msg("converting V6 checkpoint to V7")

	var err error
	if flagStream {
		err = wal.ConvertCheckpointV6ToV7Stream(
			flagCheckpointDir,
			flagCheckpoint,
			outputDir,
			outputFile,
			log.Logger,
			flagNWorker,
			flagVerifyLeafHash,
		)
	} else {
		if flagVerifyLeafHash {
			// The in-memory converter already cross-checks every trie root hash,
			// which transitively verifies all leaf hashes, so the flag is a no-op here.
			log.Warn().Msg("--verify-leaf-hash has no effect without --stream; the in-memory converter already verifies root hashes")
		}
		err = wal.ConvertCheckpointV6ToV7(
			flagCheckpointDir,
			flagCheckpoint,
			outputDir,
			outputFile,
			log.Logger,
			flagNWorker,
		)
	}
	if err != nil {
		log.Fatal().Err(err).Msg("checkpoint conversion failed")
	}

	log.Info().
		Str("output", filepath.Join(outputDir, outputFile)).
		Msg("✅ V6→V7 checkpoint conversion completed successfully")
}

// defaultV7Filename returns the default V7 output filename for a given V6
// checkpoint filename: append ".v7" unless it already carries the suffix.
func defaultV7Filename(v6Name string) string {
	if strings.HasSuffix(v6Name, wal.V7FileSuffix) {
		return v6Name
	}
	return v6Name + wal.V7FileSuffix
}
