package extract

import (
	"encoding/hex"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/badger"
)

var (
	flagExecutionStateDir string
	flagOutputDir         string
	flagBlockHash         string
	flagStateCommitment   string
	flagDatadir           string
	flagNoMigration       bool
	flagNoReport          bool
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

		db := common.InitStorage(flagDatadir)
		defer db.Close()

		cache := &metrics.NoopCollector{}
		commits := badger.NewCommits(cache, db)

		stateCommitment, err = getStateCommitment(commits, blockID)
		if err != nil {
			log.Fatal().Err(err).Msg("cannot get state commitment for block")
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
	}

	log.Info().Msgf("Block state commitment: %s", hex.EncodeToString(stateCommitment[:]))

	err := extractExecutionState(flagExecutionStateDir, stateCommitment, flagOutputDir, log.Logger, !flagNoMigration, !flagNoReport)
	if err != nil {
		log.Fatal().Err(err).Msgf("error extracting the execution state: %s", err.Error())
	}
}
