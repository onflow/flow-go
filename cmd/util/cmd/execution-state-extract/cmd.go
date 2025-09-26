package extract

import (
	"encoding/hex"
	"fmt"
	"os"
	"path"
	"runtime/pprof"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	common2 "github.com/onflow/flow-go/cmd/util/common"
	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/store"
)

var (
	flagExecutionStateDir             string
	flagOutputDir                     string
	flagBlockHash                     string
	flagStateCommitment               string
	flagDatadir                       string
	flagChain                         string
	flagNWorker                       int
	flagNoMigration                   bool
	flagMigration                     string
	flagNoReport                      bool
	flagAllowPartialStateFromPayloads bool
	flagSortPayloads                  bool
	flagPrune                         bool
	flagInputPayloadFileName          string
	flagOutputPayloadFileName         string
	flagOutputPayloadByAddresses      string
	flagCPUProfile                    string
	flagZeroMigration                 bool
	flagValidate                      bool
)

var Cmd = &cobra.Command{
	Use:   "execution-state-extract",
	Short: "Reads WAL files and generates the checkpoint containing state commitment for given block hash",
	RunE:  runE,
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

	common.InitDataDirFlag(Cmd, &flagDatadir)

	Cmd.Flags().BoolVar(&flagNoMigration, "no-migration", false,
		"don't migrate the state")

	Cmd.Flags().BoolVar(&flagZeroMigration, "estimate-migration-duration", false,
		"run zero migrations to get minimum duration needed by migrations (load execution state, group payloads by account, iterate account payloads, create trie from payload, and generate checkpoint)")

	Cmd.Flags().StringVar(&flagMigration, "migration", "", "migration name")

	Cmd.Flags().BoolVar(&flagNoReport, "no-report", false,
		"don't report the state")

	Cmd.Flags().IntVar(&flagNWorker, "n-migrate-worker", 10, "number of workers to migrate payload concurrently")

	Cmd.Flags().BoolVar(&flagAllowPartialStateFromPayloads, "allow-partial-state-from-payload-file", false,
		"allow input payload file containing partial state (e.g. not all accounts)")

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

	Cmd.Flags().StringVar(&flagCPUProfile, "cpu-profile", "",
		"enable CPU profiling")

	Cmd.Flags().BoolVar(&flagValidate, "validate-public-key-migration", false,
		"validate migrated account public keys")
}

func runE(*cobra.Command, []string) error {
	if flagCPUProfile != "" {
		f, err := os.Create(flagCPUProfile)
		if err != nil {
			return fmt.Errorf("could not create CPU profile: %w", err)
		}

		err = pprof.StartCPUProfile(f)
		if err != nil {
			return fmt.Errorf("could not start CPU profile: %w", err)
		}

		defer pprof.StopCPUProfile()
	}

	err := os.MkdirAll(flagOutputDir, 0755)
	if err != nil {
		return fmt.Errorf("cannot create output directory %s: %w", flagOutputDir, err)
	}

	if flagNoMigration && flagZeroMigration {
		return fmt.Errorf("cannot run the command with both --no-migration and --estimate-migration-duration flags, one of them or none of them should be provided")
	}

	if len(flagBlockHash) > 0 && len(flagStateCommitment) > 0 {
		return fmt.Errorf("cannot run the command with both block hash and state commitment as inputs, only one of them should be provided")
	}

	if len(flagBlockHash) == 0 && len(flagStateCommitment) == 0 && len(flagInputPayloadFileName) == 0 {
		return fmt.Errorf("--block-hash or --state-commitment or --input-payload-filename must be specified")
	}

	if len(flagInputPayloadFileName) > 0 && (len(flagBlockHash) > 0 || len(flagStateCommitment) > 0) {
		return fmt.Errorf("--input-payload-filename cannot be used with --block-hash or --state-commitment")
	}

	// When flagOutputPayloadByAddresses is specified, flagOutputPayloadFileName is required.
	if len(flagOutputPayloadFileName) == 0 && len(flagOutputPayloadByAddresses) > 0 {
		return fmt.Errorf("--extract-payloads-by-address requires --output-payload-filename to be specified")
	}

	var stateCommitment flow.StateCommitment

	if len(flagBlockHash) > 0 {
		blockID, err := flow.HexStringToIdentifier(flagBlockHash)
		if err != nil {
			return fmt.Errorf("malformed block hash: %w", err)
		}

		log.Info().Msgf("extracting state by block ID: %v", blockID)

		err = common.WithStorage(flagDatadir, func(db storage.DB) error {
			cache := &metrics.NoopCollector{}
			commits := store.NewCommits(cache, db)

			stateCommitment, err = commits.ByBlockID(blockID)
			if err != nil {
				return fmt.Errorf("cannot get state commitment for block %v: %w", blockID, err)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("cannot initialize storage with datadir %s: %w", flagDatadir, err)
		}
	}

	if len(flagStateCommitment) > 0 {
		var err error
		stateCommitmentBytes, err := hex.DecodeString(flagStateCommitment)
		if err != nil {
			return fmt.Errorf("cannot decode the state commitment: %w", err)
		}
		stateCommitment, err = flow.ToStateCommitment(stateCommitmentBytes)
		if err != nil {
			return fmt.Errorf("invalid state commitment length: %w", err)
		}

		log.Info().Msgf("extracting state by state commitment: %x", stateCommitment)
	}

	if len(flagInputPayloadFileName) > 0 {
		if _, err := os.Stat(flagInputPayloadFileName); os.IsNotExist(err) {
			return fmt.Errorf("payload input file %s doesn't exist", flagInputPayloadFileName)
		}

		partialState, err := util.IsPayloadFilePartialState(flagInputPayloadFileName)
		if err != nil {
			return fmt.Errorf("cannot get flag from payload input file %s: %w", flagInputPayloadFileName, err)
		}

		// Check if payload file contains partial state and is allowed by --allow-partial-state-from-payload-file.
		if !flagAllowPartialStateFromPayloads && partialState {
			return fmt.Errorf("payload input file %s contains partial state, please specify --allow-partial-state-from-payload-file", flagInputPayloadFileName)
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
			return fmt.Errorf("payload output file %s exists", flagOutputPayloadFileName)
		}
	}

	var exportPayloadsForOwners map[string]struct{}

	if len(flagOutputPayloadByAddresses) > 0 {
		var err error
		exportPayloadsForOwners, err = common2.ParseOwners(strings.Split(flagOutputPayloadByAddresses, ","))
		if err != nil {
			return fmt.Errorf("failed to parse addresses: %w", err)
		}
	}

	// Validate chain ID
	chain := flow.ChainID(flagChain).Chain()

	if flagNoReport {
		log.Warn().Msgf("--no-report flag is deprecated")
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

	// Extract state and create checkpoint files without migration.
	if flagNoMigration &&
		len(flagInputPayloadFileName) == 0 &&
		len(flagOutputPayloadFileName) == 0 {

		exportedState, err := extractStateToCheckpointWithoutMigration(
			log.Logger,
			flagExecutionStateDir,
			flagOutputDir,
			stateCommitment)
		if err != nil {
			return fmt.Errorf("error extracting state for commitment %s: %w", stateCommitment, err)
		}

		reportExtraction(stateCommitment, exportedState)
		return nil
	}

	if flagZeroMigration {
		newStateCommitment, err := emptyMigration(
			log.Logger,
			flagExecutionStateDir,
			flagOutputDir,
			stateCommitment)
		if err != nil {
			return fmt.Errorf("error extracting state for commitment %s: %w", stateCommitment, err)
		}
		if stateCommitment != flow.StateCommitment(newStateCommitment) {
			return fmt.Errorf("empty migration failed: state commitments are different: %v != %s", stateCommitment, newStateCommitment)
		}
		return nil
	}

	var extractor extractor
	if len(flagInputPayloadFileName) > 0 {
		extractor = newPayloadFileExtractor(log.Logger, flagInputPayloadFileName)
	} else {
		extractor = newExecutionStateExtractor(log.Logger, flagExecutionStateDir, stateCommitment)
	}

	// Extract payloads.

	payloadsFromPartialState, payloads, err := extractor.extract()
	if err != nil {
		return fmt.Errorf("error extracting payloads: %w", err)
	}

	log.Info().Msgf("extracted %d payloads", len(payloads))

	// Migrate payloads.

	if !flagNoMigration {
		var migs []migrations.NamedMigration

		if len(flagMigration) > 0 {
			switch flagMigration {
			case "add-migrationmainnet-keys":
				migs = append(migs, addMigrationMainnetKeysMigration(log.Logger, flagOutputDir, flagNWorker, chain.ChainID())...)
			default:
				return fmt.Errorf("unknown migration: %s", flagMigration)
			}
		}

		migs = append(
			migs,
			migrations.NamedMigration{
				Name: "account-public-key-deduplication",
				Migrate: migrations.NewAccountBasedMigration(
					log.Logger,
					flagNWorker,
					[]migrations.AccountBasedMigration{
						migrations.NewAccountPublicKeyDeduplicationMigration(
							chain.ChainID(),
							flagOutputDir,
							flagValidate,
							reporters.NewReportFileWriterFactory(flagOutputDir, log.Logger),
						),
						migrations.NewAccountUsageMigration(
							reporters.NewReportFileWriterFactoryWithFormat(flagOutputDir, log.Logger, reporters.ReportFormatCSV),
						),
					},
				),
			},
		)

		migration := newMigration(log.Logger, migs, flagNWorker)

		payloads, err = migration(payloads)
		if err != nil {
			return fmt.Errorf("error migrating payloads: %w", err)
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
		return fmt.Errorf("error exporting migrated payloads: %w", err)
	}

	log.Info().Msgf("exported %d payloads", len(payloads))

	reportExtraction(stateCommitment, exportedState)
	return nil
}

func reportExtraction(loadedState flow.StateCommitment, exportedState ledger.State) {
	// Create export reporter.
	reporter := reporters.NewExportReporter(
		log.Logger,
		func() flow.StateCommitment { return loadedState },
	)

	err := reporter.Report(nil, exportedState)
	if err != nil {
		log.Error().Err(err).Msgf("can not generate report for migrated state: %v", exportedState)
	}

	log.Info().Msgf(
		"New state commitment for the exported state is: %s (base64: %s)",
		exportedState.String(),
		exportedState.Base64(),
	)
}

func extractStateToCheckpointWithoutMigration(
	logger zerolog.Logger,
	executionStateDir string,
	outputDir string,
	stateCommitment flow.StateCommitment,
) (ledger.State, error) {
	// Load state for given state commitment
	newTrie, err := util.ReadTrie(executionStateDir, stateCommitment)
	if err != nil {
		return ledger.DummyState, fmt.Errorf("failed to load state: %w", err)
	}

	// Create checkpoint files
	return createCheckpoint(logger, newTrie, outputDir, bootstrap.FilenameWALRootCheckpoint)
}

func emptyMigration(
	logger zerolog.Logger,
	executionStateDir string,
	outputDir string,
	stateCommitment flow.StateCommitment,
) (ledger.State, error) {

	log.Info().Msgf("Loading state with commitment %s", stateCommitment)

	// Load state for given state commitment
	trie, err := util.ReadTrie(executionStateDir, stateCommitment)
	if err != nil {
		return ledger.DummyState, fmt.Errorf("failed to load state: %w", err)
	}

	log.Info().Msgf("Getting payloads from loaded state")

	// Get payloads from trie.
	payloads := trie.AllPayloads()

	log.Info().Msgf("Migrating %d payloads", len(payloads))

	// Migrate payloads (migration is no-op)
	migs := []migrations.NamedMigration{
		{
			Name: "empty migration",
			Migrate: func(*registers.ByAccount) error {
				return nil
			},
		},
	}

	migration := newMigration(log.Logger, migs, flagNWorker)

	migratedPayloads, err := migration(payloads)
	if err != nil {
		return ledger.DummyState, fmt.Errorf("failed to migrate payloads: %w", err)
	}

	log.Info().Msgf("Migrated %d payloads", len(migratedPayloads))

	// Create trie from migrated payloads
	migratedTrie, err := createTrieFromPayloads(log.Logger, payloads)
	if err != nil {
		return ledger.DummyState, fmt.Errorf("failed to create new trie from migrated payloads: %w", err)
	}

	log.Info().Msgf("Created trie from migrated payloads with commitment %s", migratedTrie.RootHash())

	// Create checkpoint files
	newState, err := createCheckpoint(logger, migratedTrie, outputDir, bootstrap.FilenameWALRootCheckpoint)
	if err != nil {
		return ledger.DummyState, fmt.Errorf("failed to create checkpoint: %w", err)
	}

	log.Info().Msgf("Created checkpoint")

	return newState, nil
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
