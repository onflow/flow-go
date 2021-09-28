package validators

import (
	"fmt"
	"runtime"
	"sync"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

type accountBalance struct {
	account flow.Address
	balance uint64
}

type AccountBalanceValidator struct {
	chain            flow.Chain
	logger           zerolog.Logger
	numberOfAccounts uint64
	addresses        chan flow.Address
	balances         chan accountBalance
}

func NewAccountBalanceValidator(logger zerolog.Logger, chain flow.Chain) *AccountBalanceValidator {
	return &AccountBalanceValidator{
		chain:  chain,
		logger: logger,
	}
}

func (v *AccountBalanceValidator) Setup(oldPayloads []ledger.Payload) error {

	mainView := newView(oldPayloads)
	st := state.NewState(mainView)
	sth := state.NewStateHolder(st)

	gen := state.NewStateBoundAddressGenerator(sth, v.chain)
	v.numberOfAccounts = gen.AddressCount()
	v.addresses = make(chan flow.Address, v.numberOfAccounts)
	v.balances = make(chan accountBalance, v.numberOfAccounts)

	fvmContext := fvm.NewContext(
		v.logger,
		fvm.WithChain(v.chain),
	)

	workerCount := runtime.NumCPU()
	wg := &sync.WaitGroup{}

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		worker := newWorker(wg, fvmContext, mainView.NewChild(), v.logger)
		go worker.Collect(v.addresses, v.balances)
	}

	wg.Wait()
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

	fvmContext := fvm.NewContext(
		v.logger,
		fvm.WithChain(v.chain),
	)

	workerCount := runtime.NumCPU()
	wg := &sync.WaitGroup{}

	notVerified := make(chan flow.Address, v.numberOfAccounts)

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		worker := newWorker(wg, fvmContext, mainView.NewChild(), v.logger)
		go worker.CollectAndVerify(v.balances, notVerified)
	}

	wg.Wait()

	if len(notVerified) != 0 {
		// TODO print them
		// if ac := range notVerified {
		// 	v.logger.Err().Msgf("balance for account %s doesn't match the one before migration", ac.String())
		// }
		return false, fmt.Errorf("some balances doesn't match")
	}

	return true, nil
}

type worker struct {
	wg     *sync.WaitGroup
	logger zerolog.Logger
	vm     *fvm.VirtualMachine
	ctx    fvm.Context
	view   state.View
	prog   *programs.Programs
	script []byte
}

func newWorker(wg *sync.WaitGroup,
	ctx fvm.Context,
	view state.View,
	logger zerolog.Logger,
) *worker {
	vm := fvm.NewVirtualMachine(fvm.NewInterpreterRuntime())
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

	return &worker{wg: wg,
		logger: logger,
		vm:     vm,
		ctx:    ctx,
		view:   view,
		prog:   prog,
		script: script}
}

// change this to incoming channel and outgoing channel
func (w *worker) Collect(addresses chan flow.Address, balances chan accountBalance) {
	for add := range addresses {
		balance, err := w.collectBalance(add)
		if err != nil {
			w.logger.Err(err).Msgf("Error collecting balance for account %s", add.String())
		}
		balances <- accountBalance{account: add, balance: balance}
	}
	w.wg.Done()
}

func (w *worker) CollectAndVerify(balances chan accountBalance, notVerified chan flow.Address) {
	for b := range balances {
		balance, err := w.collectBalance(b.account)
		if err != nil {
			w.logger.Err(err).Msgf("Error collecting balance for account %s", b.account.String())
		}
		if balance != b.balance {
			notVerified <- b.account
		}
	}
	w.wg.Done()
}

func (w *worker) collectBalance(address flow.Address) (uint64, error) {
	script := fvm.Script(w.script).WithArguments(
		jsoncdc.MustEncode(cadence.NewAddress(address)),
	)

	err := w.vm.Run(w.ctx, script, w.view, w.prog)
	if err != nil {
		return 0, err
	}
	return script.Value.ToGoValue().(uint64), nil
}
