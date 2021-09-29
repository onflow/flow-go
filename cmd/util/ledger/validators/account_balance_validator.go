package validators

import (
	"fmt"
	"runtime"
	"sync"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"

	oldFvm "github.com/onflow/flow-go/v21/fvm"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

type accountBalance struct {
	addressIndex uint64
	balance      uint64
}

type balanceCollector interface {
	CollectBalances(addressIndices chan uint64, balances chan accountBalance)
}

type newVersionCollector struct {
	wg     *sync.WaitGroup
	logger zerolog.Logger
	vm     *fvm.VirtualMachine
	ctx    fvm.Context
	view   state.View
	prog   *programs.Programs
	script []byte
}

func newNewVersionCollector(
	wg *sync.WaitGroup,
	logger zerolog.Logger,
	chain flow.Chain,
	view state.View) *newVersionCollector {
	vm := fvm.NewVirtualMachine(fvm.NewInterpreterRuntime())
	ctx := fvm.NewContext(zerolog.Nop(), fvm.WithChain(chain))
	prog := programs.NewEmptyPrograms()
	script := []byte(fmt.Sprintf(`
				import FungibleToken from 0x%s
				import FlowToken from 0x%s
				
				pub fun main(account: Address): UFix64 {
					let acct = getAccount(account)
					let vaultRef = acct.getCapability(/public/flowTokenBalance)
						.borrow<&FlowToken.Vault{FungibleToken.Balance}>()
						?? panic("Could not borrow Balance reference to the Vault")
				
					return vaultRef.balance
				}
			`, fvm.FungibleTokenAddress(ctx.Chain), fvm.FlowTokenAddress(ctx.Chain)))

	return &newVersionCollector{
		wg:     wg,
		logger: logger,
		vm:     vm,
		ctx:    ctx,
		view:   view.NewChild(),
		prog:   prog,
		script: script}
}

// change this to incoming channel and outgoing channel
func (c *newVersionCollector) CollectBalances(addresses chan uint64, balances chan accountBalance) {
	for indx := range addresses {
		var balance uint64
		address, err := c.ctx.Chain.AddressAtIndex(indx)
		if err != nil {
			c.logger.Err(err).Msgf("Error collecting balance for account %s", address.String())
		}

		script := fvm.Script(c.script).WithArguments(
			jsoncdc.MustEncode(cadence.NewAddress(address)),
		)

		err = c.vm.Run(c.ctx, script, c.view, c.prog)
		if err != nil {
			c.logger.Err(err).Msgf("Error collecting balance for account %s", address.String())
		}

		if script.Err == nil && script.Value != nil {
			balance = script.Value.ToGoValue().(uint64)
		} else {
			c.logger.Err(script.Err).Msgf("Error collecting balance for account %s", address.String())
		}

		balances <- accountBalance{addressIndex: indx, balance: balance}
		// for debugging
		c.logger.Info().Msgf("collected balance for %s (%d)", address.String(), balance)
	}
	c.wg.Done()
}

type oldVersionCollector struct {
	wg     *sync.WaitGroup
	logger zerolog.Logger
	vm     *oldFvm.VirtualMachine
	ctx    oldFvm.Context
	view   state.View
	prog   *programs.Programs
	script []byte
}

func newOldVersionCollector(wg *sync.WaitGroup,
	logger zerolog.Logger,
	chain flow.Chain,
	view state.View) *oldVersionCollector {
	vm := oldFvm.NewVirtualMachine(oldFvm.NewInterpreterRuntime())
	ctx := oldFvm.NewContext(zerolog.Nop(), oldFvm.WithChain(chain))
	prog := programs.NewEmptyPrograms()
	script := []byte(fmt.Sprintf(`
				import FungibleToken from 0x%s
				import FlowToken from 0x%s
				
				pub fun main(account: Address): UFix64 {
					let acct = getAccount(account)
					let vaultRef = acct.getCapability(/public/flowTokenBalance)
						.borrow<&FlowToken.Vault{FungibleToken.Balance}>()
						?? panic("Could not borrow Balance reference to the Vault")
				
					return vaultRef.balance
				}
			`, fvm.FungibleTokenAddress(ctx.Chain), fvm.FlowTokenAddress(ctx.Chain)))

	return &oldVersionCollector{
		wg:     wg,
		logger: logger,
		vm:     vm,
		ctx:    ctx,
		view:   view.NewChild(),
		prog:   prog,
		script: script}
}

// change this to incoming channel and outgoing channel
func (c *oldVersionCollector) CollectBalances(addresses chan uint64, balances chan accountBalance) {
	for indx := range addresses {
		var balance uint64
		address, err := c.ctx.Chain.AddressAtIndex(indx)
		if err != nil {
			c.logger.Err(err).Msgf("Error collecting balance for account %s", address.String())
		}

		script := oldFvm.Script(c.script).WithArguments(
			jsoncdc.MustEncode(cadence.NewAddress(address)),
		)

		err = c.vm.Run(c.ctx, script, c.view, c.prog)
		if err != nil {
			c.logger.Err(err).Msgf("Error collecting balance for account %s", address.String())
		}

		if script.Err == nil && script.Value != nil {
			balance = script.Value.ToGoValue().(uint64)
		} else {
			c.logger.Err(script.Err).Msgf("Error collecting balance for account %s", address.String())
		}

		balances <- accountBalance{addressIndex: indx, balance: balance}
		// for debugging
		c.logger.Info().Msgf("collected balance for %s (%d)", address.String(), balance)
	}
	c.wg.Done()
}

type AccountBalanceValidator struct {
	chain            flow.Chain
	logger           zerolog.Logger
	numberOfAccounts uint64
	originalBalances map[uint64]uint64
}

func NewAccountBalanceValidator(logger zerolog.Logger, chain flow.Chain) *AccountBalanceValidator {
	return &AccountBalanceValidator{
		chain:            chain,
		logger:           logger,
		originalBalances: make(map[uint64]uint64),
	}
}

func (v *AccountBalanceValidator) Setup(oldPayloads []ledger.Payload) error {

	mainView := newView(oldPayloads)
	st := state.NewState(mainView)
	sth := state.NewStateHolder(st)
	gen := state.NewStateBoundAddressGenerator(sth, v.chain)
	v.numberOfAccounts = gen.AddressCount()

	addressIndices := make(chan uint64, v.numberOfAccounts)
	balances := make(chan accountBalance, v.numberOfAccounts)

	for i := uint64(0); i < v.numberOfAccounts; i++ {
		addressIndices <- i
	}

	workerCount := runtime.NumCPU()
	wg := &sync.WaitGroup{}

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		worker := newOldVersionCollector(wg, v.logger, v.chain, mainView)
		go worker.CollectBalances(addressIndices, balances)
	}

	close(addressIndices)
	wg.Wait()
	close(balances)

	for balance := range balances {
		v.originalBalances[balance.addressIndex] = balance.balance
	}

	return nil
}

func (v *AccountBalanceValidator) Validate(newPayloads []ledger.Payload) (isValid bool, err error) {

	mainView := newView(newPayloads)
	st := state.NewState(mainView)
	sth := state.NewStateHolder(st)

	gen := state.NewStateBoundAddressGenerator(sth, v.chain)
	if v.numberOfAccounts != gen.AddressCount() {
		return false, fmt.Errorf("number of accounts doesn't match after migration (before:%d after: %d)", v.numberOfAccounts, gen.AddressCount())
	}

	addressIndices := make(chan uint64, v.numberOfAccounts)
	newBalances := make(chan accountBalance, v.numberOfAccounts)

	for i := uint64(0); i < v.numberOfAccounts; i++ {
		addressIndices <- i
	}

	workerCount := runtime.NumCPU()
	wg := &sync.WaitGroup{}

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		worker := newNewVersionCollector(wg, v.logger, v.chain, mainView)
		go worker.CollectBalances(addressIndices, newBalances)
	}

	close(addressIndices)
	wg.Wait()
	close(newBalances)

	// comparing the balances
	for newb := range newBalances {
		orgBalance := v.originalBalances[newb.addressIndex]
		if orgBalance != newb.balance {
			add, _ := v.chain.AddressAtIndex(newb.addressIndex)
			return false, fmt.Errorf("balances are not matching for account %s (before: %d, after: %d)", add.String(), orgBalance, newb.balance)
		}
	}

	return true, nil
}
