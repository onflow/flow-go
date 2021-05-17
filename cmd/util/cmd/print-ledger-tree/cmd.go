package printer

import (
	"fmt"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/module/metrics"
)

var (
	flagExecutionStateDir string
)

var Cmd = &cobra.Command{
	Use:   "print-ledger-tree",
	Short: "prints the ledger tree",
	Run:   run,
}

func init() {
	Cmd.Flags().StringVar(&flagExecutionStateDir, "execution-state-dir", "",
		"Execution Node state dir (where WAL logs are written")
	_ = Cmd.MarkFlagRequired("execution-state-dir")
}

func run(*cobra.Command, []string) {
	log.Info().Msg("start printing ledger")
	err := PrintLedger(flagExecutionStateDir)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot get export ledger")
	}
}

// PrintLedger
func PrintLedger(ledgerPath string) error {

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

	_ = led

	return nil
}
