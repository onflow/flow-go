package flex

import (
	"math"

	env "github.com/onflow/flow-go/fvm/flex/environment"
	"github.com/onflow/flow-go/fvm/flex/models"
	"github.com/onflow/flow-go/fvm/flex/storage"
	"github.com/onflow/flow-go/model/flow"
)

type FlexContractHandler struct {
	db      *storage.Database
	backend models.Backend
}

var _ models.FlexContractHandler = &FlexContractHandler{}

func NewFlexContractHandler(backend models.Backend, flexAddress flow.Address) *FlexContractHandler {
	return &FlexContractHandler{
		db:      storage.NewDatabase(backend, flexAddress),
		backend: backend,
	}
}

func (h FlexContractHandler) AllocateAddress() models.FlexAddress {
	config := env.NewFlexConfig(
		env.WithBlockNumber(env.BlockNumberForEVMRules))
	env, err := env.NewEnvironment(config, h.db)
	if err != nil {
		panic(err)
	}
	addr, err := env.AllocateAddress()
	if err != nil {
		panic(err)
	}

	return addr
}

func (h FlexContractHandler) AccountByAddress(addr models.FlexAddress, isFOA bool) models.FlexAccount {
	return newFlexAccount(h, addr, isFOA)
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
		env.WithCoinbase(coinbase.ToCommon()),
		env.WithBlockNumber(env.BlockNumberForEVMRules))
	env, err := env.NewEnvironment(config, h.db)
	// TODO improve this
	if err != nil {
		panic(err)
	}
	// TODO compute the max gas using backend
	gasLimit := uint64(math.MaxUint64)
	err = env.RunTransaction(tx, gasLimit)
	if err != nil {
		panic(err)
	}
	return !env.Result.Failed
}

func (h FlexContractHandler) getNewDefaultConfig() *env.Config {
	return env.NewFlexConfig(
		env.WithBlockNumber(env.BlockNumberForEVMRules))
}

func (h FlexContractHandler) getNewDefaultEnv() *env.Environment {
	env, err := env.NewEnvironment(h.getNewDefaultConfig(), h.db)
	h.handleError(err)
	return env
}

// TODO: properly implement this
func (h FlexContractHandler) handleError(err error) {
	if err != nil {
		panic(err)
	}
}

type flexAccount struct {
	isFOA   bool
	address models.FlexAddress
	fch     FlexContractHandler
}

// newFlexAccount creates a new flex account
func newFlexAccount(fch FlexContractHandler, addr models.FlexAddress, isFOA bool) *flexAccount {
	return &flexAccount{
		isFOA:   isFOA,
		fch:     fch,
		address: addr,
	}
}

// Address returns the flex address associated with the FOA account
func (f *flexAccount) Address() models.FlexAddress {
	return f.address
}

// Balance returns the balance of this foa
func (f *flexAccount) Balance() models.Balance {
	env := f.fch.getNewDefaultEnv()
	bl, err := env.Balance(f.address)
	if err != nil {
		panic(err)
	}
	balance, err := models.NewBalanceFromAttoFlow(bl)
	if err != nil {
		panic(err)
	}
	return balance
}

// Deposit deposits the token from the given vault into the Flex main vault
// and update the FOA balance with the new amount
func (f *flexAccount) Deposit(v *models.FLOWTokenVault) {
	env := f.fch.getNewDefaultEnv()
	err := env.MintTo(v.Balance().ToAttoFlow(), f.address.ToCommon())
	f.fch.handleError(err)
	// emit event
	// TODO pass encoded payload data
	f.fch.backend.EmitFlowEvent(models.EventFlexTokenDeposit, nil)
}

// Withdraw deducts the balance from the FOA account and
// withdraw and return flow token from the Flex main vault.
func (f *flexAccount) Withdraw(b models.Balance) *models.FLOWTokenVault {
	if !f.isFOA {
		panic(models.ErrUnAuthroizedMethodCall)
	}
	env := f.fch.getNewDefaultEnv()
	err := env.WithdrawFrom(b.ToAttoFlow(), f.address.ToCommon())
	f.fch.handleError(err)
	return models.NewFlowTokenVault(b)
}

// Deploy deploys a contract to the Flex environment
// the new deployed contract would be at the returned address and
// the contract data is not controlled by the FOA accounts
func (f *flexAccount) Deploy(code models.Code, gaslimit models.GasLimit, balance models.Balance) models.FlexAddress {
	if !f.isFOA {
		panic(models.ErrUnAuthroizedMethodCall)
	}
	env := f.fch.getNewDefaultEnv()
	// TODO check gas limit against what has been left on the transaction side
	err := env.Deploy(f.address.ToCommon(), code, uint64(gaslimit), balance.ToAttoFlow())
	f.fch.handleError(err)
	if env.Result.Failed {
		panic("deploy failed")
	}
	return models.FlexAddress(env.Result.DeployedContractAddress)
}

// Call calls a smart contract function with the given data
// it would limit the gas used according to the limit provided
// given it doesn't goes beyond what Flow transaction allows.
// the balance would be deducted from the OFA account and would be transferred to the target address
// contract data is not controlled by the FOA accounts
func (f *flexAccount) Call(to models.FlexAddress, data models.Data, gaslimit models.GasLimit, balance models.Balance) models.Data {
	if !f.isFOA {
		panic(models.ErrUnAuthroizedMethodCall)
	}
	env := f.fch.getNewDefaultEnv()
	// TODO check the gas on the backend
	// TODO check gas limit against what has been left on the transaction side
	err := env.Call(f.address.ToCommon(), to.ToCommon(), data, uint64(gaslimit), balance.ToAttoFlow())
	f.fch.handleError(err)
	if env.Result.Failed {
		panic(models.NewEVMExecutionError(env.Result.Error))
	}
	return env.Result.RetValue
}
