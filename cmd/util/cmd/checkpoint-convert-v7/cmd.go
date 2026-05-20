package convertv7

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/ledger/complete/wal"
)

var (
	flagInputDir   string
	flagOutputDir  string
	flagCheckpoint string
	flagNWorker    int
	flagLogLevel   string
)

var Cmd = &cobra.Command{
	Use:   "checkpoint-convert-v7",
	Short: "Converts a V6 checkpoint to V7 (payloadless) format",
	Long: `Converts a V6 checkpoint file to V7 (payloadless) format.

V7 checkpoints store payload hashes instead of full payloads, which is used
for payloadless trie mode where actual payloads are stored in a separate
storehouse database.

This conversion is deterministic - running it multiple times on the same
input produces the same output. However, the resulting trie root hashes
will be different from the original V6 checkpoint because the payload
content changes from values to hashes.

Example:
  ./util checkpoint-convert-v7 --input-dir /path/to/checkpoint --output-dir /path/to/output --checkpoint root.checkpoint
`,
	RunE: run,
}

func init() {
	Cmd.Flags().StringVar(&flagInputDir, "input-dir", "",
		"Directory containing the input checkpoint file")
	_ = Cmd.MarkFlagRequired("input-dir")

	Cmd.Flags().StringVar(&flagOutputDir, "output-dir", "",
		"Directory to write the converted V7 checkpoint")
	_ = Cmd.MarkFlagRequired("output-dir")

	Cmd.Flags().StringVar(&flagCheckpoint, "checkpoint", "root.checkpoint",
		"Name of the checkpoint file to convert (default: root.checkpoint)")

	Cmd.Flags().IntVar(&flagNWorker, "nworker", 16,
		"Number of workers for concurrent checkpoint writing (1-16)")

	Cmd.Flags().StringVar(&flagLogLevel, "loglevel", "info",
		"Log level (debug, info, warn, error)")
}

func run(cmd *cobra.Command, _ []string) error {
	// Set up logging
	level, err := zerolog.ParseLevel(flagLogLevel)
	if err != nil {
		return fmt.Errorf("invalid log level %q: %w", flagLogLevel, err)
	}
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger().Level(level)

	// Validate nworker
	if flagNWorker < 1 || flagNWorker > 16 {
		return fmt.Errorf("nworker must be between 1 and 16, got %d", flagNWorker)
	}

	// Ensure output directory exists
	if err := os.MkdirAll(flagOutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Load the input checkpoint
	inputPath := filepath.Join(flagInputDir, flagCheckpoint)
	logger.Info().
		Str("input_path", inputPath).
		Msg("loading checkpoint")

	startLoad := time.Now()
	tries, err := wal.LoadCheckpoint(inputPath, logger)
	if err != nil {
		return fmt.Errorf("failed to load checkpoint: %w", err)
	}
	logger.Info().
		Int("trie_count", len(tries)).
		Dur("duration", time.Since(startLoad)).
		Msg("checkpoint loaded")

	if len(tries) == 0 {
		logger.Warn().Msg("checkpoint contains no tries, nothing to convert")
		return nil
	}

	// Check if already payloadless
	if tries[0].IsPayloadless() {
		logger.Warn().Msg("checkpoint is already in payloadless format (V7), no conversion needed")
		return nil
	}

	// Convert each trie to payloadless format
	logger.Info().
		Int("trie_count", len(tries)).
		Msg("converting tries to payloadless format")

	startConvert := time.Now()
	convertedTries := make([]*trie.MTrie, len(tries))
	for i, t := range tries {
		if t.IsEmpty() {
			convertedTries[i] = trie.NewEmptyMTrieWithPayloadless(true)
			continue
		}

		converted, err := t.ConvertToPayloadless()
		if err != nil {
			return fmt.Errorf("failed to convert trie %d: %w", i, err)
		}
		convertedTries[i] = converted

		if (i+1)%100 == 0 || i == len(tries)-1 {
			logger.Info().
				Int("progress", i+1).
				Int("total", len(tries)).
				Msg("conversion progress")
		}
	}
	logger.Info().
		Dur("duration", time.Since(startConvert)).
		Msg("tries converted to payloadless format")

	// Write the V7 checkpoint
	logger.Info().
		Str("output_dir", flagOutputDir).
		Str("checkpoint", flagCheckpoint).
		Int("nworker", flagNWorker).
		Msg("writing V7 checkpoint")

	startWrite := time.Now()
	err = wal.StoreCheckpointV7(convertedTries, flagOutputDir, flagCheckpoint, logger, uint(flagNWorker))
	if err != nil {
		return fmt.Errorf("failed to write V7 checkpoint: %w", err)
	}
	logger.Info().
		Dur("duration", time.Since(startWrite)).
		Msg("V7 checkpoint written successfully")

	// Log summary
	logger.Info().
		Str("input", inputPath).
		Str("output", filepath.Join(flagOutputDir, flagCheckpoint)).
		Int("tries_converted", len(convertedTries)).
		Dur("total_duration", time.Since(startLoad)).
		Msg("checkpoint conversion complete")

	return nil
}
