package extract

import (
	"encoding/hex"
	"fmt"
	"os"
	"path"
	"runtime/pprof"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	common2 "github.com/onflow/flow-go/cmd/util/common"
	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/badger"
)

var (
	flagExecutionStateDir                  string
	flagOutputDir                          string
	flagBlockHash                          string
	flagStateCommitment                    string
	flagDatadir                            string
	flagChain                              string
	flagNWorker                            int
	flagNoMigration                        bool
	flagMigration                          string
	flagAuthorizationFixes                 string
	flagNoReport                           bool
	flagValidateMigration                  bool
	flagAllowPartialStateFromPayloads      bool
	flagSortPayloads                       bool
	flagPrune                              bool
	flagLogVerboseValidationError          bool
	flagDiffMigration                      bool
	flagLogVerboseDiff                     bool
	flagVerboseErrorOutput                 bool
	flagStagedContractsFile                string
	flagContinueMigrationOnValidationError bool
	flagCheckStorageHealthBeforeMigration  bool
	flagCheckStorageHealthAfterMigration   bool
	flagInputPayloadFileName               string
	flagOutputPayloadFileName              string
	flagOutputPayloadByAddresses           string
	flagMaxAccountSize                     uint64
	flagFixSlabsWithBrokenReferences       bool
	flagFilterUnreferencedSlabs            bool
	flagCPUProfile                         string
	flagReportMetrics                      bool
	flagCacheStaticTypeMigrationResults    bool
	flagCacheEntitlementsMigrationResults  bool
)

var Cmd = &cobra.Command{
	Use:   "execution-state-extract",
	Short: "Reads WAL files and generates the checkpoint containing state commitment for given block hash",
	Run:   run,
}

func init() {
	Cmd.Flags().StringVar(&flagExecutionStateDir, "execution-state-dir", "",
		"Execution Node state dir (where WAL logs are written")
	_ = Cmd.MarkFlagRequired("execution-state-dir")

	Cmd.Flags().StringVar(&flagOutputDir, "output-dir", "",
		"Directory to write new Execution State to")
	_ = Cmd.MarkFlagRequired("output-dir")

	Cmd.Flags().StringVar(&flagChain, "chain", "", "Chain name")
	_ = Cmd.MarkFlagRequired("chain")

	Cmd.Flags().StringVar(&flagStateCommitment, "state-commitment", "",
		"state commitment (hex-encoded, 64 characters)")

	Cmd.Flags().StringVar(&flagBlockHash, "block-hash", "",
		"Block hash (hex-encoded, 64 characters)")

	Cmd.Flags().StringVar(&flagDatadir, "datadir", "",
		"directory that stores the protocol state")

	Cmd.Flags().BoolVar(&flagNoMigration, "no-migration", false,
		"don't migrate the state")

	Cmd.Flags().StringVar(&flagMigration, "migration", "cadence-1.0",
		"migration name. 'cadence-1.0' (default) or 'fix-authorizations'")

	Cmd.Flags().StringVar(&flagAuthorizationFixes, "authorization-fixes", "",
		"authorization fixes to apply. requires '--migration=fix-authorizations'")

	Cmd.Flags().BoolVar(&flagNoReport, "no-report", false,
		"don't report the state")

	Cmd.Flags().IntVar(&flagNWorker, "n-migrate-worker", 10, "number of workers to migrate payload concurrently")

	Cmd.Flags().BoolVar(&flagValidateMigration, "validate", false,
		"validate migrated Cadence values (atree migration)")

	Cmd.Flags().BoolVar(&flagLogVerboseValidationError, "log-verbose-validation-error", false,
		"log entire Cadence values on validation error (atree migration)")

	Cmd.Flags().BoolVar(&flagDiffMigration, "diff", false,
		"compare Cadence values and log diff (migration)")

	Cmd.Flags().BoolVar(&flagLogVerboseDiff, "log-verbose-diff", false,
		"log entire Cadence values on diff (requires --diff flag)")

	Cmd.Flags().BoolVar(&flagVerboseErrorOutput, "verbose-error-output", true,
		"log verbose output on migration errors")

	Cmd.Flags().StringVar(&flagStagedContractsFile, "staged-contracts", "",
		"Staged contracts CSV file")

	Cmd.Flags().BoolVar(&flagAllowPartialStateFromPayloads, "allow-partial-state-from-payload-file", false,
		"allow input payload file containing partial state (e.g. not all accounts)")

	Cmd.Flags().BoolVar(&flagCheckStorageHealthBeforeMigration, "check-storage-health-before", false,
		"check (atree) storage health before migration")

	Cmd.Flags().BoolVar(&flagCheckStorageHealthAfterMigration, "check-storage-health-after", false,
		"check (atree) storage health after migration")

	Cmd.Flags().BoolVar(&flagContinueMigrationOnValidationError, "continue-migration-on-validation-errors", false,
		"continue migration even if validation fails")

	Cmd.Flags().BoolVar(&flagSortPayloads, "sort-payloads", true,
		"sort payloads (generate deterministic output; disable only for development purposes)")

	Cmd.Flags().BoolVar(&flagPrune, "prune", false,
		"prune the state (for development purposes)")

	// If specified, the state will consist of payloads from the given input payload file.
	// If not specified, then the state will be extracted from the latest checkpoint file.
	// This flag can be used to reduce total duration of migrations when state extraction involves
	// multiple migrations because it helps avoid repeatedly reading from checkpoint file to rebuild trie.
	// The input payload file must be created by state extraction running with either
	// flagOutputPayloadFileName or flagOutputPayloadByAddresses.
	Cmd.Flags().StringVar(
		&flagInputPayloadFileName,
		"input-payload-filename",
		"",
		"input payload file",
	)

	Cmd.Flags().StringVar(
		&flagOutputPayloadFileName,
		"output-payload-filename",
		"",
		"output payload file",
	)

	Cmd.Flags().StringVar(
		// Extract payloads of specified addresses (comma separated list of hex-encoded addresses)
		// to file specified by --output-payload-filename.
		// If no address is specified (empty string) then this flag is ignored.
		&flagOutputPayloadByAddresses,
		"extract-payloads-by-address",
		"",
		"extract payloads of addresses (comma separated hex-encoded addresses) to file specified by output-payload-filename",
	)

	Cmd.Flags().Uint64Var(&flagMaxAccountSize, "max-account-size", 0,
		"max account size")

	Cmd.Flags().BoolVar(&flagFixSlabsWithBrokenReferences, "fix-testnet-slabs-with-broken-references", false,
		"fix slabs with broken references in testnet")

	Cmd.Flags().BoolVar(&flagFilterUnreferencedSlabs, "filter-unreferenced-slabs", false,
		"filter unreferenced slabs")

	Cmd.Flags().StringVar(&flagCPUProfile, "cpu-profile", "",
		"enable CPU profiling")

	Cmd.Flags().BoolVar(&flagReportMetrics, "report-metrics", false,
		"report migration metrics")

	Cmd.Flags().BoolVar(&flagCacheStaticTypeMigrationResults, "cache-static-type-migration", false,
		"cache static type migration results")

	Cmd.Flags().BoolVar(&flagCacheEntitlementsMigrationResults, "cache-entitlements-migration", false,
		"cache entitlements migration results")
}

func run(*cobra.Command, []string) {
	if flagCPUProfile != "" {
		f, err := os.Create(flagCPUProfile)
		if err != nil {
			log.Fatal().Err(err).Msg("could not create CPU profile")
		}

		err = pprof.StartCPUProfile(f)
		if err != nil {
			log.Fatal().Err(err).Msg("could not start CPU profile")
		}

		defer pprof.StopCPUProfile()
	}

	err := os.MkdirAll(flagOutputDir, 0755)
	if err != nil {
		log.Fatal().Err(err).Msgf("cannot create output directory %s", flagOutputDir)
	}

	if len(flagBlockHash) > 0 && len(flagStateCommitment) > 0 {
		log.Fatal().Msg("cannot run the command with both block hash and state commitment as inputs, only one of them should be provided")
		return
	}

	if len(flagBlockHash) == 0 && len(flagStateCommitment) == 0 && len(flagInputPayloadFileName) == 0 {
		log.Fatal().Msg("--block-hash or --state-commitment or --input-payload-filename must be specified")
	}

	if len(flagInputPayloadFileName) > 0 && (len(flagBlockHash) > 0 || len(flagStateCommitment) > 0) {
		log.Fatal().Msg("--input-payload-filename cannot be used with --block-hash or --state-commitment")
	}

	// When flagOutputPayloadByAddresses is specified, flagOutputPayloadFileName is required.
	if len(flagOutputPayloadFileName) == 0 && len(flagOutputPayloadByAddresses) > 0 {
		log.Fatal().Msg("--extract-payloads-by-address requires --output-payload-filename to be specified")
	}

	if flagValidateMigration && flagDiffMigration {
		log.Fatal().Msg("Both --validate and --diff are enabled, please specify only one (or none) of these")
	}

	switch flagMigration {
	case "cadence-1.0":
		// valid, no-op

	case "fix-authorizations":
		if flagAuthorizationFixes == "" {
			log.Fatal().Msg("--migration=fix-authorizations requires --authorization-fixes")
		}

	default:
		log.Fatal().Msg("Invalid --migration: got %s, expected 'cadence-1.0' or 'fix-authorizations'")
	}

	var stateCommitment flow.StateCommitment

	if len(flagBlockHash) > 0 {
		blockID, err := flow.HexStringToIdentifier(flagBlockHash)
		if err != nil {
			log.Fatal().Err(err).Msg("malformed block hash")
		}

		log.Info().Msgf("extracting state by block ID: %v", blockID)

		db := common.InitStorage(flagDatadir)
		defer db.Close()

		cache := &metrics.NoopCollector{}
		commits := badger.NewCommits(cache, db)

		stateCommitment, err = commits.ByBlockID(blockID)
		if err != nil {
			log.Fatal().Err(err).Msgf("cannot get state commitment for block %v", blockID)
		}
	}

	if len(flagStateCommitment) > 0 {
		var err error
		stateCommitmentBytes, err := hex.DecodeString(flagStateCommitment)
		if err != nil {
			log.Fatal().Err(err).Msg("cannot get decode the state commitment")
		}
		stateCommitment, err = flow.ToStateCommitment(stateCommitmentBytes)
		if err != nil {
			log.Fatal().Err(err).Msg("invalid state commitment length")
		}

		log.Info().Msgf("extracting state by state commitment: %x", stateCommitment)
	}

	if len(flagInputPayloadFileName) > 0 {
		if _, err := os.Stat(flagInputPayloadFileName); os.IsNotExist(err) {
			log.Fatal().Msgf("payload input file %s doesn't exist", flagInputPayloadFileName)
		}

		partialState, err := util.IsPayloadFilePartialState(flagInputPayloadFileName)
		if err != nil {
			log.Fatal().Err(err).Msgf("cannot get flag from payload input file %s", flagInputPayloadFileName)
		}

		// Check if payload file contains partial state and is allowed by --allow-partial-state-from-payload-file.
		if !flagAllowPartialStateFromPayloads && partialState {
			log.Fatal().Msgf("payload input file %s contains partial state, please specify --allow-partial-state-from-payload-file", flagInputPayloadFileName)
		}

		msg := "input payloads represent "
		if partialState {
			msg += "partial state"
		} else {
			msg += "complete state"
		}
		if flagAllowPartialStateFromPayloads {
			msg += ", and --allow-partial-state-from-payload-file is specified"
		} else {
			msg += ", and --allow-partial-state-from-payload-file is NOT specified"
		}
		log.Info().Msg(msg)
	}

	if len(flagOutputPayloadFileName) > 0 {
		if _, err := os.Stat(flagOutputPayloadFileName); os.IsExist(err) {
			log.Fatal().Msgf("payload output file %s exists", flagOutputPayloadFileName)
		}
	}

	var exportPayloadsForOwners map[string]struct{}

	if len(flagOutputPayloadByAddresses) > 0 {
		var err error
		exportPayloadsForOwners, err = common2.ParseOwners(strings.Split(flagOutputPayloadByAddresses, ","))
		if err != nil {
			log.Fatal().Err(err).Msgf("failed to parse addresses")
		}
	}

	// Validate chain ID
	_ = flow.ChainID(flagChain).Chain()

	if flagNoReport {
		log.Warn().Msgf("--no-report flag is deprecated")
	}

	if flagValidateMigration {
		log.Warn().Msgf("--validate flag is enabled and will increase duration of migration")
	}

	if flagLogVerboseValidationError {
		log.Warn().Msgf("--log-verbose-validation-error flag is enabled which may increase size of log")
	}

	if flagDiffMigration {
		log.Warn().Msgf("--diff flag is enabled and will increase duration of migration")
	}

	if flagLogVerboseDiff {
		log.Warn().Msgf("--log-verbose-diff flag is enabled which may increase size of log")
	}

	if flagVerboseErrorOutput {
		log.Warn().Msgf("--verbose-error-output flag is enabled which may increase size of log")
	}

	if flagCheckStorageHealthBeforeMigration {
		log.Warn().Msgf("--check-storage-health-before flag is enabled and will increase duration of migration")
	}

	if flagCheckStorageHealthAfterMigration {
		log.Warn().Msgf("--check-storage-health-after flag is enabled and will increase duration of migration")
	}

	var inputMsg string
	if len(flagInputPayloadFileName) > 0 {
		// Input is payloads
		inputMsg = fmt.Sprintf("reading payloads from %s", flagInputPayloadFileName)
	} else {
		// Input is execution state
		inputMsg = fmt.Sprintf("reading block state commitment %s from %s",
			hex.EncodeToString(stateCommitment[:]),
			flagExecutionStateDir,
		)

		err := ensureCheckpointFileExist(flagExecutionStateDir)
		if err != nil {
			log.Error().Err(err).Msgf("cannot ensure checkpoint file exist in folder %v", flagExecutionStateDir)
		}

	}

	var outputMsg string
	if len(flagOutputPayloadFileName) > 0 {
		// Output is payload file
		if len(exportPayloadsForOwners) == 0 {
			outputMsg = fmt.Sprintf("exporting all payloads to %s", flagOutputPayloadFileName)
		} else {
			outputMsg = fmt.Sprintf(
				"exporting payloads for owners %v to %s",
				common2.OwnersToString(exportPayloadsForOwners),
				flagOutputPayloadFileName,
			)
		}
	} else {
		// Output is checkpoint files
		outputMsg = fmt.Sprintf(
			"exporting root checkpoint to %s, version: %d",
			path.Join(flagOutputDir, bootstrap.FilenameWALRootCheckpoint),
			6,
		)
	}

	log.Info().Msgf("state extraction plan: %s, %s", inputMsg, outputMsg)

	var extractor extractor
	if len(flagInputPayloadFileName) > 0 {
		extractor = newPayloadFileExtractor(log.Logger, flagInputPayloadFileName)
	} else {
		extractor = newExecutionStateExtractor(log.Logger, flagExecutionStateDir, stateCommitment)
	}

	// Extract payloads.

	payloadsFromPartialState, payloads, err := extractor.extract()
	if err != nil {
		log.Fatal().Err(err).Msgf("error extracting payloads: %s", err.Error())
	}

	log.Info().Msgf("extracted %d payloads", len(payloads))

	// Migrate payloads.

	if !flagNoMigration {
		var migs []migrations.NamedMigration

		switch flagMigration {
		default:
			log.Fatal().Msgf("unknown migration: %s", flagMigration)
		}

		migration := newMigration(log.Logger, migs, flagNWorker)

		payloads, err = migration(payloads)
		if err != nil {
			log.Fatal().Err(err).Msgf("error migrating payloads: %s", err.Error())
		}

		log.Info().Msgf("migrated %d payloads", len(payloads))
	}

	// Export migrated payloads.

	var exporter exporter
	if len(flagOutputPayloadFileName) > 0 {
		exporter = newPayloadFileExporter(
			log.Logger,
			flagNWorker,
			flagOutputPayloadFileName,
			exportPayloadsForOwners,
			flagSortPayloads,
		)
	} else {
		exporter = newCheckpointFileExporter(
			log.Logger,
			flagOutputDir,
		)
	}

	log.Info().Msgf("exporting %d payloads", len(payloads))

	exportedState, err := exporter.export(payloadsFromPartialState, payloads)
	if err != nil {
		log.Fatal().Err(err).Msgf("error exporting migrated payloads: %s", err.Error())
	}

	log.Info().Msgf("exported %d payloads", len(payloads))

	// Create export reporter.
	reporter := reporters.NewExportReporter(
		log.Logger,
		func() flow.StateCommitment { return stateCommitment },
	)

	err = reporter.Report(nil, exportedState)
	if err != nil {
		log.Error().Err(err).Msgf("can not generate report for migrated state: %v", exportedState)
	}

	log.Info().Msgf(
		"New state commitment for the exported state is: %s (base64: %s)",
		exportedState.String(),
		exportedState.Base64(),
	)
}

func ensureCheckpointFileExist(dir string) error {
	checkpoints, err := wal.Checkpoints(dir)
	if err != nil {
		return fmt.Errorf("could not find checkpoint files: %v", err)
	}

	if len(checkpoints) != 0 {
		log.Info().Msgf("found checkpoint %v files: %v", len(checkpoints), checkpoints)
		return nil
	}

	has, err := wal.HasRootCheckpoint(dir)
	if err != nil {
		return fmt.Errorf("could not check has root checkpoint: %w", err)
	}

	if has {
		log.Info().Msg("found root checkpoint file")
		return nil
	}

	return fmt.Errorf("no checkpoint file was found, no root checkpoint file was found in %v, check the --execution-state-dir flag", dir)
}
