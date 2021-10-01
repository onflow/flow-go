package validators


import (
	"fmt"
	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"runtime"
	"sync"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	oldCadence "github.com/onflow/cadence/v19"
	oldjsoncdc "github.com/onflow/cadence/v19/encoding/json"
	oldRuntime "github.com/onflow/cadence/v19/runtime"
	oldInterpreter "github.com/onflow/cadence/v19/runtime/interpreter"
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

	mainView := migrations.NewView(oldPayloads)
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
		worker := newOldVersionCollector(wg, oldPayloads, v.logger, v.chain)
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

	mainView := migrations.NewView(newPayloads)
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

type oldVersionCollector struct {
	wg      *sync.WaitGroup
	logger  zerolog.Logger
	chain   flow.Chain
	runtime oldRuntime.Runtime
	env     *oldInter
	script  []byte
}

func newOldVersionCollector(
	wg *sync.WaitGroup,
	payloads []ledger.Payload,
	logger zerolog.Logger,
	chain flow.Chain) *oldVersionCollector {

	env := newOldInter(payloads)
	runtime := oldRuntime.NewInterpreterRuntime()
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
			`, fvm.FungibleTokenAddress(chain), fvm.FlowTokenAddress(chain)))

	return &oldVersionCollector{
		wg:      wg,
		logger:  logger,
		chain:   chain,
		runtime: runtime,
		env:     env,
		script:  script}
}

// change this to incoming channel and outgoing channel
func (c *oldVersionCollector) CollectBalances(addresses chan uint64, balances chan accountBalance) {
	for indx := range addresses {
		var balance uint64
		address, err := c.chain.AddressAtIndex(indx)
		if err != nil {
			c.logger.Err(err).Msgf("Error collecting balance for account %s", address.String())
		}

		// add := oldCadence.NewAddress(address)
		value, err := c.runtime.ExecuteScript(
			oldRuntime.Script{
				Source:    c.script,
				Arguments: nil,
			},
			oldRuntime.Context{
				Interface: c.env,
				Location:  oldRuntime.ScriptLocation("LOCATION"),
			},
		)

		if err == nil && value != nil {
			balance = value.ToGoValue().(uint64)
		} else {
			c.logger.Err(err).Msgf("Error collecting balance for account %s", address.String())
		}

		balances <- accountBalance{addressIndex: indx, balance: balance}
		// for debugging
		c.logger.Info().Msgf("collected balance for %s (%d)", address.String(), balance)
	}
	c.wg.Done()
}

type oldInter struct {
	payloads map[string][]byte
	programs map[oldRuntime.Location]*oldInterpreter.Program
}

func newOldInter(data []ledger.Payload) *oldInter {
	payloads := make(map[string][]byte, len(data))
	for _, p := range data {
		fk := string(p.Key.KeyParts[0].Value) + "-" + string(p.Key.KeyParts[2].Value)
		payloads[fk] = p.Value
	}

	return &oldInter{
		payloads: payloads,
		programs: make(map[oldRuntime.Location]*oldInterpreter.Program),
	}
}

func (i *oldInter) ResolveLocation(identifiers []oldRuntime.Identifier, location oldRuntime.Location) ([]oldRuntime.ResolvedLocation, error) {
	return []oldRuntime.ResolvedLocation{
		{
			Location:    location,
			Identifiers: identifiers,
		},
	}, nil
}
func (i *oldInter) SetProgram(location oldRuntime.Location, program *oldInterpreter.Program) error {
	i.programs[location] = program
	return nil
}
func (i *oldInter) GetProgram(location oldRuntime.Location) (*oldInterpreter.Program, error) {
	return i.programs[location], nil
}
func (i *oldInter) GetCode(_ oldRuntime.Location) ([]byte, error) { return nil, nil }
func (i *oldInter) ValueExists(owner, key []byte) (exists bool, err error) {
	fk := string(owner) + "-" + string(key)
	return len(i.payloads[fk]) > 0, nil
}
func (i *oldInter) GetValue(owner, key []byte) (value []byte, err error) {
	fk := string(owner) + "-" + string(key)
	return i.payloads[fk], nil
}
func (i *oldInter) SetValue(owner, key, value []byte) error {
	fk := string(owner) + "-" + string(key)
	i.payloads[fk] = value
	return nil
}
func (i *oldInter) CreateAccount(_ oldRuntime.Address) (address oldRuntime.Address, err error) {
	return oldRuntime.Address{}, nil
}
func (i *oldInter) AddEncodedAccountKey(_ oldRuntime.Address, _ []byte) error {
	return nil
}
func (i *oldInter) RevokeEncodedAccountKey(_ oldRuntime.Address, _ int) ([]byte, error) {
	return nil, nil
}
func (i *oldInter) AddAccountKey(_ oldRuntime.Address, _ *oldRuntime.PublicKey, _ oldRuntime.HashAlgorithm, _ int) (*oldRuntime.AccountKey, error) {
	return nil, nil
}
func (i *oldInter) RevokeAccountKey(_ oldRuntime.Address, _ int) (*oldRuntime.AccountKey, error) {
	return nil, nil
}
func (i *oldInter) GetAccountKey(_ oldRuntime.Address, _ int) (*oldRuntime.AccountKey, error) {
	return nil, nil
}
func (i *oldInter) UpdateAccountContractCode(_ oldRuntime.Address, _ string, _ []byte) (err error) {
	return nil
}
func (i *oldInter) GetAccountContractCode(_ oldRuntime.Address, _ string) (code []byte, err error) {
	return nil, nil
}
func (i *oldInter) RemoveAccountContractCode(_ oldRuntime.Address, _ string) (err error) {
	return nil
}
func (i *oldInter) GetSigningAccounts() ([]oldRuntime.Address, error) {
	return nil, nil
}
func (i *oldInter) ProgramLog(_ string) error {
	return nil
}
func (i *oldInter) EmitEvent(_ oldCadence.Event) error {
	return nil
}
func (i *oldInter) GenerateUUID() (uint64, error) {
	return 0, nil
}
func (i *oldInter) GetComputationLimit() uint64 {
	return 0
}
func (i *oldInter) SetComputationUsed(uint64) error {
	return nil
}
func (i *oldInter) DecodeArgument(input []byte, _ oldCadence.Type) (oldCadence.Value, error) {
	return oldjsoncdc.Decode(input)
}
func (i *oldInter) GetCurrentBlockHeight() (uint64, error) {
	return 0, nil
}
func (i *oldInter) GetBlockAtHeight(_ uint64) (block oldRuntime.Block, exists bool, err error) {
	return
}
func (i *oldInter) UnsafeRandom() (uint64, error) {
	return 0, nil
}
func (i *oldInter) ImplementationDebugLog(_ string) error {
	return nil
}
func (i *oldInter) VerifySignature(_ []byte, _ string, _ []byte, _ []byte, _ oldRuntime.SignatureAlgorithm, _ oldRuntime.HashAlgorithm) (bool, error) {
	return false, nil
}
func (i *oldInter) Hash(_ []byte, _ string, _ oldRuntime.HashAlgorithm) ([]byte, error) {
	return nil, nil
}
func (i oldInter) GetAccountBalance(_ oldRuntime.Address) (uint64, error) {
	return 0, nil
}
func (i oldInter) GetAccountAvailableBalance(_ oldRuntime.Address) (uint64, error) {
	return 0, nil
}
func (i oldInter) GetStorageUsed(_ oldRuntime.Address) (uint64, error) {
	return 0, nil
}
func (i oldInter) GetStorageCapacity(_ oldRuntime.Address) (uint64, error) {
	return 0, nil
}
func (i *oldInter) ValidatePublicKey(_ *oldRuntime.PublicKey) (bool, error) {
	return false, nil
}
func (i *oldInter) GetAccountContractNames(_ oldRuntime.Address) ([]string, error) {
	return nil, nil
}
