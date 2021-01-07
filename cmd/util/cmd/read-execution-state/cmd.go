package read

import (
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	list_accounts "github.com/onflow/flow-go/cmd/util/cmd/read-execution-state/list-accounts"
	list_tries "github.com/onflow/flow-go/cmd/util/cmd/read-execution-state/list-tries"
	list_wals "github.com/onflow/flow-go/cmd/util/cmd/read-execution-state/list-wals"

	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/mtrie"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/module/metrics"
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
	Cmd.AddCommand(list_wals.Init())
}

func loadExecutionState() *mtrie.Forest {

	w, err := wal.NewWAL(
		zerolog.Nop(),
		nil,
		flagExecutionStateDir,
		complete.DefaultCacheSize,
		pathfinder.PathByteSize,
		wal.SegmentSize,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("error while creating WAL")
	}

	forest, err := mtrie.NewForest(pathfinder.PathByteSize, flagExecutionStateDir, complete.DefaultCacheSize, metrics.NewNoopCollector(), nil)
	if err != nil {
		log.Fatal().Err(err).Msg("error while creating mForest")
	}

	err = w.ReplayOnForest(forest)
	if err != nil {
		log.Fatal().Err(err).Msg("error while replaying execution state")
	}

	return forest
}

func run(*cobra.Command, []string) {

	log.Info().Msg("reading")

	startTime := time.Now()

	_ = loadExecutionState()

	duration := time.Since(startTime)

	log.Info().Float64("total_time_s", duration.Seconds()).Msg("finished")
}
