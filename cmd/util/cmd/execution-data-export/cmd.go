package exporter

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/dapperlabs/flow-go/model/flow"
)

var (
	flagExecutionStateDir string
	flagOutputDir         string
	flagBlockHash         string
	flagDatadir           string
)

var Cmd = &cobra.Command{
	Use:   "execution-data-export",
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
}

func run(*cobra.Command, []string) {

	blockID, err := flow.HexStringToIdentifier(flagBlockHash)
	if err != nil {
		log.Fatal().Err(err).Msg("malformed block hash")
	}

	log.Info().Msg("start exporting blocks")
	err = ExportBlocks(blockID, flagDatadir, flagOutputDir)
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

	log.Info().Msg("start exporting ledger")
	err = ExportLedger(blockID, flagDatadir, flagExecutionStateDir, flagOutputDir)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot get export ledger")
	}
}
