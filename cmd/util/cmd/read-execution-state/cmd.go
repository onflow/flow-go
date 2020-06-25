package read

import (
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/storage/ledger"
)

var (
	flagExecutionStateDir string
)

var Cmd = &cobra.Command{
	Use:   "read-execution-state",
	Short: "Reads Execution State and will allow (one day) queries against it",
	Run:   run,
}

func init() {

	Cmd.Flags().StringVar(&flagExecutionStateDir, "execution-state-dir", "",
		"Execution Node state dir (where WAL logs are written")
	_ = Cmd.MarkFlagRequired("execution-state-dir")

}

func run(*cobra.Command, []string) {

	startTime := time.Now()

	_, err := ledger.NewMTrieStorage(flagExecutionStateDir, ledger.CacheSize, metrics.NewNoopCollector(), nil)
	if err != nil {
		log.Fatal().Err(err).Msg("error while reading execution state")
	}

	duration := time.Since(startTime)

	log.Info().Float64("total_time_s", duration.Seconds()).Msg("finished")
}
