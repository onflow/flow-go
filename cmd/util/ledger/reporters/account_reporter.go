package reporters

import (
	"fmt"
	"math"
	goRuntime "runtime"
	"sync"

	"github.com/rs/zerolog"
	"github.com/schollz/progressbar/v3"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/programs"
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
	Address        string `json:"address"`
	StorageUsed    uint64 `json:"storageUsed"`
	AccountBalance uint64 `json:"accountBalance"`
	FUSDBalance    uint64 `json:"fusdBalance"`
	HasVault       bool   `json:"hasVault"`
	HasReceiver    bool   `json:"hasReceiver"`
	IsDapper       bool   `json:"isDapper"`
}

type contractRecord struct {
	Address  string `json:"address"`
	Contract string `json:"contract"`
}

type momentsRecord struct {
	Address string `json:"address"`
	Moments int    `json:"moments"`
}

func (r *AccountReporter) Report(payload []ledger.Payload) error {
	rwa := r.RWF.ReportWriter("account_report")
	rwc := r.RWF.ReportWriter("contract_report")
	rwm := r.RWF.ReportWriter("moments_report")
	defer rwa.Close()
	defer rwc.Close()
	defer rwm.Close()

	l := migrations.NewView(payload)
	st := state.NewState(l, state.WithMaxInteractionSizeAllowed(math.MaxUint64))
	sth := state.NewStateHolder(st)
	gen := state.NewStateBoundAddressGenerator(sth, r.Chain)

	progress := progressbar.Default(int64(gen.AddressCount()), "Processing:")

	workerCount := goRuntime.NumCPU() / 2
	if workerCount == 0 {
		workerCount = 1
	}

	addressIndexes := make(chan uint64, workerCount)
	defer close(addressIndexes)

	// create multiple workers to generate account data report concurrently
	wg := &sync.WaitGroup{}
	for i := 0; i < workerCount; i++ {
		go func() {
			adp := newAccountDataProcessor(r.Log, rwa, rwc, rwm, r.Chain, l)
			for indx := range addressIndexes {
				adp.reportAccountData(indx)
				wg.Done()
			}
		}()
	}

	addressCount := gen.AddressCount()
	// produce jobs for workers to process
	for i := uint64(1); i <= addressCount; i++ {
		addressIndexes <- i

		wg.Add(1)

		err := progress.Add(1)
		if err != nil {
			panic(fmt.Errorf("progress.Add(1): %w", err))
		}
	}

	// wait until all jobs are done
	wg.Wait()

	err := progress.Finish()
	if err != nil {
		panic(fmt.Errorf("progress.Finish(): %w", err))
	}

	return nil
}

type balanceProcessor struct {
	vm            *fvm.VirtualMachine
	ctx           fvm.Context
	view          state.View
	prog          *programs.Programs
	intf          runtime.Interface
	balanceScript []byte
	momentsScript []byte

	accounts state.Accounts
	st       *state.State

	rwa        ReportWriter
	rwc        ReportWriter
	logger     zerolog.Logger
	rwm        ReportWriter
	fusdScript []byte
}

func NewBalanceReporter(chain flow.Chain, view state.View) *balanceProcessor {
	vm := fvm.NewVirtualMachine(fvm.NewInterpreterRuntime())
	ctx := fvm.NewContext(zerolog.Nop(), fvm.WithChain(chain))
	prog := programs.NewEmptyPrograms()

	v := view.NewChild()
	st := state.NewState(v, state.WithMaxInteractionSizeAllowed(math.MaxUint64))
	sth := state.NewStateHolder(st)
	accounts := state.NewAccounts(sth)

	return &balanceProcessor{
		vm:       vm,
		ctx:      ctx,
		view:     v,
		accounts: accounts,
		st:       st,
		prog:     prog,
		intf:     fvm.NewScriptEnvironment(ctx, vm, sth, prog),
	}
}

func newAccountDataProcessor(logger zerolog.Logger, rwa ReportWriter, rwc ReportWriter, rwm ReportWriter, chain flow.Chain, view state.View) *balanceProcessor {
	bp := NewBalanceReporter(chain, view)

	bp.logger = logger
	bp.rwa = rwa
	bp.rwc = rwc
	bp.rwm = rwm
	bp.balanceScript = []byte(fmt.Sprintf(`
				import FungibleToken from 0x%s
				import FlowToken from 0x%s

				pub fun main(account: Address): UFix64 {
					let acct = getAccount(account)
					let vaultRef = acct.getCapability(/public/flowTokenBalance)
						.borrow<&FlowToken.Vault{FungibleToken.Balance}>()
						?? panic("Could not borrow Balance reference to the Vault")

					return vaultRef.balance
				}
			`, fvm.FungibleTokenAddress(bp.ctx.Chain), fvm.FlowTokenAddress(bp.ctx.Chain)))

	bp.fusdScript = []byte(fmt.Sprintf(`
			import FungibleToken from 0x%s
			import FUSD from 0x%s

			pub fun main(address: Address): UFix64 {
				let account = getAccount(address)

				let vaultRef = account.getCapability(/public/fusdBalance)!
					.borrow<&FUSD.Vault{FungibleToken.Balance}>()
					?? panic("Could not borrow Balance reference to the Vault")

				return vaultRef.balance
			}
			`, fvm.FungibleTokenAddress(bp.ctx.Chain), "3c5959b568896393"))

	bp.momentsScript = []byte(`
			import TopShot from 0x0b2a3299cc857e29

			pub fun main(account: Address): Int {
				let acct = getAccount(account)
				let collectionRef = acct.getCapability(/public/MomentCollection)
										.borrow<&{TopShot.MomentCollectionPublic}>()!

				return collectionRef.getIDs().length
			}
			`)

	return bp
}

func (c *balanceProcessor) reportAccountData(indx uint64) {
	address, err := c.ctx.Chain.AddressAtIndex(indx)
	if err != nil {
		c.logger.
			Err(err).
			Uint64("index", indx).
			Msgf("Error getting address")
		return
	}

	u, err := c.storageUsed(address)
	if err != nil {
		c.logger.
			Err(err).
			Uint64("index", indx).
			Str("address", address.String()).
			Msgf("Error getting storage used for account")
		return
	}

	balance, hasVault, err := c.balance(address)
	if err != nil {
		c.logger.
			Err(err).
			Uint64("index", indx).
			Str("address", address.String()).
			Msgf("Error getting balance for account")
		return
	}
	fusdBalance, err := c.fusdBalance(address)
	if err != nil {
		c.logger.
			Err(err).
			Uint64("index", indx).
			Str("address", address.String()).
			Msgf("Error getting FUSD balance for account")
		return
	}

	dapper, err := c.isDapper(address)
	if err != nil {
		c.logger.
			Err(err).
			Uint64("index", indx).
			Str("address", address.String()).
			Msgf("Error determining if account is dapper account")
		return
	}
	if dapper {
		m, err := c.moments(address)
		if err != nil {
			c.logger.
				Err(err).
				Uint64("index", indx).
				Str("address", address.String()).
				Msgf("Error getting moments for account")
			return
		}
		c.rwm.Write(momentsRecord{
			Address: address.Hex(),
			Moments: m,
		})
	}

	hasReceiver, err := c.hasReceiver(address)
	if err != nil {
		c.logger.
			Err(err).
			Uint64("index", indx).
			Str("address", address.String()).
			Msgf("Error checking if account has a receiver")
		return
	}

	c.rwa.Write(accountRecord{
		Address:        address.Hex(),
		StorageUsed:    u,
		AccountBalance: balance,
		FUSDBalance:    fusdBalance,
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
		return
	}
	if len(contracts) == 0 {
		return
	}
	for _, contract := range contracts {
		c.rwc.Write(contractRecord{
			Address:  address.Hex(),
			Contract: contract,
		})
	}

}

func (c *balanceProcessor) balance(address flow.Address) (uint64, bool, error) {
	script := fvm.Script(c.balanceScript).WithArguments(
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

func (c *balanceProcessor) fusdBalance(address flow.Address) (uint64, error) {
	script := fvm.Script(c.fusdScript).WithArguments(
		jsoncdc.MustEncode(cadence.NewAddress(address)),
	)

	err := c.vm.Run(c.ctx, script, c.view, c.prog)
	if err != nil {
		return 0, err
	}

	var balance uint64
	if script.Err == nil && script.Value != nil {
		balance = script.Value.ToGoValue().(uint64)
	}
	return balance, nil
}

func (c *balanceProcessor) moments(address flow.Address) (int, error) {
	script := fvm.Script(c.momentsScript).WithArguments(
		jsoncdc.MustEncode(cadence.NewAddress(address)),
	)

	err := c.vm.Run(c.ctx, script, c.view, c.prog)
	if err != nil {
		return 0, err
	}

	var m int
	if script.Err == nil && script.Value != nil {
		m = script.Value.(cadence.Int).Int()
	}
	return m, nil
}

func (c *balanceProcessor) storageUsed(address flow.Address) (uint64, error) {
	return c.accounts.GetStorageUsed(address)
}

func (c *balanceProcessor) isDapper(address flow.Address) (bool, error) {
	receiver, err := c.ReadStored(address, common.PathDomainPublic, "dapperUtilityCoinReceiver")
	if err != nil {
		return false, fmt.Errorf("could not load dapper receiver at %s: %w", address, err)
	}
	return receiver != nil, nil
}

func (c *balanceProcessor) hasReceiver(address flow.Address) (bool, error) {
	receiver, err := c.ReadStored(address, common.PathDomainPublic, "flowTokenReceiver")

	if err != nil {
		return false, fmt.Errorf("could not load receiver at %s: %w", address, err)
	}
	return receiver != nil, nil
}

func (c *balanceProcessor) ReadStored(address flow.Address, domain common.PathDomain, id string) (cadence.Value, error) {
	addr, err := common.BytesToAddress(address.Bytes())
	if err != nil {
		return nil, err
	}
	receiver, err := c.vm.Runtime.ReadStored(addr,
		cadence.Path{
			Domain:     domain.Identifier(),
			Identifier: id,
		},
		runtime.Context{Interface: c.intf},
	)
	return receiver, err
}
