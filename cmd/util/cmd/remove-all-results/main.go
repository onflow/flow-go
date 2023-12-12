package remove

import (
	"fmt"
	"os"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/storage/badger/operation"
)

var (
	flagDataDir string
)

func main() {
}

var rootCmd = &cobra.Command{
	Use:   "remove-all-results",
	Short: "remove-all-results",
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

	cobra.OnInitialize(initConfig)
}

func initConfig() {
	viper.AutomaticEnv()
}

func run(*cobra.Command, []string) {
	log.Info().
		Str("datadir", flagDataDir).
		Msg("flags")

	db := common.InitStorage(flagDataDir)
	defer db.Close()

	err := operation.RemoveAll(db)
	if err != nil {
		log.Fatal().Err(err).Msgf("fail to remove commit")
	}

	log.Info().Msgf("remove commit success")
}
