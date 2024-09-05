package link_reporter

import (
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

var (
	flagOutputDirectory string
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
		&flagOutputDirectory,
		"output-directory",
		"",
		"Output directory",
	)

	Cmd.Flags().StringVar(
		&flagChain,
		"chain",
		"",
		"Chain name",
	)
	_ = Cmd.MarkFlagRequired("chain")
}

type LinkDataPoint struct {
	Address    string `json:"address"`
	Identifier string `json:"identifier"`
	TargetPath string `json:"targetPath"`
	LinkTypeID string `json:"linkType"`
}

type CapDataPoint struct {
	Address    string `json:"address"`
	Identifier string `json:"identifier"`
	BorrowType string `json:"borrowType"`
	ID         string `json:"id"`
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

	reporter := reporters.NewReportFileWriterFactory(flagOutputDirectory, log.Logger)
	linkReporter := reporter.ReportWriter("link-reporter")
	defer linkReporter.Close()

	capabilityReporter := reporter.ReportWriter("capability-reporter")
	defer capabilityReporter.Close()

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

		// log.Info().Msgf("account: %s", address)

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

			switch obj := value.(type) {
			case interpreter.PathLinkValue:
				targetPath := obj.TargetPath
				linkType := obj.Type
				linkTypeID := linkType.ID()

				linkReporter.Write(LinkDataPoint{
					Address:    address.String(),
					Identifier: identifier,
					TargetPath: targetPath.String(),
					LinkTypeID: string(linkTypeID),
				})

				// temp test after the first one break
				break

			default:
				// ignore
				continue
			}
		}
		return nil
	})
	if err != nil {
		log.Fatal().Err(err)
	}
}
