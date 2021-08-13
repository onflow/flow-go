package jsonexporter

import (
	"encoding/hex"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/model/flow"
)

var (
	flagExecutionStateDir string
	flagOutputDir         string
	flagBlockHash         string
	flagDatadir           string
	flagStateCommitment   string
)

var Cmd = &cobra.Command{
	Use:   "exec-data-json-export",
	Short: "exports all the execution data into json files",
	Run:   run,
}

func init() {
	Cmd.Flags().StringVar(&flagExecutionStateDir, "execution-state-dir", "",
		"Execution Node state dir (where WAL logs are written")
	_ = Cmd.MarkFlagRequired("execution-state-dir")

	Cmd.Flags().StringVar(&flagOutputDir, "output-dir", "",
		"Directory to write new Execution State to")
	_ = Cmd.MarkFlagRequired("output-dir")

	Cmd.Flags().StringVar(&flagBlockHash, "block-hash", "",
		"Block hash (hex-encoded, 64 characters)")
	_ = Cmd.MarkFlagRequired("block-hash")

	Cmd.Flags().StringVar(&flagDatadir, "datadir", "",
		"directory that stores the protocol state")
	_ = Cmd.MarkFlagRequired("datadir")

	Cmd.Flags().StringVar(&flagStateCommitment, "state-commitment", "",
		"state commitment (hex-encoded, 64 characters)")
}

func run(*cobra.Command, []string) {

	blockID, err := flow.HexStringToIdentifier(flagBlockHash)
	if err != nil {
		log.Fatal().Err(err).Msg("malformed block hash")
	}

	log.Info().Msg("start exporting blocks")
	fallbackState, err := ExportBlocks(blockID, flagDatadir, flagOutputDir)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot get export blocks")
	}

	log.Info().Msg("start exporting events")
	err = ExportEvents(blockID, flagDatadir, flagOutputDir)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot get export events")
	}

	log.Info().Msg("start exporting transactions")
	err = ExportExecutedTransactions(blockID, flagDatadir, flagOutputDir)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot get export transactions")
	}

	log.Info().Msg("start exporting delta snapshots")
	err = ExportDeltaSnapshots(blockID, flagDatadir, flagOutputDir)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot get export delta snapshots")
	}

	log.Info().Msg("start exporting ledger")
	// if state commitment not provided do the fall back to the one connected to the block
	if len(flagStateCommitment) == 0 {
		flagStateCommitment = hex.EncodeToString(fallbackState[:])
		log.Info().Msg("no state commitment is provided, falling back to the one attached to the block")
	}

	err = ExportLedger(flagExecutionStateDir, flagStateCommitment, flagOutputDir)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot get export ledger")
	}
}
