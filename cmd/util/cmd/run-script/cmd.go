package run_script

import (
	"io"
	"os"

	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/engine/execution/computation"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

var (
	flagPayloads        string
	flagState           string
	flagStateCommitment string
	flagChain           string
)

var Cmd = &cobra.Command{
	Use:   "run-script",
	Short: "run a script against the execution state",
	Run:   run,
}

func init() {

	// Input 1

	Cmd.Flags().StringVar(
		&flagPayloads,
		"payloads",
		"",
		"Input payload file name",
	)

	Cmd.Flags().StringVar(
		&flagState,
		"state",
		"",
		"Input state file name",
	)
	Cmd.Flags().StringVar(
		&flagStateCommitment,
		"state-commitment",
		"",
		"Input state commitment",
	)

	Cmd.Flags().StringVar(
		&flagChain,
		"chain",
		"",
		"Chain name",
	)
	_ = Cmd.MarkFlagRequired("chain")
}

func run(*cobra.Command, []string) {

	if flagPayloads == "" && flagState == "" {
		log.Fatal().Msg("Either --payloads or --state must be provided")
	} else if flagPayloads != "" && flagState != "" {
		log.Fatal().Msg("Only one of --payloads or --state must be provided")
	}
	if flagState != "" && flagStateCommitment == "" {
		log.Fatal().Msg("--state-commitment must be provided when --state is provided")
	}

	chainID := flow.ChainID(flagChain)
	// Validate chain ID
	_ = chainID.Chain()

	code, err := io.ReadAll(os.Stdin)
	if err != nil {
		log.Fatal().Msgf("failed to read script: %s", err)
	}

	var payloads []*ledger.Payload

	if flagPayloads != "" {
		_, payloads, err = util.ReadPayloadFile(log.Logger, flagPayloads)
	} else {
		log.Info().Msg("reading trie")

		stateCommitment := util.ParseStateCommitment(flagStateCommitment)
		payloads, err = util.ReadTrie(flagState, stateCommitment)
	}
	if err != nil {
		log.Fatal().Err(err).Msg("failed to read payloads")
	}

	log.Info().Msgf("creating registers from payloads (%d)", len(payloads))

	registersByAccount, err := registers.NewByAccountFromPayloads(payloads)
	if err != nil {
		log.Fatal().Err(err)
	}
	log.Info().Msgf(
		"created %d registers from payloads (%d accounts)",
		registersByAccount.Count(),
		registersByAccount.AccountCount(),
	)

	options := computation.DefaultFVMOptions(chainID, false, false)
	options = append(
		options,
		fvm.WithContractDeploymentRestricted(false),
		fvm.WithContractRemovalRestricted(false),
		fvm.WithAuthorizationChecksEnabled(false),
		fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
		fvm.WithTransactionFeesEnabled(false),
	)
	ctx := fvm.NewContext(options...)

	storageSnapshot := registers.StorageSnapshot{
		Registers: registersByAccount,
	}

	vm := fvm.NewVirtualMachine()

	_, res, err := vm.Run(
		ctx,
		fvm.Script(code),
		storageSnapshot,
	)
	if err != nil {
		log.Fatal().Msgf("failed to run script: %s", err)
	}

	if res.Err != nil {
		log.Fatal().Msgf("script failed: %s", res.Err)
	}

	encoded, err := jsoncdc.Encode(res.Value)
	if err != nil {
		log.Fatal().Msgf("failed to encode result: %s", err)
	}

	_, _ = os.Stdout.Write(encoded)
}
