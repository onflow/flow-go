package list_tries

import (
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/module/metrics"
)

var flagExecutionStateDir string

var Cmd = &cobra.Command{
	Use:   "list-wals",
	Short: "lists ledger write-a-head(WAL) logs",
	Run:   run,
}

func Init() *cobra.Command {
	Cmd.PersistentFlags().StringVar(&flagExecutionStateDir, "execution-state-dir", "",
		"Execution Node state dir (where WAL logs are written")
	_ = Cmd.MarkPersistentFlagRequired("execution-state-dir")

	return Cmd
}

func run(*cobra.Command, []string) {
	startTime := time.Now()

	w, err := wal.NewDiskWAL(
		zerolog.Nop(),
		nil,
		metrics.NewNoopCollector(),
		flagExecutionStateDir,
		complete.DefaultCacheSize,
		pathfinder.PathByteSize,
		wal.SegmentSize,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("error while creating WAL")
	}
	defer func() {
		<-w.Done()
	}()

	err = w.ReplayLogsOnly(
		func(tries []*trie.MTrie) error {
			fmt.Printf("forest sequencing \n")
			return nil
		},
		func(update *ledger.TrieUpdate) error {
			fmt.Printf("trie update to root hash (%s) \n", update.RootHash.String())
			return nil
		},
		func(rootHash ledger.RootHash) error {
			fmt.Printf("remove trie with root hash (%s) \n", rootHash.String())
			return nil
		},
	)
	if err != nil {
		log.Fatal().Err(err).Msg("error while replaying execution state")
	}

	duration := time.Since(startTime)

	log.Info().Float64("total_time_s", duration.Seconds()).Msg("finished")
}
