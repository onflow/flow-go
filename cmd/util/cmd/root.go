package cmd

import (
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	checkpoint_list_tries "github.com/onflow/flow-go/cmd/util/cmd/checkpoint-list-tries"
	epochs "github.com/onflow/flow-go/cmd/util/cmd/epochs/cmd"
	export "github.com/onflow/flow-go/cmd/util/cmd/exec-data-json-export"
	edbs "github.com/onflow/flow-go/cmd/util/cmd/execution-data-blobstore/cmd"
	extract "github.com/onflow/flow-go/cmd/util/cmd/execution-state-extract"
	ledger_json_exporter "github.com/onflow/flow-go/cmd/util/cmd/export-json-execution-state"
	read_badger "github.com/onflow/flow-go/cmd/util/cmd/read-badger/cmd"
	read_protocol_state "github.com/onflow/flow-go/cmd/util/cmd/read-protocol-state/cmd"
	truncate_database "github.com/onflow/flow-go/cmd/util/cmd/truncate-database"
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
	rootCmd.AddCommand(extract.Cmd)
	rootCmd.AddCommand(export.Cmd)
	rootCmd.AddCommand(checkpoint_list_tries.Cmd)
	rootCmd.AddCommand(truncate_database.Cmd)
	rootCmd.AddCommand(read_badger.RootCmd)
	rootCmd.AddCommand(read_protocol_state.RootCmd)
	rootCmd.AddCommand(ledger_json_exporter.Cmd)
	rootCmd.AddCommand(epochs.RootCmd)
	rootCmd.AddCommand(edbs.RootCmd)
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
