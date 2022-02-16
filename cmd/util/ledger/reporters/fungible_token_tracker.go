package reporters

import (
	"fmt"
	"strings"
	"sync"

	"github.com/rs/zerolog"
	"github.com/schollz/progressbar/v3"

	cadenceRuntime "github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

const FungibleTokenTrackerReportPrefix = "fungible_token_report"

// FungibleTokenTracker iterates through stored cadence values over all accounts and check for any
// value with the given resource typeID
type FungibleTokenTracker struct {
	log          zerolog.Logger
	chain        flow.Chain
	rwf          ReportWriterFactory
	rw           ReportWriter
	progress     *progressbar.ProgressBar
	vaultTypeIDs []string
}

func FlowTokenTypeID(chain flow.Chain) string {
	return fmt.Sprintf("A.%s.FlowToken.Vault", fvm.FlowTokenAddress(chain).Hex())
}

func NewFungibleTokenTracker(logger zerolog.Logger, rwf ReportWriterFactory, chain flow.Chain, vaultTypeIDs []string) *FungibleTokenTracker {
	return &FungibleTokenTracker{
		log:          logger,
		rwf:          rwf,
		chain:        chain,
		vaultTypeIDs: vaultTypeIDs,
	}
}

func (r *FungibleTokenTracker) Name() string {
	return "Resource Tracker"
}

type trace []string

func (t trace) String() string {
	return strings.Join(t, "/")
}

type TokenDataPoint struct {
	// Path is the storage path of the composite the vault was found in
	Path string `json:"path"`
	// Address is the owner of the composite the vault was found in
	Address string `json:"address"`
	// Balance is the balance of the flow vault
	Balance uint64 `json:"balance"`
	// token type
	TypeID string `json:"type_id"`
}

// Report creates a fungible_token_report_*.json file that contains data on all fungible token Vaults in the state commitment.
// I recommend using gojq to browse through the data, because of the large uint64 numbers which jq won't be able to handle.
func (r *FungibleTokenTracker) Report(payload []ledger.Payload) error {
	r.rw = r.rwf.ReportWriter(FungibleTokenTrackerReportPrefix)
	defer r.rw.Close()

	r.progress = progressbar.Default(int64(len(payload)), "Processing:")

	ldg := migrations.NewView(payload)
	sth := state.NewStateHolder(state.NewState(ldg))
	addressGen := state.NewStateBoundAddressGenerator(sth, r.chain)
	addressCount := addressGen.AddressCount()

	wg := &sync.WaitGroup{}
	jobs := make(chan flow.Address, addressCount)

	for i := uint64(1); i < addressCount; i++ {
		addr, err := r.chain.AddressAtIndex(i)
		if err != nil {
			panic(err)
		}
		jobs <- addr
	}

	close(jobs)

	// RAMTIN: hmm, ledger is not thread safe :-?
	workerCount := 1 // runtime.NumCPU()
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go r.worker(ldg, jobs, wg)
	}

	wg.Wait()

	err := r.progress.Finish()
	if err != nil {
		panic(err)
	}

	return nil
}

func (r *FungibleTokenTracker) worker(
	l state.View,
	jobs <-chan flow.Address,
	wg *sync.WaitGroup) {
	st := state.NewState(l)
	sth := state.NewStateHolder(st)
	accounts := state.NewAccounts(sth)
	storage := cadenceRuntime.NewStorage(
		&migrations.AccountsAtreeLedger{Accounts: accounts},
	)

	for address := range jobs {
		r.handleAddress(address, storage)

		err := r.progress.Add(1)
		if err != nil {
			panic(err)
		}
	}

	wg.Done()
}

func (r *FungibleTokenTracker) iterateChildren(tr trace, addr flow.Address, value interpreter.Value) {

	compValue, ok := value.(*interpreter.CompositeValue)
	if !ok {
		return
	}

	if compValue.IsResourceKinded(nil) {
		typeIDStr := string(compValue.TypeID())
		hasTargetType := false
		for _, vt := range r.vaultTypeIDs {
			if typeIDStr == vt {
				hasTargetType = true
				break
			}
		}

		if hasTargetType {
			b := uint64(compValue.GetField(nil, nil, "balance").(interpreter.UFix64Value))
			if b > 0 {
				r.rw.Write(TokenDataPoint{
					Path:    tr.String(),
					Address: addr.Hex(),
					Balance: b,
					TypeID:  string(compValue.TypeID()),
				})
			}
		}
	}

	// iterate over fields
	compValue.ForEachField(func(key string, value interpreter.Value) {
		r.iterateChildren(append(tr, key), addr, value)
	})
}

func (r *FungibleTokenTracker) handleAddress(adr flow.Address, storage *cadenceRuntime.Storage) {

	domains := []string{
		common.PathDomainPublic.Identifier(),
		common.PathDomainPrivate.Identifier(),
		common.PathDomainStorage.Identifier(),
	}

	owner, err := common.BytesToAddress(adr[:])
	if err != nil {
		panic(err)
	}

	for _, domain := range domains {
		storageMap := storage.GetStorageMap(owner, domain)
		itr := storageMap.Iterator()
		key, value := itr.Next()
		for value != nil {
			r.iterateChildren(append([]string{domain}, key), adr, value)
			key, value = itr.Next()
		}
	}
}
