package reporters

import (
	"fmt"
	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/rs/zerolog"
	"github.com/schollz/progressbar/v3"
	goRuntime "runtime"
	"sync"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

// AccountReporter iterates through registers keeping a map of register sizes
// reports on storage metrics
type AccountReporter struct {
	Log   zerolog.Logger
	RWF   ReportWriterFactory
	Chain flow.Chain
}

var _ ledger.Reporter = &AccountReporter{}

func (r *AccountReporter) Name() string {
	return "Account Reporter"
}

type accountRecord struct {
	Address        string
	StorageUsed    uint64
	AccountBalance uint64
	HasVault       bool
	HasReceiver    bool
	IsDapper       bool
}

type contractRecord struct {
	Address  string
	Contract string
}

func (r *AccountReporter) Report(payload []ledger.Payload) error {
	rwa := r.RWF.ReportWriter("account_report")
	rwc := r.RWF.ReportWriter("contract_report")
	defer rwa.Close()
	defer rwc.Close()

	l := migrations.NewView(payload)
	st := state.NewState(l)
	sth := state.NewStateHolder(st)
	gen := state.NewStateBoundAddressGenerator(sth, r.Chain)

	progress := progressbar.Default(int64(gen.AddressCount()), "Processing:")

	addressIndexes := make(chan uint64)
	wg := &sync.WaitGroup{}

	workerCount := goRuntime.NumCPU() / 2
	if workerCount == 0 {
		workerCount = 1
	}

	for i := 0; i < workerCount; i++ {
		adp := newAccountDataProcessor(wg, r.Log, progress, rwa, rwc, r.Chain, l)
		wg.Add(1)
		go adp.reportAccountData(addressIndexes)
	}

	for i := uint64(0); i < gen.AddressCount(); i++ {
		addressIndexes <- i
	}
	close(addressIndexes)

	wg.Wait()

	err := progress.Finish()
	if err != nil {
		panic(err)
	}

	return nil
}

type balanceProcessor struct {
	vm     *fvm.VirtualMachine
	ctx    fvm.Context
	view   state.View
	prog   *programs.Programs
	script []byte

	accounts state.Accounts
	st       *state.State

	rwa      ReportWriter
	rwc      ReportWriter
	progress *progressbar.ProgressBar
	wg       *sync.WaitGroup
	logger   zerolog.Logger
}

func newAccountDataProcessor(wg *sync.WaitGroup, logger zerolog.Logger, progress *progressbar.ProgressBar, rwa ReportWriter, rwc ReportWriter, chain flow.Chain, view state.View) *balanceProcessor {

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

	v := view.NewChild()
	st := state.NewState(v)
	sth := state.NewStateHolder(st)
	accounts := state.NewAccounts(sth)

	return &balanceProcessor{
		wg:       wg,
		logger:   logger,
		vm:       vm,
		ctx:      ctx,
		view:     v,
		accounts: accounts,
		st:       st,
		prog:     prog,
		rwa:      rwa,
		rwc:      rwc,
		progress: progress,
		script:   script}
}

func (c *balanceProcessor) reportAccountData(addressIndexes <-chan uint64) {
	for indx := range addressIndexes {

		address, err := c.ctx.Chain.AddressAtIndex(indx)
		if err != nil {
			c.logger.
				Err(err).
				Uint64("index", indx).
				Msgf("Error getting address")
			continue
		}

		u, err := c.storageUsed(address)
		if err != nil {
			c.logger.
				Err(err).
				Uint64("index", indx).
				Str("address", address.String()).
				Msgf("Error getting storage used for account")
			continue
		}

		balance, hasVault, err := c.balance(address)
		if err != nil {
			c.logger.
				Err(err).
				Uint64("index", indx).
				Str("address", address.String()).
				Msgf("Error getting balance for account")
			continue
		}

		dapper, err := c.isDapper(address)
		if err != nil {
			c.logger.
				Err(err).
				Uint64("index", indx).
				Str("address", address.String()).
				Msgf("Error determining if account is dapper account")
			continue
		}

		hasReceiver, err := c.hasReceiver(address)
		if err != nil {
			c.logger.
				Err(err).
				Uint64("index", indx).
				Str("address", address.String()).
				Msgf("Error checking if account has a receiver")
			continue
		}

		c.rwa.Write(accountRecord{
			Address:        address.Hex(),
			StorageUsed:    u,
			AccountBalance: balance,
			HasVault:       hasVault,
			HasReceiver:    hasReceiver,
			IsDapper:       dapper,
		})

		contracts, err := c.accounts.GetContractNames(address)
		if err != nil {
			c.logger.
				Err(err).
				Uint64("index", indx).
				Str("address", address.String()).
				Msgf("Error getting account contract names")
			continue
		}
		if len(contracts) == 0 {
			continue
		}
		for _, contract := range contracts {
			c.rwc.Write(contractRecord{
				Address:  address.Hex(),
				Contract: contract,
			})
		}

		err = c.progress.Add(1)
		if err != nil {
			panic(err)
		}
	}
	c.wg.Done()
}

func (c *balanceProcessor) balance(address flow.Address) (uint64, bool, error) {
	script := fvm.Script(c.script).WithArguments(
		jsoncdc.MustEncode(cadence.NewAddress(address)),
	)

	err := c.vm.Run(c.ctx, script, c.view, c.prog)
	if err != nil {
		return 0, false, err
	}

	var balance uint64
	var hasVault bool
	if script.Err == nil && script.Value != nil {
		balance = script.Value.ToGoValue().(uint64)
		hasVault = true
	} else {
		hasVault = false
	}
	return balance, hasVault, nil
}

func (c *balanceProcessor) storageUsed(address flow.Address) (uint64, error) {
	return c.accounts.GetStorageUsed(address)
}

func (c *balanceProcessor) isDapper(address flow.Address) (bool, error) {
	id := resourceId(address,
		interpreter.PathValue{
			Domain:     common.PathDomainPublic,
			Identifier: "dapperUtilityCoinReceiver",
		})

	receiver, err := c.st.Get(id.Owner, id.Controller, id.Key)
	if err != nil {
		return false, fmt.Errorf("could not load dapper receiver at %s: %w", address, err)
	}
	return len(receiver) != 0, nil
}

func (c *balanceProcessor) hasReceiver(address flow.Address) (bool, error) {
	id := resourceId(address,
		interpreter.PathValue{
			Domain:     common.PathDomainPublic,
			Identifier: "flowTokenReceiver",
		})

	receiver, err := c.st.Get(id.Owner, id.Controller, id.Key)
	if err != nil {
		return false, fmt.Errorf("could not load receiver at %s: %w", address, err)
	}
	return len(receiver) != 0, nil
}

func resourceId(address flow.Address, path interpreter.PathValue) flow.RegisterID {
	// Copied logic from interpreter.storageKey(path)
	key := fmt.Sprintf("%s\x1F%s", path.Domain.Identifier(), path.Identifier)

	return flow.RegisterID{
		Owner:      string(address.Bytes()),
		Controller: "",
		Key:        key,
	}
}
