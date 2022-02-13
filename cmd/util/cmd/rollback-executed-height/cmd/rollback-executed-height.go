package cmd

import (
	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/module/metrics"
	storagebadger "github.com/onflow/flow-go/storage/badger"
)

var (
	flagHeight  uint64
	flagDataDir string
)

// rolls back the execution height to the specified height
var Cmd = &cobra.Command{
	Use:   "rollback-executed-height",
	Short: "Rollback the executed height",
	Run:   run,
}

func init() {

	Cmd.Flags().Uint64Var(&flagHeight, "height", 0,
		"the height of the block to update the highest executed height")
	_ = Cmd.MarkFlagRequired("from-height")

	Cmd.Flags().StringVar(&flagDataDir, "datadir", "",
		"directory that stores the protocol state")
	_ = Cmd.MarkFlagRequired("datadir")
}

func run(*cobra.Command, []string) {
	log.Info().
		Str("datadir", flagDataDir).
		Uint64("height", flagHeight).
		Msg("flags")

	db := common.InitStorage(flagDataDir)

	headers, err := initHeader(db)
	if err != nil {
		log.Fatal().Err(err).Msg("could not init states")
	}

	header, err := headers.ByHeight(flagHeight)
	if err != nil {
		log.Fatal().Err(err).Msgf("could not find finalized height %v", flagHeight)
	}

	err = headers.RollbackExecutedBlock(header)
	if err != nil {
		log.Fatal().Err(err).Msgf("could not roll back executed block at height %v", flagHeight)
	}

	log.Info().Msgf("executed height rolled back to", flagHeight)
}

func initHeader(db *badger.DB) (*storagebadger.Headers, error) {

	metrics := &metrics.NoopCollector{}

	headers := storagebadger.NewHeaders(metrics, db)
	return headers, nil
}
