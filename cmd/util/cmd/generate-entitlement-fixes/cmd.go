package generate_entitlement_fixes

import (
	"encoding/json"

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
	flagPayloads        string
	flagState           string
	flagStateCommitment string
	flagOutputDirectory string
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

const contractCountEstimate = 1000

func run(*cobra.Command, []string) {

	if flagPayloads == "" && flagState == "" {
		log.Fatal().Msg("Either --payloads or --state must be provided")
	} else if flagPayloads != "" && flagState != "" {
		log.Fatal().Msg("Only one of --payloads or --state must be provided")
	}
	if flagState != "" && flagStateCommitment == "" {
		log.Fatal().Msg("--state-commitment must be provided when --state is provided")
	}

	rwf := reporters.NewReportFileWriterFactory(flagOutputDirectory, log.Logger)

	reporter := rwf.ReportWriter("entitlement-fixes")
	defer reporter.Close()

	chainID := flow.ChainID(flagChain)
	// Validate chain ID
	_ = chainID.Chain()

	var payloads []*ledger.Payload
	var err error

	// Read payloads from payload file or checkpoint file

	if flagPayloads != "" {
		log.Info().Msgf("Reading payloads from %s", flagPayloads)

		_, payloads, err = util.ReadPayloadFile(log.Logger, flagPayloads)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to read payloads")
		}
	} else {
		log.Info().Msgf("Reading trie %s", flagStateCommitment)

		stateCommitment := util.ParseStateCommitment(flagStateCommitment)
		payloads, err = util.ReadTrie(flagState, stateCommitment)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to read state")
		}
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
		log.Fatal().Err(err)
	}

	checkContracts(registersByAccount, mr, reporter)

}

func checkContracts(
	registersByAccount *registers.ByAccount,
	mr *migrations.InterpreterMigrationRuntime,
	reporter reporters.ReportWriter,
) {
	contracts, err := gatherContractsFromRegisters(registersByAccount)
	if err != nil {
		log.Fatal().Err(err)
	}

	programs := make(map[common.Location]*interpreter.Program, contractCountEstimate)

	contractsForPrettyPrinting := make(map[common.Location][]byte, len(contracts))
	for _, contract := range contracts {
		contractsForPrettyPrinting[contract.Location] = contract.Code
	}

	log.Info().Msg("Checking contracts ...")

	for _, contract := range contracts {
		checkContract(
			contract,
			mr,
			contractsForPrettyPrinting,
			reporter,
			programs,
		)
	}

	log.Info().Msgf("Checked %d contracts ...", len(contracts))
}

func jsonEncodeAuthorization(authorization interpreter.Authorization) string {
	switch authorization {
	case interpreter.UnauthorizedAccess, interpreter.InaccessibleAccess:
		return ""
	default:
		return string(authorization.ID())
	}
}

type fixEntitlementsEntry struct {
	AccountCapabilityID
	NewAuthorization interpreter.Authorization
}

var _ json.Marshaler = fixEntitlementsEntry{}

func (e fixEntitlementsEntry) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind              string `json:"kind"`
		CapabilityAddress string `json:"capability_address"`
		CapabilityID      uint64 `json:"capability_id"`
		NewAuthorization  string `json:"new_authorization"`
	}{
		Kind:              "fix-entitlements",
		CapabilityAddress: e.Address.String(),
		CapabilityID:      e.CapabilityID,
		NewAuthorization:  jsonEncodeAuthorization(e.NewAuthorization),
	})
}
