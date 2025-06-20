package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/onflow/flow-go/cmd/util/cmd/addresses"
	"github.com/onflow/flow-go/cmd/util/cmd/atree_inlined_status"
	bootstrap_execution_state_payloads "github.com/onflow/flow-go/cmd/util/cmd/bootstrap-execution-state-payloads"
	check_storage "github.com/onflow/flow-go/cmd/util/cmd/check-storage"
	checkpoint_collect_stats "github.com/onflow/flow-go/cmd/util/cmd/checkpoint-collect-stats"
	checkpoint_list_tries "github.com/onflow/flow-go/cmd/util/cmd/checkpoint-list-tries"
	checkpoint_trie_stats "github.com/onflow/flow-go/cmd/util/cmd/checkpoint-trie-stats"
	db_migration "github.com/onflow/flow-go/cmd/util/cmd/db-migration"
	debug_script "github.com/onflow/flow-go/cmd/util/cmd/debug-script"
	debug_tx "github.com/onflow/flow-go/cmd/util/cmd/debug-tx"
	diff_states "github.com/onflow/flow-go/cmd/util/cmd/diff-states"
	epochs "github.com/onflow/flow-go/cmd/util/cmd/epochs/cmd"
	export "github.com/onflow/flow-go/cmd/util/cmd/exec-data-json-export"
	edbs "github.com/onflow/flow-go/cmd/util/cmd/execution-data-blobstore/cmd"
	extract "github.com/onflow/flow-go/cmd/util/cmd/execution-state-extract"
	evm_state_exporter "github.com/onflow/flow-go/cmd/util/cmd/export-evm-state"
	ledger_json_exporter "github.com/onflow/flow-go/cmd/util/cmd/export-json-execution-state"
	export_json_transactions "github.com/onflow/flow-go/cmd/util/cmd/export-json-transactions"
	extractpayloads "github.com/onflow/flow-go/cmd/util/cmd/extract-payloads-by-address"
	find_inconsistent_result "github.com/onflow/flow-go/cmd/util/cmd/find-inconsistent-result"
	find_trie_root "github.com/onflow/flow-go/cmd/util/cmd/find-trie-root"
	generate_authorization_fixes "github.com/onflow/flow-go/cmd/util/cmd/generate-authorization-fixes"
	"github.com/onflow/flow-go/cmd/util/cmd/leaders"
	pebble_checkpoint "github.com/onflow/flow-go/cmd/util/cmd/pebble-checkpoint"
	read_badger "github.com/onflow/flow-go/cmd/util/cmd/read-badger/cmd"
	read_execution_state "github.com/onflow/flow-go/cmd/util/cmd/read-execution-state"
	read_hotstuff "github.com/onflow/flow-go/cmd/util/cmd/read-hotstuff/cmd"
	read_protocol_state "github.com/onflow/flow-go/cmd/util/cmd/read-protocol-state/cmd"
	index_er "github.com/onflow/flow-go/cmd/util/cmd/reindex/cmd"
	rollback_executed_height "github.com/onflow/flow-go/cmd/util/cmd/rollback-executed-height/cmd"
	run_script "github.com/onflow/flow-go/cmd/util/cmd/run-script"
	"github.com/onflow/flow-go/cmd/util/cmd/snapshot"
	system_addresses "github.com/onflow/flow-go/cmd/util/cmd/system-addresses"
	truncate_database "github.com/onflow/flow-go/cmd/util/cmd/truncate-database"
	verify_evm_offchain_replay "github.com/onflow/flow-go/cmd/util/cmd/verify-evm-offchain-replay"
	verify_execution_result "github.com/onflow/flow-go/cmd/util/cmd/verify_execution_result"
	"github.com/onflow/flow-go/cmd/util/cmd/version"
	"github.com/onflow/flow-go/module/profiler"
)

var (
	flagLogLevel         string
	flagProfilerEnabled  bool
	flagProfilerDir      string
	flagProfilerInterval time.Duration
	flagProfilerDuration time.Duration
)

var rootCmd = &cobra.Command{
	Use:   "util",
	Short: "Utility functions for a flow network",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		setLogLevel()
		err := initProfiler()
		if err != nil {
			log.Fatal().Err(err).Msg("could not initialize profiler")
		}
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
	rootCmd.PersistentFlags().BoolVar(&flagProfilerEnabled, "profiler-enabled", false, "whether to enable the auto-profiler")

	rootCmd.PersistentFlags().StringVar(&flagProfilerDir, "profiler-dir", "profiler", "directory to create auto-profiler profiles")
	rootCmd.PersistentFlags().DurationVar(&flagProfilerInterval, "profiler-interval", 1*time.Minute, "the interval between auto-profiler runs")
	rootCmd.PersistentFlags().DurationVar(&flagProfilerDuration, "profiler-duration", 10*time.Second, "the duration to run the auto-profile for")

	log.Logger = log.Output(zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.TimeOnly,
	})

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
	rootCmd.AddCommand(leaders.Cmd)
	rootCmd.AddCommand(epochs.RootCmd)
	rootCmd.AddCommand(edbs.RootCmd)
	rootCmd.AddCommand(index_er.RootCmd)
	rootCmd.AddCommand(rollback_executed_height.Cmd)
	rootCmd.AddCommand(read_execution_state.Cmd)
	rootCmd.AddCommand(snapshot.Cmd)
	rootCmd.AddCommand(export_json_transactions.Cmd)
	rootCmd.AddCommand(read_hotstuff.RootCmd)
	rootCmd.AddCommand(addresses.Cmd)
	rootCmd.AddCommand(bootstrap_execution_state_payloads.Cmd)
	rootCmd.AddCommand(extractpayloads.Cmd)
	rootCmd.AddCommand(find_inconsistent_result.Cmd)
	rootCmd.AddCommand(diff_states.Cmd)
	rootCmd.AddCommand(atree_inlined_status.Cmd)
	rootCmd.AddCommand(find_trie_root.Cmd)
	rootCmd.AddCommand(run_script.Cmd)
	rootCmd.AddCommand(system_addresses.Cmd)
	rootCmd.AddCommand(check_storage.Cmd)
	rootCmd.AddCommand(debug_tx.Cmd)
	rootCmd.AddCommand(debug_script.Cmd)
	rootCmd.AddCommand(generate_authorization_fixes.Cmd)
	rootCmd.AddCommand(evm_state_exporter.Cmd)
	rootCmd.AddCommand(verify_execution_result.Cmd)
	rootCmd.AddCommand(verify_evm_offchain_replay.Cmd)
	rootCmd.AddCommand(pebble_checkpoint.Cmd)
	rootCmd.AddCommand(db_migration.Cmd)
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

func initProfiler() error {
	uploader := &profiler.NoopUploader{}
	profilerConfig := profiler.ProfilerConfig{
		Enabled:         flagProfilerEnabled,
		UploaderEnabled: false,

		Dir:      flagProfilerDir,
		Interval: flagProfilerInterval,
		Duration: flagProfilerDuration,
	}

	profiler, err := profiler.New(log.Logger, uploader, profilerConfig)
	if err != nil {
		return fmt.Errorf("could not initialize profiler: %w", err)
	}

	<-profiler.Ready()
	return nil
}
