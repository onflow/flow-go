package extractpayloads

import (
	"os"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/common"
	"github.com/onflow/flow-go/cmd/util/ledger/util"
)

var (
	flagInputPayloadFileName  string
	flagOutputPayloadFileName string
	flagAddresses             string
)

var Cmd = &cobra.Command{
	Use:   "extract-payload-by-address",
	Short: "Read payload file and generate payload file containing payloads with specified addresses",
	Run:   run,
}

func init() {
	Cmd.Flags().StringVar(
		&flagInputPayloadFileName,
		"input-filename",
		"",
		"Input payload file name")
	_ = Cmd.MarkFlagRequired("input-filename")

	Cmd.Flags().StringVar(
		&flagOutputPayloadFileName,
		"output-filename",
		"",
		"Output payload file name")
	_ = Cmd.MarkFlagRequired("output-filename")

	Cmd.Flags().StringVar(
		&flagAddresses,
		"addresses",
		"",
		"extract payloads of addresses (comma separated hex-encoded addresses) to file specified by output-payload-filename",
	)
	_ = Cmd.MarkFlagRequired("addresses")
}

func run(*cobra.Command, []string) {

	if _, err := os.Stat(flagInputPayloadFileName); os.IsNotExist(err) {
		log.Fatal().Msgf("Input file %s doesn't exist", flagInputPayloadFileName)
	}

	if _, err := os.Stat(flagOutputPayloadFileName); os.IsExist(err) {
		log.Fatal().Msgf("Output file %s exists", flagOutputPayloadFileName)
	}

	addresses, err := common.ParseOwners(strings.Split(flagAddresses, ","))
	if err != nil {
		log.Fatal().Err(err)
	}

	log.Info().Msgf(
		"extracting payloads with address %v from %s to %s",
		addresses,
		flagInputPayloadFileName,
		flagOutputPayloadFileName,
	)

	inputPayloadsFromPartialState, payloads, err := util.ReadPayloadFile(log.Logger, flagInputPayloadFileName)
	if err != nil {
		log.Fatal().Err(err)
	}

	numOfPayloadWritten, err := util.CreatePayloadFile(
		log.Logger,
		flagOutputPayloadFileName,
		payloads,
		addresses,
		inputPayloadsFromPartialState,
	)
	if err != nil {
		log.Fatal().Err(err)
	}

	log.Info().Msgf(
		"extracted %d payloads with addresses %v from %s to %s",
		numOfPayloadWritten,
		addresses,
		flagInputPayloadFileName,
		flagOutputPayloadFileName,
	)
}
