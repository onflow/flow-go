package link_reporter

import (
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
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
	Use:   "report-links",
	Short: "reports links",
	Run:   run,
}

func init() {

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

	var payloads []*ledger.Payload
	var err error

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

	mr, err := migrations.NewInterpreterMigrationRuntime(
		registersByAccount,
		chainID,
		migrations.InterpreterMigrationRuntimeConfig{},
	)
	if err != nil {
		panic(err)
	}

	err = registersByAccount.ForEachAccount(func(accountRegisters *registers.AccountRegisters) error {

		address := common.MustBytesToAddress([]byte(accountRegisters.Owner()))

		log.Info().Msgf("account: %s", address)

		publicStorage := mr.Storage.GetStorageMap(
			address,
			common.PathDomainPublic.Identifier(),
			false,
		)

		iterator := publicStorage.Iterator(nil)
		for {
			k, v := iterator.Next()

			if k == nil || v == nil {
				break
			}

			key, ok := k.(interpreter.StringAtreeValue)
			if !ok {
				log.Fatal().Msgf("unexpected key type: %T", k)
			}

			identifier := string(key)

			value := interpreter.MustConvertUnmeteredStoredValue(v)

			switch link := value.(type) {
			case interpreter.PathLinkValue:
				targetPath := link.TargetPath
				linkType := link.Type

				log.Info().Msgf("%s: %s -> %s = %s", address, identifier, targetPath, linkType.ID())

			case interpreter.AccountLinkValue:
				// ignore
				continue
			default:
				log.Fatal().Msgf("unexpected value type: %T", value)

			}
		}
		return nil
	})
	if err != nil {
		log.Fatal().Err(err)
	}
}
