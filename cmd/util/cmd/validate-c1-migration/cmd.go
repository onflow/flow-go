package validate_c1_migration

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"os"

	"github.com/onflow/cadence/runtime/common"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/model/flow"
)

var (
	flagChain string
	flagGzip  bool
)

var Cmd = &cobra.Command{
	Use:   "validate-c1-migration",
	Short: "validate the Cadence 1.0 migration",
}

var systemContractsCheckingCmd = &cobra.Command{
	Use:   "system-contracts-checking [report]",
	Short: "validate checking of all system contracts",
	Args:  cobra.ExactArgs(1),
	Run:   runSystemContractsChecking,
}

func init() {
	Cmd.PersistentFlags().StringVar(&flagChain, "chain", "", "Chain name")
	_ = Cmd.MarkFlagRequired("chain")

	Cmd.AddCommand(systemContractsCheckingCmd)

	systemContractsCheckingCmd.Flags().BoolVar(&flagGzip, "gzip", false, "report is gzipped")
}

func runSystemContractsChecking(_ *cobra.Command, args []string) {
	chainID := flow.ChainID(flagChain)
	_ = chainID.Chain()

	report := args[0]

	log.Info().Msgf("Validating checking of system contracts for chain %s, report %s ...", chainID, report)

	burnerContractChange, evmContractChange := migrations.SystemContractChangesForChainID(chainID)

	systemContractChanges := migrations.SystemContractChanges(
		chainID,
		migrations.SystemContractsMigrationOptions{
			StagedContractsMigrationOptions: migrations.StagedContractsMigrationOptions{
				ChainID: chainID,
			},
			EVM:    evmContractChange,
			Burner: burnerContractChange,
		},
	)

	systemContractLocations := make(map[common.AddressLocation]struct{}, len(systemContractChanges))
	for _, change := range systemContractChanges {
		systemContractLocations[change.AddressLocation()] = struct{}{}
	}

	var reader io.Reader
	var err error

	reader, err = os.Open(report)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to open report")
	}

	if flagGzip {
		reader, err = gzip.NewReader(reader)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to open gzip reader")
		}
	}

	var results []migrations.ContractCheckingResultJSON
	err = json.NewDecoder(reader).Decode(&results)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to decode report")
	}

	log.Info().Msgf("Read %d checking results", len(results))

	var failures bool

	for _, result := range results {
		if result.ContractCheckingSuccess != nil {
			continue
		}

		failure := result.ContractCheckingFailure

		location := common.AddressLocation{
			Address: failure.AccountAddress,
			Name:    failure.ContractName,
		}

		_, ok := systemContractLocations[location]
		if !ok {
			continue
		}

		log.Error().Msgf("Failed to check system contract %s:\n%s", location, failure.Error)
		failures = true
	}

	if !failures {
		log.Info().Msg("All system contracts checked successfully")
	}
}
