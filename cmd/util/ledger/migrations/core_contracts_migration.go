package migrations

import (
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/rs/zerolog"

	coreContracts "github.com/onflow/flow-core-contracts/lib/go/contracts"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

type ContractLocation struct {
	address, contractName string
}

type CoreContractsMigration struct {
	Log       zerolog.Logger
	Chain     flow.Chain
	OutputDir string
	Updates   map[ContractLocation]struct{}
}

type networkAddresses struct {
	fungibleToken     string
	flowToken         string
	contractAudits    string
	serviceAccount    string
	fees              string
	storageFees       string
	qc                string
	epoch             string
	dkg               string
	stakingTable      string
	lockedTokens      string
	stakingProxy      string
	stakingCollection string
}

func coreContractAddresses(chain flow.Chain) networkAddresses {
	switch chain.ChainID() {
	case flow.Testnet:
		return networkAddresses{
			fungibleToken:     "9a0766d93b6608b7",
			flowToken:         "7e60df042a9c0868",
			contractAudits:    "8c5303eaa26202d6",
			serviceAccount:    "8c5303eaa26202d6",
			storageFees:       "8c5303eaa26202d6",
			fees:              "912d5440f7e3769e",
			qc:                "9eca2b38b18b5dfe",
			epoch:             "9eca2b38b18b5dfe",
			dkg:               "9eca2b38b18b5dfe",
			stakingTable:      "9eca2b38b18b5dfe",
			lockedTokens:      "95e019a17d0e23d7",
			stakingCollection: "95e019a17d0e23d7",
			stakingProxy:      "7aad92e5a0715d21",
		}
	case flow.Mainnet:
		return networkAddresses{
			fungibleToken:     "f233dcee88fe0abe",
			flowToken:         "1654653399040a61",
			contractAudits:    "e467b9dd11fa00df",
			serviceAccount:    "e467b9dd11fa00df",
			storageFees:       "e467b9dd11fa00df",
			fees:              "f919ee77447b7497",
			qc:                "8624b52f9ddcd04a",
			epoch:             "8624b52f9ddcd04a",
			dkg:               "8624b52f9ddcd04a",
			stakingTable:      "8624b52f9ddcd04a",
			lockedTokens:      "8d0e87b65159ae63",
			stakingCollection: "8d0e87b65159ae63",
			stakingProxy:      "62430cf28c26d095",
		}
	default:
		panic(fmt.Errorf("unsupported chain: %s", chain))
	}
}

func coreContractCodes(addresses networkAddresses) map[string]map[string][]byte {
	codes := map[string]map[string][]byte{}

	addCode := func(address, name string, code []byte) {
		addressCodes, ok := codes[address]
		if !ok {
			addressCodes = map[string][]byte{}
			codes[address] = addressCodes
		}
		addressCodes[name] = code
	}

	addCode(
		addresses.serviceAccount,
		"FlowServiceAccount",
		coreContracts.FlowServiceAccount(
			// fungibleTokenAddress:
			addresses.fungibleToken,
			// flowTokenAddress:
			addresses.flowToken,
			// flowFeesAddress:
			addresses.fees,
			// storageFeesAddress:
			addresses.storageFees,
		),
	)
	addCode(
		addresses.stakingTable,
		"FlowIDTableStaking",
		coreContracts.FlowIDTableStaking(
			// fungibleTokenAddress:
			addresses.fungibleToken,
			// flowTokenAddress:
			addresses.flowToken,
			// flowFeesAddress:
			addresses.fees,
			// latest:
			true,
		),
	)
	addCode(
		addresses.stakingCollection,
		"FlowStakingCollection",
		coreContracts.FlowStakingCollection(
			// fungibleTokenAddress:
			addresses.fungibleToken,
			// flowTokenAddress:
			addresses.flowToken,
			// idTableAddress:
			addresses.stakingTable,
			// stakingProxyAddress:
			addresses.stakingProxy,
			// lockedTokensAddress:
			addresses.lockedTokens,
			// storageFeesAddress:
			addresses.storageFees,
			// qcAddress:
			addresses.qc,
			// dkgAddress:
			addresses.dkg,
			// epochAddress:
			addresses.epoch,
		),
	)
	addCode(
		addresses.storageFees,
		"FlowStorageFees",
		coreContracts.FlowStorageFees(
			// fungibleTokenAddress:
			addresses.fungibleToken,
			// flowTokenAddress:
			addresses.flowToken,
		),
	)
	addCode(
		addresses.qc,
		"FlowClusterQC",
		coreContracts.FlowQC(),
	)

	//// Other contracts which were not updated:

	//addCode(
	//	addresses.contractAudits,
	//	"FlowContractAudits",
	//	coreContracts.FlowContractAudits(),
	//)
	//addCode(
	//	addresses.fees,
	//	"FlowFees",
	//	coreContracts.FlowFees(
	//		// fungibleTokenAddress:
	//		addresses.fungibleToken,
	//		// flowTokenAddress:
	//		addresses.flowToken,
	//	),
	//)
	//addCode(
	//	addresses.dkg,
	//	"FlowDKG",
	//	coreContracts.FlowDKG(),
	//)
	//addCode(
	//	addresses.epoch,
	//	"FlowEpoch",
	//	coreContracts.FlowEpoch(
	//		// fungibleTokenAddress:
	//		addresses.fungibleToken,
	//		// flowTokenAddress:
	//		addresses.flowToken,
	//		// idTableAddress:
	//		addresses.stakingTable,
	//		// qcAddress:
	//		addresses.qc,
	//		// dkgAddress:
	//		addresses.dkg,
	//		// flowFeesAddress:
	//		addresses.fees,
	//	),
	//)
	//addCode(
	//	addresses.lockedTokens,
	//	"LockedTokens",
	//	coreContracts.FlowLockedTokens(
	//		// fungibleTokenAddress:
	//		addresses.fungibleToken,
	//		// flowTokenAddress:
	//		addresses.flowToken,
	//		// idTableAddress:
	//		addresses.stakingTable,
	//		// stakingProxyAddress:
	//		addresses.stakingProxy,
	//		// storageFeesAddress:
	//		addresses.storageFees,
	//	),
	//)
	//
	//addCode(
	//	addresses.flowToken,
	//	"FlowToken",
	//	coreContracts.FlowToken(
	//		// fungibleTokenAddress:
	//		addresses.fungibleToken,
	//	),
	//)
	//addCode(
	//	addresses.stakingProxy,
	//	"StakingProxy",
	//	coreContracts.FlowStakingProxy(),
	//)

	return codes
}

const codeKeyPrefix = "code."

func (m *CoreContractsMigration) path() string {
	filename := fmt.Sprintf(
		"core_contracts_migration_%d.csv",
		int32(time.Now().Unix()),
	)
	return path.Join(m.OutputDir, filename)
}

func (m *CoreContractsMigration) Migrate(payloads []ledger.Payload) ([]ledger.Payload, error) {

	m.Updates = map[ContractLocation]struct{}{}

	csvPath := m.path()

	m.Log.Info().Msgf("Running Core Contracts migration. Saving output to %s.", csvPath)

	f, err := os.Create(csvPath)
	if err != nil {
		return nil, err
	}

	defer func() {
		err = f.Close()
		if err != nil {
			panic(err)
		}
	}()

	csvWriter := csv.NewWriter(f)
	defer csvWriter.Flush()

	err = csvWriter.Write([]string{"address", "contractName", "old", "new"})
	if err != nil {
		panic(err)
	}

	addresses := coreContractAddresses(m.Chain)
	codes := coreContractCodes(addresses)

	for i, payload := range payloads {
		key := string(payload.Key.KeyParts[2].Value)
		if !strings.HasPrefix(key, codeKeyPrefix) {
			continue
		}

		owner := hex.EncodeToString(payload.Key.KeyParts[0].Value)

		addressCodes, ok := codes[owner]
		if !ok {
			continue
		}

		contractName := strings.TrimPrefix(key, codeKeyPrefix)

		newCode, ok := addressCodes[contractName]
		if !ok {
			m.Log.Warn().Msgf("unknown contract: %s", contractName)
			continue
		}

		oldCode := payload.Value
		payloads[i].Value = newCode

		m.Log.Info().Msgf("Updated core contract: %s.%s", owner, contractName)

		err = csvWriter.Write([]string{
			owner,
			contractName,
			string(oldCode),
			string(newCode),
		})
		if err != nil {
			panic(err)
		}

		location := ContractLocation{
			address:      owner,
			contractName: contractName,
		}

		m.Updates[location] = struct{}{}
	}

	// Validate all contracts were updated

	for address, addressCodes := range codes {
		for contractName := range addressCodes {
			location := ContractLocation{
				address:      address,
				contractName: contractName,
			}
			if _, ok := m.Updates[location]; !ok {
				m.Log.Warn().Msgf("Core Contract was not found and not updated: %s.%s", address, contractName)
			}
		}
	}

	m.Log.Info().Msg("Core Contracts update complete.")

	return payloads, nil
}
