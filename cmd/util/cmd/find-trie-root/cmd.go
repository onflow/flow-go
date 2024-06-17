package find_trie_root

import (
	"encoding/hex"
	"fmt"
	"math"
	"os"

	prometheusWAL "github.com/onflow/wal/wal"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/complete/wal"
)

var (
	flagExecutionStateDir string
	flagRootHash          string
	flagFrom              int
	flagTo                int
	flagOutputDir         string
)

// find trie root hash from the wal files.
// useful for state extraction and rolling back executed height.
// for instance, when extracting state for a target height, it requires the wal files
// has the trie root hash of the target block as the latest few records. If not the case,
// then it is necessary to trim the wal files to the last record with the target trie root hash.
// in order to do that, this command can be used to find the trie root hash in the wal files,
// and copy the wal that contains the trie root hash to a new directory and trim it to
// have the target trie root hash as the last record.
// after that, the new wal file can be used to extract the state for the target height.
var Cmd = &cobra.Command{
	Use:   "find-trie-root",
	Short: "find trie root hash from the wal files",
	Run:   run,
}

func init() {
	Cmd.Flags().StringVar(&flagExecutionStateDir, "execution-state-dir", "/var/flow/data/execution", "directory to the execution state")
	_ = Cmd.MarkFlagRequired("execution-state-dir")

	Cmd.Flags().StringVar(&flagRootHash, "root-hash", "",
		"ledger root hash (hex-encoded, 64 characters)")
	_ = Cmd.MarkFlagRequired("root-hash")

	Cmd.Flags().IntVar(&flagFrom, "from", 0, "from segment")
	Cmd.Flags().IntVar(&flagTo, "to", math.MaxInt32, "to segment")

	Cmd.Flags().StringVar(&flagOutputDir, "output-dir", "", "output directory")
}

func run(*cobra.Command, []string) {
	rootHash, err := parseInput(flagRootHash)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot parse input")
	}

	if flagExecutionStateDir == flagOutputDir {
		log.Fatal().Msg("output directory cannot be the same as the execution state directory")
	}

	segment, offset, err := searchRootHashInSegments(rootHash, flagExecutionStateDir, flagFrom, flagTo)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot find root hash in segments")
	}
	log.Info().Msgf("found root hash in segment %d at offset %d", segment, offset)

	if len(flagOutputDir) == 0 {
		return
	}

	err = copyWAL(flagExecutionStateDir, flagOutputDir, segment, rootHash)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot copy WAL")
	}

	log.Info().Msgf("copied WAL to %s", flagOutputDir)
}

func parseInput(rootHashStr string) (ledger.RootHash, error) {
	rootHashBytes, err := hex.DecodeString(rootHashStr)
	if err != nil {
		return ledger.RootHash(hash.DummyHash), fmt.Errorf("cannot decode root hash: %w", err)
	}
	rootHash, err := ledger.ToRootHash(rootHashBytes)
	if err != nil {
		return ledger.RootHash(hash.DummyHash), fmt.Errorf("invalid root hash: %w", err)
	}
	return rootHash, nil
}

func searchRootHashInSegments(
	expectedHash ledger.RootHash,
	dir string,
	wantFrom, wantTo int,
) (int, int64, error) {
	lg := zerolog.New(os.Stderr).With().Timestamp().Logger()
	from, to, err := prometheusWAL.Segments(dir)
	if err != nil {
		return 0, 0, fmt.Errorf("cannot get segments: %w", err)
	}

	if from < 0 {
		return 0, 0, fmt.Errorf("no segments found in %s", dir)
	}

	if wantFrom > to {
		return 0, 0, fmt.Errorf("from segment %d is greater than the last segment %d", wantFrom, to)
	}

	if wantTo < from {
		return 0, 0, fmt.Errorf("to segment %d is less than the first segment %d", wantTo, from)
	}

	if wantFrom > from {
		from = wantFrom
	}

	if wantTo < to {
		to = wantTo
	}

	lg.Info().
		Str("dir", dir).
		Int("from", from).
		Int("to", to).
		Int("want-from", wantFrom).
		Int("want-to", wantTo).
		Msgf("searching for trie root hash %v in segments [%d,%d]", expectedHash, wantFrom, wantTo)

	sr, err := prometheusWAL.NewSegmentsRangeReader(lg, prometheusWAL.SegmentRange{
		Dir:   dir,
		First: from,
		Last:  to,
	})

	if err != nil {
		return 0, 0, fmt.Errorf("cannot create WAL segments reader: %w", err)
	}

	defer sr.Close()

	reader := prometheusWAL.NewReader(sr)

	for reader.Next() {
		record := reader.Record()
		operation, _, update, err := wal.Decode(record)
		if err != nil {
			return 0, 0, fmt.Errorf("cannot decode LedgerWAL record: %w", err)
		}

		lg = lg.With().
			Uint8("operation", uint8(operation)).
			Logger()

		switch operation {
		case wal.WALUpdate:
			rootHash := update.RootHash

			lg.Debug().
				Str("root-hash", rootHash.String()).
				Msg("found WALUpdate")

			if rootHash.Equals(expectedHash) {
				log.Info().Msgf("found expected trie root hash %v", rootHash)
				return reader.Segment(), reader.Offset(), nil
			}
		default:
		}

		err = reader.Err()
		if err != nil {
			return 0, 0, fmt.Errorf("cannot read LedgerWAL: %w", err)
		}
	}

	return 0, 0, fmt.Errorf("finish reading all segment files from %d to %d, but not found", from, to)
}

func copyWAL(dir, outputDir string, segment int, expectedRoot ledger.RootHash) error {
	writer, err := prometheusWAL.NewSize(log.Logger, nil, outputDir, wal.SegmentSize, false)
	if err != nil {
		return fmt.Errorf("cannot create writer WAL: %w", err)
	}

	defer writer.Close()

	w, err := prometheusWAL.NewSize(log.Logger, nil, dir, wal.SegmentSize, false)
	if err != nil {
		return fmt.Errorf("cannot create WAL: %w", err)
	}

	sr, err := prometheusWAL.NewSegmentsRangeReader(log.Logger, prometheusWAL.SegmentRange{
		Dir:   w.Dir(),
		First: segment,
		Last:  segment,
	})
	if err != nil {
		return fmt.Errorf("cannot create WAL segments reader: %w", err)
	}

	defer sr.Close()

	reader := prometheusWAL.NewReader(sr)

	for reader.Next() {
		record := reader.Record()
		operation, _, update, err := wal.Decode(record)
		if err != nil {
			return fmt.Errorf("cannot decode LedgerWAL record: %w", err)
		}

		switch operation {
		case wal.WALUpdate:

			bytes := wal.EncodeUpdate(update)
			_, err = writer.Log(bytes)
			if err != nil {
				return fmt.Errorf("cannot write LedgerWAL record: %w", err)
			}

			rootHash := update.RootHash

			if rootHash.Equals(expectedRoot) {
				log.Info().Msgf("found expected trie root hash %v, finish writing", rootHash)
				return nil
			}
		default:
		}

		err = reader.Err()
		if err != nil {
			return fmt.Errorf("cannot read LedgerWAL: %w", err)
		}
	}

	return fmt.Errorf("finish reading all segment files from %d to %d, but not found", segment, segment)
}
