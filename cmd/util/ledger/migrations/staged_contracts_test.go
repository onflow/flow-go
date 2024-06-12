package migrations

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/model/flow"
)

func TestStagedContractsChecking(t *testing.T) {

	contractCodes := map[common.Location][]byte{}

	chainID := flow.Testnet

	// Get core contracts

	options := SystemContractsMigrationOptions{
		StagedContractsMigrationOptions: StagedContractsMigrationOptions{
			ChainID: chainID,
		},
		EVM:    EVMContractChangeUpdate,
		Burner: BurnerContractChangeUpdate,
	}

	for _, change := range SystemContractChanges(options.ChainID, options) {
		location := common.AddressLocation{
			Name:    change.Name,
			Address: change.Address,
		}
		contractCodes[location] = change.Code
	}

	// Get staged contracts
	// These were extracted using the script that can be found at the end of this file.

	// TODO: Replace with proper path
	bytes, err := os.ReadFile("/Users/supunsetunga/work/projects/staged-contracts.json")
	require.NoError(t, err)

	// There are cadence unicode characters in comments in some places.
	// JSON marshaller fail to parse them. So just get rid of them.
	// Comments are not important for the type checker.
	bytes = []byte(strings.ReplaceAll(string(bytes), "\\\\u", "u"))
	bytes = []byte(strings.ReplaceAll(string(bytes), "\\u", "u"))

	stagedContracts := map[string]string{}
	err = json.Unmarshal(bytes, &stagedContracts)
	require.NoError(t, err)

	for key, code := range stagedContracts {
		keyParts := strings.Split(key, ".")
		require.Len(t, keyParts, 2)

		address, err := common.HexToAddress(keyParts[0])
		if err != nil {
			require.NoError(t, err)
		}

		location := common.AddressLocation{
			Name:    keyParts[1],
			Address: address,
		}

		contractCodes[location] = []byte(code)
	}

	// Run the checker

	registersByAccount := registers.NewByAccount()
	mr, err := NewInterpreterMigrationRuntime(
		registersByAccount,
		chainID,
		InterpreterMigrationRuntimeConfig{
			GetCode: func(location common.AddressLocation) ([]byte, error) {
				return contractCodes[location], nil
			},
			GetContractNames:         nil,
			GetOrLoadProgram:         nil,
			GetOrLoadProgramListener: nil,
		},
	)

	errors := map[common.Location]error{}

	const getAndSetProgram = true
	for location, code := range contractCodes {
		_, err := mr.ContractAdditionHandler.ParseAndCheckProgram(
			code,
			location,
			getAndSetProgram,
		)
		if err != nil {
			errors[location] = err
		}
	}

	fmt.Println("Total:", len(contractCodes))
	fmt.Println("Errors:", len(errors))
	fmt.Println("Errors:", errors)
}

/*
Script for extracting all the staged contracts from testnet (0x2ceae959ed1a7e7a)
--------------------------------------------------------------------------------

import MigrationContractStaging from 0x2ceae959ed1a7e7a

access(all) fun main(): {String: String} {
    var hosts = MigrationContractStaging.getAllStagedContractHosts()

    var contracts: {String: String} = {}

    for host in hosts {
        var contractsForAddress = MigrationContractStaging.getAllStagedContractCode(forAddress: host)
        for name in contractsForAddress.keys {
            var code = contractsForAddress[name]
            var key = host.toString().concat(".").concat(name)
            contracts[key] = code
        }
    }

    return contracts
}
*/
