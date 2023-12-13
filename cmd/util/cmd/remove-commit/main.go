package remove

import (
	"fmt"
	"os"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/storage/badger/operation"
)

var (
	flagDataDir     string
	flagRootBlockID string
)

func main() {
}

var rootCmd = &cobra.Command{
	Use:   "remove-commit",
	Short: "remove-commit",
	Run:   run,
}

var RootCmd = rootCmd

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println("error", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&flagDataDir, "datadir", "d", "/var/flow/data/protocol", "directory to the badger dababase")
	_ = rootCmd.MarkPersistentFlagRequired("datadir")
	rootCmd.PersistentFlags().StringVarP(&flagRootBlockID, "root-block-id", "", "", "root block id")
	_ = rootCmd.MarkPersistentFlagRequired("root-block-id")

	cobra.OnInitialize(initConfig)
}

func initConfig() {
	viper.AutomaticEnv()
}

func run(*cobra.Command, []string) {
	log.Info().
		Str("datadir", flagDataDir).
		Str("rootblockid", flagRootBlockID).
		Msg("flags")

	rootBlockID, err := flow.HexStringToIdentifier(flagRootBlockID)
	if err != nil {
		log.Fatal().Err(err).Msgf("fail to parse block id")
	}

	db := common.InitStorage(flagDataDir)
	defer db.Close()

	metrics := &metrics.NoopCollector{}
	commits := badger.NewCommits(metrics, db)

	err = commits.RemoveByBlockID(flow.ZeroID)
	if err != nil {
		log.Fatal().Err(err).Msgf("fail to remove commit")
	}

	err = commits.RemoveByBlockID(rootBlockID)
	if err != nil {
		log.Fatal().Err(err).Msgf("fail to remove commit")
	}

	err = db.Update(operation.SkipNonExist(operation.RemoveExecutionStateInteractions(rootBlockID)))
	if err != nil {
		log.Fatal().Err(err).Msgf("fail to remove execution state interactions")
	}

	err = db.Update(operation.SkipNonExist(operation.RemoveExecutedBlock()))
	if err != nil {
		log.Fatal().Err(err).Msgf("fail to remove executed block")
	}

	log.Info().Msgf("remove commit success")
}
