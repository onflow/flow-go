package exporter

import (
	"bufio"
	"compress/gzip"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

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

	Cmd.Flags().BoolVar(&flagGzip, "gzip", false,
		"Write GZip-encoded")
}

func run(*cobra.Command, []string) {
	log.Info().Msg("start exporting ledger")
	err := ExportLedger(flagExecutionStateDir, flagStateCommitment, flagOutputDir)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot get export ledger")
	}
}

// ExportLedger exports ledger key value pairs at the given blockID
func ExportLedger(ledgerPath string, targetstate string, outputPath string) error {

	diskWal, err := wal.NewDiskWAL(
		zerolog.Nop(),
		nil,
		&metrics.NoopCollector{},
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

	led, err := complete.NewLedger(diskWal, complete.DefaultCacheSize, &metrics.NoopCollector{}, log.Logger, 0)
	if err != nil {
		return fmt.Errorf("cannot create ledger from write-a-head logs and checkpoints: %w", err)
	}

	var state ledger.State
	// if no target state provided export the most recent state
	if len(targetstate) == 0 {
		state, err = led.MostRecentTouchedState()
		if err != nil {
			return fmt.Errorf("failed to load most recently used state: %w", err)
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

	err = led.DumpTrieAsJSON(state, writer)
	if err != nil {
		return fmt.Errorf("cannot dump trie as json: %w", err)
	}
	return nil
}
