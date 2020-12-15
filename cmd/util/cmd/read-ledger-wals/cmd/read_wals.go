package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/mtrie/flattener"
	"github.com/onflow/flow-go/ledger/complete/wal"
)

var (
	flagExecutionStateDir string
)

var Cmd = &cobra.Command{
	Use:   "read-ledger-wals",
	Short: "Reads ledger write-a-head(WAL) logs ",
	Run:   run,
}

func init() {

	Cmd.PersistentFlags().StringVar(&flagExecutionStateDir, "execution-state-dir", "",
		"Execution Node state dir (where WAL logs are written")
	_ = Cmd.MarkPersistentFlagRequired("execution-state-dir")

}

func Execute() {
	if err := Cmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func readWals() {

	w, err := wal.NewWAL(
		nil,
		nil,
		flagExecutionStateDir,
		complete.DefaultCacheSize,
		pathfinder.PathByteSize,
		wal.SegmentSize,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("error while creating WAL")
	}

	err = w.ReplayLogsOnly(
		func(forestSequencing *flattener.FlattenedForest) error {
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

}

func run(*cobra.Command, []string) {

	log.Info().Msg("reading")

	startTime := time.Now()

	readWals()

	duration := time.Since(startTime)

	log.Info().Float64("total_time_s", duration.Seconds()).Msg("finished")
}
