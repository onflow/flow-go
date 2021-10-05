package reporters

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/schollz/progressbar/v3"

	"github.com/onflow/atree"

	cadenceRuntime "github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

// BalanceReporter iterates through registers getting the location and balance of all FlowVaults
type BalanceReporter struct {
	Log         zerolog.Logger
	RWF         ReportWriterFactory
	Chain       flow.Chain
	rw          ReportWriter
	progress    *progressbar.ProgressBar
	vaultTypeID string
}

func (r *BalanceReporter) Name() string {
	return "Balance Reporter"
}

type balanceDataPoint struct {
	// Path is the storage path of the composite the vault was found in
	Path string `json:"path"`
	// Address is the owner of the composite the vault was found in
	Address string `json:"address"`
	// LastComposite is the Composite directly containing the FlowVault
	LastComposite string `json:"last_composite"`
	// FirstComposite is the root composite at this path which directly or indirectly contains the vault
	FirstComposite string `json:"first_composite"`
	// Balance is the balance of the flow vault
	Balance uint64 `json:"balance"`
}

// Report creates a balance_report_*.json file that contains data on all FlowVaults in the state commitment.
// I recommend using gojq to browse through the data, because of the large uint64 numbers which jq won't be able to handle.
func (r *BalanceReporter) Report(payload []ledger.Payload) error {
	r.rw = r.RWF.ReportWriter("balance_report")
	defer r.rw.Close()

	r.progress = progressbar.Default(int64(len(payload)), "Processing:")
	r.vaultTypeID = fmt.Sprintf("A.%s.FlowToken.Vault", fvm.FlowTokenAddress(r.Chain).Hex())

	l := migrations.NewView(payload)

	wg := &sync.WaitGroup{}
	jobs := make(chan ledger.Payload)
	workerCount := runtime.NumCPU()

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go r.balanceReporterWorker(l, jobs, wg)
	}

	for _, p := range payload {
		jobs <- p
	}

	close(jobs)
	wg.Wait()

	err := r.progress.Finish()
	if err != nil {
		panic(err)
	}

	return nil
}

func (r *BalanceReporter) balanceReporterWorker(
	l state.View,
	jobs <-chan ledger.Payload,
	wg *sync.WaitGroup) {
	st := state.NewState(l)
	sth := state.NewStateHolder(st)
	accounts := state.NewAccounts(sth)
	storage := cadenceRuntime.NewStorage(
		&migrations.AccountsAtreeLedger{Accounts: accounts},
		func(f func(), _ func(metrics cadenceRuntime.Metrics, duration time.Duration)) {
			f()
		},
	)

	for payload := range jobs {
		r.handlePayload(payload, storage)

		err := r.progress.Add(1)
		if err != nil {
			panic(err)
		}
	}

	wg.Done()
}

func (r *BalanceReporter) handlePayload(p ledger.Payload, storage *cadenceRuntime.Storage) {
	id, err := migrations.KeyToRegisterID(p.Key)
	if err != nil {
		panic(err)
	}

	// Ignore known payload keys that are not Cadence values
	if state.IsFVMStateKey(id.Owner, id.Controller, id.Key) {
		return
	}
	if !(strings.HasPrefix(id.Key, common.PathDomainPublic.Identifier()) &&
		strings.HasPrefix(id.Key, common.PathDomainPrivate.Identifier()) &&
		strings.HasPrefix(id.Key, common.PathDomainStorage.Identifier())) {
		// this is not a storage path
		return
	}

	owner := common.BytesToAddress([]byte(id.Owner))
	decoder := interpreter.CBORDecMode.NewByteStreamDecoder(p.Value)

	storable, err := interpreter.DecodeStorable(decoder, atree.StorageIDUndefined)
	if err != nil || storable == nil {
		r.Log.
			Error().
			Err(err).
			Str("owner", owner.Hex()).
			Hex("key", []byte(id.Key)).
			Hex("storable", p.Value).
			Msg("Could not decode storable")
		return
	}
	storedValue, err := storable.StoredValue(storage)
	cValue := interpreter.MustConvertStoredValue(storedValue)
	if err != nil || cValue == nil {
		r.Log.
			Error().
			Err(err).
			Str("owner", owner.Hex()).
			Hex("key", []byte(id.Key)).
			Hex("storable", p.Value).
			Msg("Could not decode value")
		return
	}

	if id.Key == "contract\u001fFlowToken" {
		tokenSupply := uint64(cValue.(*interpreter.CompositeValue).GetField("totalSupply").(interpreter.UFix64Value))
		r.Log.Info().Uint64("tokenSupply", tokenSupply).Msg("total token supply")
	}

	lastComposite := "none"
	firstComposite := ""

	balanceVisitor := &interpreter.EmptyVisitor{
		CompositeValueVisitor: func(inter *interpreter.Interpreter, value *interpreter.CompositeValue) bool {
			if firstComposite == "" {
				firstComposite = string(value.TypeID())
			}

			if string(value.TypeID()) == r.vaultTypeID {
				b := uint64(value.GetField("balance").(interpreter.UFix64Value))
				if b == 0 {
					// ignore 0 balance results
					return false
				}

				r.rw.Write(balanceDataPoint{
					Path:           id.Key,
					Address:        flow.BytesToAddress([]byte(id.Owner)).Hex(),
					LastComposite:  lastComposite,
					FirstComposite: firstComposite,
					Balance:        b,
				})

				return false
			}

			lastComposite = string(value.TypeID())
			return true
		},
	}

	inter, err := interpreter.NewInterpreter(nil, common.StringLocation("somewhere"))
	if err != nil {
		r.Log.
			Error().
			Err(err).
			Str("owner", owner.Hex()).
			Hex("key", []byte(id.Key)).
			Msg("Could not create interpreter")
		return
	}
	cValue.Accept(inter, balanceVisitor)
}
