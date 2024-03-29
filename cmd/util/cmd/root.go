package cmd

import (
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	checkpoint_collect_stats "github.com/onflow/flow-go/cmd/util/cmd/checkpoint-collect-stats"
	checkpoint_list_tries "github.com/onflow/flow-go/cmd/util/cmd/checkpoint-list-tries"
	checkpoint_trie_stats "github.com/onflow/flow-go/cmd/util/cmd/checkpoint-trie-stats"
	epochs "github.com/onflow/flow-go/cmd/util/cmd/epochs/cmd"
	export "github.com/onflow/flow-go/cmd/util/cmd/exec-data-json-export"
	edbs "github.com/onflow/flow-go/cmd/util/cmd/execution-data-blobstore/cmd"
	extract "github.com/onflow/flow-go/cmd/util/cmd/execution-state-extract"
	ledger_json_exporter "github.com/onflow/flow-go/cmd/util/cmd/export-json-execution-state"
	export_json_transactions "github.com/onflow/flow-go/cmd/util/cmd/export-json-transactions"
	extractpayloads "github.com/onflow/flow-go/cmd/util/cmd/extract-payloads-by-address"
	read_badger "github.com/onflow/flow-go/cmd/util/cmd/read-badger/cmd"
	read_execution_state "github.com/onflow/flow-go/cmd/util/cmd/read-execution-state"
	read_hotstuff "github.com/onflow/flow-go/cmd/util/cmd/read-hotstuff/cmd"
	read_protocol_state "github.com/onflow/flow-go/cmd/util/cmd/read-protocol-state/cmd"
	index_er "github.com/onflow/flow-go/cmd/util/cmd/reindex/cmd"
	rollback_executed_height "github.com/onflow/flow-go/cmd/util/cmd/rollback-executed-height/cmd"
	"github.com/onflow/flow-go/cmd/util/cmd/snapshot"
	truncate_database "github.com/onflow/flow-go/cmd/util/cmd/truncate-database"
	"github.com/onflow/flow-go/cmd/util/cmd/version"
)

var (
	flagLogLevel string
)

var rootCmd = &cobra.Command{
	Use:   "util",
	Short: "Utility functions for a flow network",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		setLogLevel()
	},
}

var Cmd = rootCmd

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&flagLogLevel, "loglevel", "l", "info",
		"log level (panic, fatal, error, warn, info, debug)")

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	cobra.OnInitialize(initConfig)

	addCommands()
}

func addCommands() {
	rootCmd.AddCommand(version.Cmd)
	rootCmd.AddCommand(extract.Cmd)
	rootCmd.AddCommand(export.Cmd)
	rootCmd.AddCommand(checkpoint_list_tries.Cmd)
	rootCmd.AddCommand(checkpoint_trie_stats.Cmd)
	rootCmd.AddCommand(checkpoint_collect_stats.Cmd)
	rootCmd.AddCommand(truncate_database.Cmd)
	rootCmd.AddCommand(read_badger.RootCmd)
	rootCmd.AddCommand(read_protocol_state.RootCmd)
	rootCmd.AddCommand(ledger_json_exporter.Cmd)
	rootCmd.AddCommand(epochs.RootCmd)
	rootCmd.AddCommand(edbs.RootCmd)
	rootCmd.AddCommand(index_er.RootCmd)
	rootCmd.AddCommand(rollback_executed_height.Cmd)
	rootCmd.AddCommand(read_execution_state.Cmd)
	rootCmd.AddCommand(snapshot.Cmd)
	rootCmd.AddCommand(export_json_transactions.Cmd)
	rootCmd.AddCommand(read_hotstuff.RootCmd)
	rootCmd.AddCommand(extractpayloads.Cmd)
}

func initConfig() {
	viper.AutomaticEnv()
}

func setLogLevel() {
	switch flagLogLevel {
	case "panic":
		zerolog.SetGlobalLevel(zerolog.PanicLevel)
	case "fatal":
		zerolog.SetGlobalLevel(zerolog.FatalLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "info":
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	default:
		log.Fatal().Str("loglevel", flagLogLevel).Msg("unsupported log level, choose one of \"panic\", \"fatal\", " +
			"\"error\", \"warn\", \"info\" or \"debug\"")
	}
}
