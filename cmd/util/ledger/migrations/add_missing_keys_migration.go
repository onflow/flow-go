package migrations

import (
	_ "embed"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-core-contracts/lib/go/templates"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

//go:embed FlowFees.cdc
var flowFeesContract string

// AddMissingKeysAndUpgradeContractsMigration is a temporary migration that adds the missing keys to the core contract accounts
// and upgrades the contracts to the latest version.
// This migration is only needed for the sandboxnet but is written to also work on benchnet, so it can be tested.
// context: https://dapperlabs.slack.com/archives/C015G65HR2P/p1671218228441159
func AddMissingKeysAndUpgradeContractsMigration(chain flow.Chain, logger zerolog.Logger) func(payloads []ledger.Payload) ([]ledger.Payload, error) {
	return func(payloads []ledger.Payload) ([]ledger.Payload, error) {
		return addMissingKeysAndUpgradeContractsMigration(
			chain,
			payloads,
			logger.With().Str("migration", "add-missing-keys-and-upgrade-contracts").Logger())
	}
}

func addMissingKeysAndUpgradeContractsMigration(
	chain flow.Chain,
	payloads []ledger.Payload,
	logger zerolog.Logger,
) ([]ledger.Payload, error) {

	// only run this migration on sandboxnet and benchnet
	switch chain {
	case flow.Sandboxnet.Chain():
		logger.Info().Msg("running migration for sandboxnet")
	case flow.Benchnet.Chain():
		logger.Info().Msg("running migration for benchnet")
	default:
		logger.Error().
			Msg("migration not supported for this chain")
		return nil, fmt.Errorf("migration not supported for chain %s", chain)
	}

	view := NewView(payloads)
	txState := state.NewTransactionState(view, state.DefaultParameters())
	accounts := environment.NewAccounts(txState)

	// get the key that we want to copy from the service account
	serviceAddress := chain.ServiceAddress()
	ok, err := accounts.Exists(serviceAddress)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("service account does not exist: %s", serviceAddress)
	}

	keys, err := accounts.GetPublicKeys(serviceAddress)
	if err != nil {
		return nil, err
	}
	if len(keys) != 1 {
		return nil, fmt.Errorf("expected 1 key for service account, got: %d", len(keys))
	}
	// this is the key we want to copy to other addresses
	key := keys[0]

	// core contract accounts with missing keys
	accountsWithMissingKeys := []flow.Address{
		fvm.FlowTokenAddress(chain),
		fvm.FlowFeesAddress(chain),
		fvm.FungibleTokenAddress(chain),
	}
	for _, address := range accountsWithMissingKeys {
		err = appendKeyForAccount(accounts, address, key, logger)
		if err != nil {
			return nil, err
		}
	}

	// upgrade the FlowFees contract
	logger.Info().
		Msg("upgrade the FlowFees contract")

	// replace the imports with actual addresses:
	// import FungibleToken from 0xFUNGIBLETOKENADDRESS
	// import FlowToken from 0xFLOWTOKENADDRESS
	// import FlowStorageFees from 0xFLOWSTORAGEFEESADDRESS
	flowFeesContractWithReplacedAddresses := templates.ReplaceAddresses(
		flowFeesContract,
		templates.Environment{
			FungibleTokenAddress: fvm.FungibleTokenAddress(chain).Hex(),
			FlowTokenAddress:     fvm.FlowTokenAddress(chain).Hex(),
			StorageFeesAddress:   chain.ServiceAddress().Hex(),
		},
	)

	err = accounts.SetContract(
		"FlowFees",
		fvm.FlowFeesAddress(chain),
		[]byte(flowFeesContractWithReplacedAddresses),
	)

	if err != nil {
		return nil, err
	}

	return view.Payloads(), nil
}

func appendKeyForAccount(
	accounts *environment.StatefulAccounts,
	address flow.Address,
	accountKey flow.AccountPublicKey,
	logger zerolog.Logger,
) error {
	ok, err := accounts.Exists(address)
	if err != nil {
		return err
	}
	if ok {
		err = accounts.AppendPublicKey(address, accountKey)
		logger.Info().
			Str("address", address.String()).
			Msg("added missing key to account")
		if err != nil {
			return err
		}
	} else {
		return fmt.Errorf("account does not exist: %s", address)
	}
	return nil
}
