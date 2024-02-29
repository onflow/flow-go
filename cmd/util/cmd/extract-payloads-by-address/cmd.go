package extractpayloads

import (
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/cadence/runtime/common"

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

	addresses, err := parseAddresses(strings.Split(flagAddresses, ","))
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

	numOfPayloadWritten, err := util.CreatePayloadFile(log.Logger, flagOutputPayloadFileName, payloads, addresses, inputPayloadsFromPartialState)
	if err != nil {
		log.Fatal().Err(err)
	}

	log.Info().Msgf(
		"extracted %d payloads with address %v from %s to %s",
		numOfPayloadWritten,
		addresses,
		flagInputPayloadFileName,
		flagOutputPayloadFileName,
	)
}

func parseAddresses(hexAddresses []string) ([]common.Address, error) {
	if len(hexAddresses) == 0 {
		return nil, fmt.Errorf("at least one address must be provided")
	}

	addresses := make([]common.Address, len(hexAddresses))
	for i, hexAddr := range hexAddresses {
		b, err := hex.DecodeString(strings.TrimSpace(hexAddr))
		if err != nil {
			return nil, fmt.Errorf("address is not hex encoded %s: %w", strings.TrimSpace(hexAddr), err)
		}

		addr, err := common.BytesToAddress(b)
		if err != nil {
			return nil, fmt.Errorf("cannot decode address %x", b)
		}

		addresses[i] = addr
	}

	return addresses, nil
}
