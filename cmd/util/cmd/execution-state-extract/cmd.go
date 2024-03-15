package extract

import (
	"encoding/hex"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	runtimeCommon "github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/badger"
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
	flagNoReport                      bool
	flagValidateMigration             bool
	flagLogVerboseValidationError     bool
	flagAllowPartialStateFromPayloads bool
	flagInputPayloadFileName          string
	flagOutputPayloadFileName         string
	flagOutputPayloadByAddresses      string
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

	Cmd.Flags().BoolVar(&flagAllowPartialStateFromPayloads, "allow-partial-state-from-payload-file", false,
		"allow input payload file containing partial state (e.g. not all accounts)")

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
}

func run(*cobra.Command, []string) {
	var stateCommitment flow.StateCommitment

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

	var exportedAddresses []runtimeCommon.Address

	if len(flagOutputPayloadByAddresses) > 0 {

		addresses := strings.Split(flagOutputPayloadByAddresses, ",")

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
	}

	var outputMsg string
	if len(flagOutputPayloadFileName) > 0 {
		// Output is payload file
		if len(exportedAddresses) == 0 {
			outputMsg = fmt.Sprintf("exporting all payloads to %s", flagOutputPayloadFileName)
		} else {
			outputMsg = fmt.Sprintf(
				"exporting payloads by addresses %v to %s",
				flagOutputPayloadByAddresses,
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

	var err error
	if len(flagInputPayloadFileName) > 0 {
		err = extractExecutionStateFromPayloads(
			log.Logger,
			flagExecutionStateDir,
			flagOutputDir,
			flagNWorker,
			!flagNoMigration,
			flagInputPayloadFileName,
			flagOutputPayloadFileName,
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
			flagOutputPayloadFileName,
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
