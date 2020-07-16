package read

import (
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	list_accounts "github.com/dapperlabs/flow-go/cmd/util/cmd/read-execution-state/list-accounts"
	list_tries "github.com/dapperlabs/flow-go/cmd/util/cmd/read-execution-state/list-tries"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/storage/ledger"
	"github.com/dapperlabs/flow-go/storage/ledger/mtrie"
	"github.com/dapperlabs/flow-go/storage/ledger/wal"
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

	Cmd.PersistentFlags().StringVar(&flagExecutionStateDir, "execution-state-dir", "",
		"Execution Node state dir (where WAL logs are written")
	_ = Cmd.MarkPersistentFlagRequired("execution-state-dir")

	addSubcommands()
}

func addSubcommands() {
	Cmd.AddCommand(list_tries.Init(loadExecutionState))
	Cmd.AddCommand(list_accounts.Init(loadExecutionState))
}

func loadExecutionState() *mtrie.MForest {

	w, err := wal.NewWAL(
		nil,
		nil,
		flagExecutionStateDir,
		ledger.CacheSize,
		ledger.RegisterKeySize,
		wal.SegmentSize,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("error while creating WAL")
	}

	mForest, err := mtrie.NewMForest(ledger.RegisterKeySize, flagExecutionStateDir, ledger.CacheSize, metrics.NewNoopCollector(), nil)
	if err != nil {
		log.Fatal().Err(err).Msg("error while creating mForest")
	}

	err = w.ReplayOnMForest(mForest)
	if err != nil {
		log.Fatal().Err(err).Msg("error while replaying execution state")
	}

	return mForest
}

func run(*cobra.Command, []string) {

	log.Info().Msg("reading")

	startTime := time.Now()

	_ = loadExecutionState()

	duration := time.Since(startTime)

	log.Info().Float64("total_time_s", duration.Seconds()).Msg("finished")
}
