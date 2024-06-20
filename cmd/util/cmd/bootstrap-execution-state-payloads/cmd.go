package addresses

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

var (
	flagChain                 string
	flagOutputPayloadFileName string
)

var Cmd = &cobra.Command{
	Use:   "bootstrap-execution-state-payloads",
	Short: "generate payloads for execution state of bootstrapped chain",
	Run:   run,
}

func init() {
	Cmd.Flags().StringVar(&flagChain, "chain", "", "Chain name")
	_ = Cmd.MarkFlagRequired("chain")

	Cmd.Flags().StringVar(
		&flagOutputPayloadFileName,
		"output-filename",
		"",
		"Output payload file name")
	_ = Cmd.MarkFlagRequired("output-filename")

}

func run(*cobra.Command, []string) {

	chain := flow.ChainID(flagChain).Chain()

	log.Info().Msgf("creating payloads for chain %s", chain)

	ctx := fvm.NewContext(
		fvm.WithChain(chain),
	)

	vm := fvm.NewVirtualMachine()

	storageSnapshot := snapshot.MapStorageSnapshot{}

	bootstrapProcedure := fvm.Bootstrap(
		unittest.ServiceAccountPublicKey,
		fvm.WithInitialTokenSupply(unittest.GenesisTokenSupply),
	)

	executionSnapshot, _, err := vm.Run(
		ctx,
		bootstrapProcedure,
		storageSnapshot,
	)
	if err != nil {
		log.Fatal().Err(err)
	}

	payloads := make([]*ledger.Payload, 0, len(executionSnapshot.WriteSet))

	for registerID, registerValue := range executionSnapshot.WriteSet {
		payloadKey := convert.RegisterIDToLedgerKey(registerID)
		payload := ledger.NewPayload(payloadKey, registerValue)
		payloads = append(payloads, payload)
	}

	log.Info().Msgf("writing payloads to %s", flagOutputPayloadFileName)

	numOfPayloadWritten, err := util.CreatePayloadFile(
		log.Logger,
		flagOutputPayloadFileName,
		payloads,
		nil,
		false,
	)
	if err != nil {
		log.Fatal().Err(err)
	}

	log.Info().Msgf("wrote %d payloads", numOfPayloadWritten)
}
