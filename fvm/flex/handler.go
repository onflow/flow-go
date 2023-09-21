package flex

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/onflow/atree"

	env "github.com/onflow/flow-go/fvm/flex/environment"
	"github.com/onflow/flow-go/fvm/flex/models"
	"github.com/onflow/flow-go/fvm/flex/storage"
)

type FOAHandler struct {
	fch     FlexContractHandler
	address *models.FlexAddress
	// TODO inject gas meter
}

// Address returns the flex address associated with the FOA account
func (h *FOAHandler) Address() *models.FlexAddress {
	return h.address
}

// Deposit deposits the token from the given vault into the Flex main vault
// and update the FOA balance with the new amount
func (h *FOAHandler) Deposit(models.FLOWTokenVault) {
	panic("not implemented")
}

// Withdraw deducts the balance from the FOA account and
// withdraw and return flow token from the Flex main vault.
func (h *FOAHandler) Withdraw(models.Balance) models.FLOWTokenVault {
	panic("not implemented")
}

// Deploy deploys a contract to the Flex environment
// the new deployed contract would be at the returned address and
// the contract data is not controlled by the FOA accounts
func (h *FOAHandler) Deploy(code models.Code, gaslimit models.GasLimit, balance models.Balance) models.FlexAddress {
	config := env.NewFlexConfig(
		env.WithBlockNumber(env.BlockNumberForEVMRules))
	env, err := env.NewEnvironment(config, h.fch.db)
	// TODO improve this
	if err != nil {
		panic(err)
	}
	// TODO check gas limit against what has been left on the transaction side
	env.Deploy(h.address.ToCommon(), code, uint64(gaslimit), balance.InAttoFlow())
	if env.Result.Failed {
		// TODO panic with a handlable error
		panic("deploy failed")
	}
	return models.FlexAddress(env.Result.DeployedContractAddress)
}

// Call calls a smart contract function with the given data
// it would limit the gas used according to the limit provided
// given it doesn't goes beyond what Flow transaction allows.
// the balance would be deducted from the OFA account and would be transferred to the target address
// contract data is not controlled by the FOA accounts
func (h *FOAHandler) Call(to models.FlexAddress, data models.Data, gaslimit models.GasLimit, balance models.Balance) models.Data {
	config := env.NewFlexConfig(
		env.WithBlockNumber(env.BlockNumberForEVMRules))
	env, err := env.NewEnvironment(config, h.fch.db)
	// TODO improve this
	if err != nil {
		panic(err)
	}
	// TODO check gas limit against what has been left on the transaction side
	env.Call(h.address.ToCommon(), to.ToCommon(), data, uint64(gaslimit), balance.InAttoFlow())
	if env.Result.Failed {
		// TODO panic with a handlable error
		panic("call failed")
	}
	return env.Result.RetValue
}

var _ models.FlowOwnedAccount = &FOAHandler{}

type FlexContractHandler struct {
	db *storage.Database
	// TODO inject what captures how much gas has been used
}

var _ models.FlexContractHandler = &FlexContractHandler{}

func NewFlexContractHandler(ledger atree.Ledger) *FlexContractHandler {
	return &FlexContractHandler{
		db: storage.NewDatabase(ledger),
	}
}

func (h FlexContractHandler) NewFlowOwnedAccount() models.FlowOwnedAccount {
	// allocate a new address
	// TODO check for collission
	// from 20 bytes, the first 12 could be zero, the next 8 could be the output of a uuid generator (resourceID?)
	// does this leads to trie depth issue?
	panic("not implemented yet")
}

func (h FlexContractHandler) LastExecutedBlock() models.FlexBlock {
	panic("not implemented yet")
}

func (h FlexContractHandler) Run(tx []byte, coinbase models.FlexAddress) bool {
	config := env.NewFlexConfig(
		env.WithCoinbase(common.Address(coinbase)),
		env.WithBlockNumber(env.BlockNumberForEVMRules))
	env, err := env.NewEnvironment(config, h.db)
	// TODO improve this
	if err != nil {
		panic(err)
	}
	env.RunTransaction(tx)
	return env.Result.Failed
}
