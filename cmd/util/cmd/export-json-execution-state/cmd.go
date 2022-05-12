package exporter

import (
	"bufio"
	"compress/gzip"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/ledger/complete/mtrie"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/module/metrics"
)

var (
	flagExecutionStateDir string
	flagOutputDir         string
	flagStateCommitment   string
	flagGzip              bool
	flagFullSearch        bool
)

var Cmd = &cobra.Command{
	Use:   "export-json-execution-state",
	Short: "exports execution state into a jsonL file",
	Run:   run,
}

func init() {
	Cmd.Flags().StringVar(&flagExecutionStateDir, "execution-state-dir", "",
		"Execution Node state dir (where WAL logs are written")
	_ = Cmd.MarkFlagRequired("execution-state-dir")

	Cmd.Flags().StringVar(&flagOutputDir, "output-dir", "",
		"Directory to write new Execution State to")
	_ = Cmd.MarkFlagRequired("output-dir")

	Cmd.Flags().StringVar(&flagStateCommitment, "state-commitment", "",
		"State commitment (hex-encoded, 64 characters)")

	Cmd.Flags().BoolVar(&flagGzip, "gzip", true,
		"Write GZip-encoded")

	Cmd.Flags().BoolVar(&flagFullSearch, "full-search", false,
		"Use full search (WARNING - traverse all WALs, extremely slow)")
}

func run(*cobra.Command, []string) {
	log.Info().Msg("start exporting ledger")
	err := ExportLedger(flagExecutionStateDir, flagStateCommitment, flagOutputDir, flagFullSearch)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot get export ledger")
	}
}

// ExportLedger exports ledger key value pairs at the given blockID
func ExportLedger(ledgerPath string, targetstate string, outputPath string, fullSearch bool) error {

	noopMetrics := &metrics.NoopCollector{}

	diskWal, err := wal.NewDiskWAL(
		zerolog.Nop(),
		nil,
		noopMetrics,
		ledgerPath,
		complete.DefaultCacheSize,
		pathfinder.PathByteSize,
		wal.SegmentSize,
	)
	if err != nil {
		return fmt.Errorf("cannot create WAL: %w", err)
	}
	defer func() {
		<-diskWal.Done()
	}()

	var state ledger.State

	if len(targetstate) == 0 {
		if fullSearch {
			return fmt.Errorf("target state must be provided when using full search")
		}
	} else {
		st, err := hex.DecodeString(targetstate)
		if err != nil {
			return fmt.Errorf("failed to decode hex code of state: %w", err)
		}
		state, err = ledger.ToState(st)
		if err != nil {
			return fmt.Errorf("failed to convert bytes to state: %w", err)
		}
	}

	var write func(writer io.Writer) error

	if fullSearch {
		forest, err := mtrie.NewForest(complete.DefaultCacheSize, noopMetrics, func(evictedTrie *trie.MTrie) {})
		if err != nil {
			return fmt.Errorf("cannot create forest: %w", err)
		}

		diskWal.PauseRecord()

		sentinel := fmt.Errorf("we_got_the_trie_error")

		rootState := ledger.RootHash(state)

		err = diskWal.ReplayLogsOnly(
			func(tries []*trie.MTrie) error {
				err = forest.AddTries(tries)
				if err != nil {
					return fmt.Errorf("adding rebuilt tries to forest failed: %w", err)
				}

				for _, trie := range tries {
					rootHash := trie.RootHash()
					if rootState.Equals(rootHash) {
						return sentinel
					}
				}
				return nil
			},
			func(update *ledger.TrieUpdate) error {
				rootHash, err := forest.Update(update)
				if rootState.Equals(rootHash) {
					return sentinel
				}
				return err
			},
			func(rootHash ledger.RootHash) error {
				forest.RemoveTrie(rootHash)
				return nil
			},
		)

		if err != nil {
			if !errors.Is(err, sentinel) {
				return fmt.Errorf("cannot restore LedgerWAL: %w", err)
			}
		}

		write = func(writer io.Writer) error {
			mTrie, err := forest.GetTrie(rootState)
			if err != nil {
				return fmt.Errorf("cannot get trie")
			}
			return mTrie.DumpAsJSON(writer)
		}

	} else {
		led, err := complete.NewLedger(diskWal, complete.DefaultCacheSize, noopMetrics, log.Logger, 0)
		if err != nil {
			return fmt.Errorf("cannot create ledger from write-a-head logs and checkpoints: %w", err)
		}

		// if no target state provided export the most recent state
		if len(targetstate) == 0 {
			state, err = led.MostRecentTouchedState()
			if err != nil {
				return fmt.Errorf("failed to load most recently used state: %w", err)
			}
		}

		write = func(writer io.Writer) error {
			err = led.DumpTrieAsJSON(state, writer)
			if err != nil {
				return fmt.Errorf("cannot dump trie as json: %w", err)
			}
			return nil
		}
	}

	filename := state.String() + ".trie.jsonl"
	if flagGzip {
		filename += ".gz"
	}
	path := filepath.Join(outputPath, filename)

	fi, err := os.Create(path)
	if err != nil {
		return err
	}
	defer fi.Close()

	fileWriter := bufio.NewWriter(fi)
	defer fileWriter.Flush()
	var writer io.Writer = fileWriter

	if flagGzip {
		gzipWriter := gzip.NewWriter(fileWriter)
		defer gzipWriter.Close()
		writer = gzipWriter
	}

	return write(writer)
}
