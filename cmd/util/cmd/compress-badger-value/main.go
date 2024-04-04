package compress

import (
	"context"
	"fmt"

	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/onflow/flow-go/storage/badger/operation"
)

var flagSourceDir string
var flagOutputDir string
var flagBatchMaxLen int
var flagBatchMaxByteSize int
var flagChanSize int
var flagWorkerCount int

var Cmd = &cobra.Command{
	Use:   "compress-badger-value",
	Short: "compress all badger values and save to a new badger directory",
	Run:   run,
}

func init() {
	Cmd.Flags().StringVar(&flagSourceDir, "source-dir", "",
		"Directory to read badger values from")
	_ = Cmd.MarkFlagRequired("source-dir")

	Cmd.Flags().StringVar(&flagOutputDir, "output-dir", "",
		"Directory to write compressed badger values to")
	_ = Cmd.MarkFlagRequired("output-dir")

	Cmd.Flags().IntVar(&flagBatchMaxLen, "batch-max-len", 1000,
		"Max number of batch items")

	Cmd.Flags().IntVar(&flagBatchMaxByteSize, "batch-max-byte-size", 100000,
		"Max number of batch byte size")

	Cmd.Flags().IntVar(&flagChanSize, "chan-size", 2000,
		"Size of the channel to store key values")

	Cmd.Flags().IntVar(&flagWorkerCount, "worker-count", 1,
		"Number of workers to compress and store values")

}

func run(*cobra.Command, []string) {
	err := runCmd(flagSourceDir, flagOutputDir, flagBatchMaxLen, flagBatchMaxByteSize, flagChanSize, flagWorkerCount)
	if err != nil {
		log.Logger.Fatal().Err(err).Msgf("fail to compress values for badger")
	}
}

func runCmd(srcDir, outputDir string, batchMaxLen, batchMaxByteSize int, chanSize int, workerCount int) error {
	srcDB, err := badger.Open(badgerOptions(srcDir))
	if err != nil {
		return fmt.Errorf("could not open source badger db %v: %w", srcDir, err)
	}
	desDB, err := badger.Open(badgerOptions(outputDir))
	if err != nil {
		return fmt.Errorf("could not open destination badger db %v: %w", outputDir, err)
	}

	keyvals := make(chan *operation.KeyValue, chanSize)

	cct, cancel := context.WithCancel(context.Background())
	defer cancel()

	g, gCtx := errgroup.WithContext(cct)
	for i := 0; i < workerCount; i++ {
		g.Go(func() error {
			err = operation.CompressAndStore(log.Logger, gCtx, keyvals, desDB, batchMaxLen, batchMaxByteSize)
			if err != nil {
				return fmt.Errorf("could not compress and store values: %w", err)
			}
			return nil
		})
	}

	err = operation.TraverseKeyValues(keyvals, srcDB)
	if err != nil {
		return fmt.Errorf("could not traverse key values: %w", err)
	}

	if err = g.Wait(); err != nil {
		return fmt.Errorf("failed to compress and store badger values: %w", err)
	}

	return nil
}

func badgerOptions(dir string) badger.Options {
	// we initialize the database with options that allow us to keep the maximum
	// item size in the trie itself (up to 1MB) and where we keep all level zero
	// tables in-memory as well; this slows down compaction and increases memory
	// usage, but it improves overall performance and disk i/o
	return badger.
		DefaultOptions(dir).
		WithKeepL0InMemory(true).

		// the ValueLogFileSize option specifies how big the value of a
		// key-value pair is allowed to be saved into badger.
		// exceeding this limit, will fail with an error like this:
		// could not store data: Value with size <xxxx> exceeded 1073741824 limit
		// Maximum value size is 10G, needed by execution node
		// TODO: finding a better max value for each node type
		WithValueLogFileSize(128 << 23).
		WithValueLogMaxEntries(100000) // Default is 1000000
}
