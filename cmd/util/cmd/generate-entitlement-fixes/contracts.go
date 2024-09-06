package generate_entitlement_fixes

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/onflow/cadence/runtime/common"
	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/model/flow"
)

type AddressContract struct {
	Location common.AddressLocation
	Code     []byte
}

func gatherContractsFromRegisters(registersByAccount *registers.ByAccount) ([]AddressContract, error) {
	log.Info().Msg("Gathering contracts ...")

	contracts := make([]AddressContract, 0, contractCountEstimate)

	err := registersByAccount.ForEachAccount(func(accountRegisters *registers.AccountRegisters) error {
		owner := accountRegisters.Owner()

		encodedContractNames, err := accountRegisters.Get(owner, flow.ContractNamesKey)
		if err != nil {
			return err
		}

		contractNames, err := environment.DecodeContractNames(encodedContractNames)
		if err != nil {
			return err
		}

		for _, contractName := range contractNames {

			contractKey := flow.ContractKey(contractName)

			code, err := accountRegisters.Get(owner, contractKey)
			if err != nil {
				return err
			}

			if len(bytes.TrimSpace(code)) == 0 {
				continue
			}

			address := common.Address([]byte(owner))
			location := common.AddressLocation{
				Address: address,
				Name:    contractName,
			}

			contracts = append(
				contracts,
				AddressContract{
					Location: location,
					Code:     code,
				},
			)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get contracts of accounts: %w", err)
	}

	sort.Slice(contracts, func(i, j int) bool {
		a := contracts[i]
		b := contracts[j]
		return a.Location.ID() < b.Location.ID()
	})

	log.Info().Msgf("Gathered all contracts (%d)", len(contracts))

	return contracts, nil
}
