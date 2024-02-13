package extract

import (
	"encoding/hex"
	"path"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	runtimeCommon "github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/badger"
)

var (
	flagExecutionStateDir         string
	flagOutputDir                 string
	flagBlockHash                 string
	flagStateCommitment           string
	flagDatadir                   string
	flagChain                     string
	flagNWorker                   int
	flagNoMigration               bool
	flagNoReport                  bool
	flagValidateMigration         bool
	flagLogVerboseValidationError bool
	flagInputPayload              bool
	flagOutputPayloadByAddresses  string
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

	Cmd.Flags().StringVar(&flagStateCommitment, "state-commitment", "",
		"state commitment (hex-encoded, 64 characters)")

	Cmd.Flags().StringVar(&flagBlockHash, "block-hash", "",
		"Block hash (hex-encoded, 64 characters)")

	Cmd.Flags().StringVar(&flagDatadir, "datadir", "",
		"directory that stores the protocol state")

	Cmd.Flags().BoolVar(&flagNoMigration, "no-migration", false,
		"don't migrate the state")

	Cmd.Flags().BoolVar(&flagNoReport, "no-report", false,
		"don't report the state")

	Cmd.Flags().IntVar(&flagNWorker, "n-migrate-worker", 10, "number of workers to migrate payload concurrently")

	Cmd.Flags().BoolVar(&flagValidateMigration, "validate", false,
		"validate migrated Cadence values (atree migration)")

	Cmd.Flags().BoolVar(&flagLogVerboseValidationError, "log-verbose-validation-error", false,
		"log entire Cadence values on validation error (atree migration)")

	Cmd.Flags().StringVar(
		&flagOutputPayloadByAddresses,
		"extract-payloads-by-address",
		"",
		"extract payloads of specified addresses (comma separated list of hex-encoded addresses or \"all\"", // empty string ignores this flag
	)

	Cmd.Flags().BoolVar(
		&flagInputPayload,
		"use-payload-as-input",
		false,
		"use payload file instead of checkpoint file as input",
	)
}

func run(*cobra.Command, []string) {
	var stateCommitment flow.StateCommitment

	if len(flagBlockHash) > 0 && len(flagStateCommitment) > 0 {
		log.Fatal().Msg("cannot run the command with both block hash and state commitment as inputs, only one of them should be provided")
		return
	}

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

		stateCommitment, err = getStateCommitment(commits, blockID)
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

	if len(flagBlockHash) == 0 && len(flagStateCommitment) == 0 && !flagInputPayload {
		log.Fatal().Msg("no --block-hash or --state-commitment or --use-payload-as-input was specified")
	}

	exportPayloads := len(flagOutputPayloadByAddresses) > 0

	var exportedAddresses []runtimeCommon.Address

	if exportPayloads {

		addresses := strings.Split(flagOutputPayloadByAddresses, ",")

		if len(addresses) == 1 && strings.TrimSpace(addresses[0]) == "all" {
			// Extract payloads of the entire state.
			log.Info().Msgf("Extracting state from %s, exporting all payloads to %s",
				flagExecutionStateDir,
				path.Join(flagOutputDir, FilenamePayloads),
			)
		} else {
			// Extract payloads of specified accounts
			for _, hexAddr := range addresses {
				b, err := hex.DecodeString(strings.TrimSpace(hexAddr))
				if err != nil {
					log.Fatal().Err(err).Msgf("cannot hex decode address %s for payload export", strings.TrimSpace(hexAddr))
				}

				addr, err := runtimeCommon.BytesToAddress(b)
				if err != nil {
					log.Fatal().Err(err).Msgf("cannot decode address %x for payload export", b)
				}

				exportedAddresses = append(exportedAddresses, addr)
			}

			log.Info().Msgf("Extracting state from %s, exporting payloads by addresses %v to %s",
				flagExecutionStateDir,
				flagOutputPayloadByAddresses,
				path.Join(flagOutputDir, FilenamePayloads),
			)
		}

	} else {
		log.Info().Msgf("Extracting state from %s, exporting root checkpoint to %s, version: %v",
			flagExecutionStateDir,
			path.Join(flagOutputDir, bootstrap.FilenameWALRootCheckpoint),
			6,
		)
	}

	if flagInputPayload {
		log.Info().Msgf("Payload input from %v, output dir: %s",
			flagExecutionStateDir,
			flagOutputDir)
	} else {
		log.Info().Msgf("Block state commitment: %s from %v, output dir: %s",
			hex.EncodeToString(stateCommitment[:]),
			flagExecutionStateDir,
			flagOutputDir)
	}

	// err := ensureCheckpointFileExist(flagExecutionStateDir)
	// if err != nil {
	// 	log.Fatal().Err(err).Msgf("cannot ensure checkpoint file exist in folder %v", flagExecutionStateDir)
	// }

	if len(flagChain) > 0 {
		log.Warn().Msgf("--chain flag is deprecated")
	}

	if flagNoReport {
		log.Warn().Msgf("--no-report flag is deprecated")
	}

	if flagValidateMigration {
		log.Warn().Msgf("atree migration validation flag is enabled and will increase duration of migration")
	}

	if flagLogVerboseValidationError {
		log.Warn().Msgf("atree migration has verbose validation error logging enabled which may increase size of log")
	}

	var err error
	if flagInputPayload {
		err = extractExecutionStateFromPayloads(
			log.Logger,
			flagExecutionStateDir,
			flagOutputDir,
			flagNWorker,
			!flagNoMigration,
			exportPayloads,
			exportedAddresses,
		)
	} else {
		err = extractExecutionState(
			log.Logger,
			flagExecutionStateDir,
			stateCommitment,
			flagOutputDir,
			flagNWorker,
			!flagNoMigration,
			exportPayloads,
			exportedAddresses,
		)
	}

	if err != nil {
		log.Fatal().Err(err).Msgf("error extracting the execution state: %s", err.Error())
	}
}

// func ensureCheckpointFileExist(dir string) error {
// 	checkpoints, err := wal.Checkpoints(dir)
// 	if err != nil {
// 		return fmt.Errorf("could not find checkpoint files: %v", err)
// 	}
//
// 	if len(checkpoints) != 0 {
// 		log.Info().Msgf("found checkpoint %v files: %v", len(checkpoints), checkpoints)
// 		return nil
// 	}
//
// 	has, err := wal.HasRootCheckpoint(dir)
// 	if err != nil {
// 		return fmt.Errorf("could not check has root checkpoint: %w", err)
// 	}
//
// 	if has {
// 		log.Info().Msg("found root checkpoint file")
// 		return nil
// 	}
//
// 	return fmt.Errorf("no checkpoint file was found, no root checkpoint file was found")
// }
