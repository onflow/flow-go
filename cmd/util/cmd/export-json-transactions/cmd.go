package export_transaction

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/cmd/util/cmd/export-json-transactions/transactions"
)

var flagDatadir string
var flagOutputDir string
var flagStartHeight uint64
var flagEndHeight uint64

// example:
// ./util export-json-transactions --output-dir ./ --datadir /var/fow/data/protocol/ --start-height 2 --end-height 242
var Cmd = &cobra.Command{
	Use:   "export-json-transactions",
	Short: "exports transactions into a json file",
	Run:   run,
}

func init() {
	Cmd.Flags().StringVar(&flagDatadir, "datadir", "/var/flow/data/protocol",
		"the protocol state")

	Cmd.Flags().StringVar(&flagOutputDir, "output-dir", "",
		"Directory to write transactions JSON to")
	_ = Cmd.MarkFlagRequired("output-dir")

	Cmd.Flags().Uint64Var(&flagStartHeight, "start-height", 0,
		"Start height of the block range")
	_ = Cmd.MarkFlagRequired("start-height")

	Cmd.Flags().Uint64Var(&flagEndHeight, "end-height", 0,
		"End height of the block range")
	_ = Cmd.MarkFlagRequired("end-height")
}

func run(*cobra.Command, []string) {
	log.Info().Msg("start exporting transactions")
	err := ExportTransactions(flagDatadir, flagOutputDir, flagStartHeight, flagEndHeight)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot get export transactions")
	}
	log.Info().Msg("transactions exported")
}

func writeJSONTo(writer io.Writer, jsonData []byte) error {
	_, err := writer.Write(jsonData)
	return err
}

// ExportTransactions exports transactions to JSON to the outputDir for height range specified by
// startHeight and endHeight
func ExportTransactions(dataDir string, outputDir string, startHeight uint64, endHeight uint64) error {

	// init dependencies
	db, err := common.InitStoragePebble(flagDatadir)
	if err != nil {
		return fmt.Errorf("could not init storage: %w", err)
	}
	defer db.Close()

	storages := common.InitStoragesPebble(db)
	state, err := common.InitProtocolStatePebble(db, storages)
	if err != nil {
		return fmt.Errorf("could not init protocol state: %w", err)
	}

	// create finder
	finder := &transactions.Finder{
		State:       state,
		Payloads:    storages.Payloads,
		Collections: storages.Collections,
	}

	// create JSON file writer
	outputFile := filepath.Join(outputDir, "transactions.json")
	fi, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("could not create block output file %v, %w", outputFile, err)
	}
	defer fi.Close()

	blockWriter := bufio.NewWriter(fi)

	// build all blocks first before writing to disk
	// TODO: if the height range is too high, consider streaming json writing for each block
	blocks, err := finder.GetByHeightRange(startHeight, endHeight)
	if err != nil {
		return err
	}

	log.Info().Msgf("exporting transactions for %v blocks", endHeight-startHeight+1)

	// converting all blocks into json
	jsonData, err := json.Marshal(blocks)
	if err != nil {
		return fmt.Errorf("could not marshal JSON: %w", err)
	}

	// writing to disk
	err = writeJSONTo(blockWriter, jsonData)
	if err != nil {
		return fmt.Errorf("could not write json to %v: %w", outputDir, err)
	}

	err = blockWriter.Flush()
	if err != nil {
		return fmt.Errorf("fail to flush block data: %w", err)
	}

	log.Info().Msgf("successfully exported transaction data to %v", outputFile)

	return nil
}
