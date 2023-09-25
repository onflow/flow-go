package flex

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/onflow/atree"

	env "github.com/onflow/flow-go/fvm/flex/environment"
	"github.com/onflow/flow-go/fvm/flex/models"
	"github.com/onflow/flow-go/fvm/flex/storage"
)

type foa struct {
	fch     FlexContractHandler
	address *models.FlexAddress
	// TODO inject gas meter
}

var _ models.FlowOwnedAccount = &foa{}

func newFOA(fch FlexContractHandler, addr *models.FlexAddress) *foa {
	return &foa{
		fch:     fch,
		address: addr,
	}
}

// Address returns the flex address associated with the FOA account
func (f *foa) Address() *models.FlexAddress {
	return f.address
}

// Deposit deposits the token from the given vault into the Flex main vault
// and update the FOA balance with the new amount
func (f *foa) Deposit(v models.FLOWTokenVault) {
	config := env.NewFlexConfig(
		env.WithBlockNumber(env.BlockNumberForEVMRules))
	env, err := env.NewEnvironment(config, f.fch.db)
	// TODO improve this
	if err != nil {
		panic(err)
	}

	err = env.MintTo(v.Balance().ToAttoFlow(), f.address.ToCommon())
	if err != nil {
		panic(err)
	}
}

// Withdraw deducts the balance from the FOA account and
// withdraw and return flow token from the Flex main vault.
func (f *foa) Withdraw(b models.Balance) models.FLOWTokenVault {
	config := env.NewFlexConfig(
		env.WithBlockNumber(env.BlockNumberForEVMRules))
	env, err := env.NewEnvironment(config, f.fch.db)
	// TODO improve this
	if err != nil {
		panic(err)
	}
	err = env.WithdrawFrom(b.ToAttoFlow(), f.address.ToCommon())
	if err != nil {
		panic(err)
	}
	// TODO: construct a new FlowTokenVault
	return nil
}

// Deploy deploys a contract to the Flex environment
// the new deployed contract would be at the returned address and
// the contract data is not controlled by the FOA accounts
func (f *foa) Deploy(code models.Code, gaslimit models.GasLimit, balance models.Balance) *models.FlexAddress {
	config := env.NewFlexConfig(
		env.WithBlockNumber(env.BlockNumberForEVMRules))
	env, err := env.NewEnvironment(config, f.fch.db)
	// TODO improve this
	if err != nil {
		panic(err)
	}
	// TODO check gas limit against what has been left on the transaction side
	env.Deploy(f.address.ToCommon(), code, uint64(gaslimit), balance.ToAttoFlow())
	if env.Result.Failed {
		// TODO panic with a handlable error
		panic("deploy failed")
	}
	return models.NewFlexAddress(env.Result.DeployedContractAddress)
}

// Call calls a smart contract function with the given data
// it would limit the gas used according to the limit provided
// given it doesn't goes beyond what Flow transaction allows.
// the balance would be deducted from the OFA account and would be transferred to the target address
// contract data is not controlled by the FOA accounts
func (f *foa) Call(to models.FlexAddress, data models.Data, gaslimit models.GasLimit, balance models.Balance) models.Data {
	config := env.NewFlexConfig(
		env.WithBlockNumber(env.BlockNumberForEVMRules))
	env, err := env.NewEnvironment(config, f.fch.db)
	// TODO improve this
	if err != nil {
		panic(err)
	}
	// TODO check gas limit against what has been left on the transaction side
	env.Call(f.address.ToCommon(), to.ToCommon(), data, uint64(gaslimit), balance.ToAttoFlow())
	if env.Result.Failed {
		// TODO panic with a handlable error
		panic("call failed")
	}
	return env.Result.RetValue
}

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
	config := env.NewFlexConfig(
		env.WithBlockNumber(env.BlockNumberForEVMRules))
	env, err := env.NewEnvironment(config, h.db)
	if err != nil {
		panic(err)
	}
	// TODO support passing values to be minted
	addr, err := env.AllocateAddressAndMintTo(big.NewInt(0))
	if err != nil {
		panic(err)
	}

	return newFOA(h, addr)
}

func (h FlexContractHandler) LastExecutedBlock() *models.FlexBlock {
	block, err := h.db.GetLatestBlock()
	if err != nil {
		panic(err)
	}
	return block
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
