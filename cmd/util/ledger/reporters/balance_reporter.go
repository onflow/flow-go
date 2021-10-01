package reporters

import (
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
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

// BalanceReporter iterates through registers getting the location and balance of all FlowVaults
type BalanceReporter struct {
	Log            zerolog.Logger
	RWF            ReportWriterFactory
	rw             ReportWriter
	progress       *progressbar.ProgressBar
	rwts           ReportWriter
	addressMoments map[string]int
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

type moments struct {
	Address string `json:"address"`
	Moments int    `json:"moments"`
}

// Report creates a balance_report_*.json file that contains data on all FlowVaults in the state commitment.
// I recommend using gojq to browse through the data, because of the large uint64 numbers which jq won't be able to handle.
func (r *BalanceReporter) Report(payload []ledger.Payload) error {
	r.rw = r.RWF.ReportWriter("balance_report")
	defer r.rw.Close()
	r.rwts = r.RWF.ReportWriter("top_shot_report")
	defer r.rwts.Close()

	addressMoments := make(map[string]int)

	r.progress = progressbar.Default(int64(len(payload)), "Processing:")

	l := migrations.NewView(payload)

	wg := &sync.WaitGroup{}
	momentsWG := &sync.WaitGroup{}
	jobs := make(chan ledger.Payload)
	momentsChan := make(chan moments)
	workerCount := runtime.NumCPU()

	momentsWG.Add(1)
	go func() {
		for m := range momentsChan {
			if m.Moments > 0 {
				addressMoments[m.Address] += m.Moments
			}
		}
		momentsWG.Done()
	}()

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go r.balanceReporterWorker(l, jobs, momentsChan, wg)
	}

	for _, p := range payload {
		jobs <- p
	}

	close(jobs)
	wg.Wait()
	close(momentsChan)
	momentsWG.Wait()

	for a, n := range addressMoments {
		r.rwts.Write(moments{
			Address: a,
			Moments: n,
		})
	}

	err := r.progress.Finish()
	if err != nil {
		panic(err)
	}

	return nil
}

func (r *BalanceReporter) balanceReporterWorker(
	l state.View,
	jobs <-chan ledger.Payload,
	momentsChan chan<- moments,
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
		r.handlePayload(payload, storage, momentsChan)

		err := r.progress.Add(1)
		if err != nil {
			panic(err)
		}
	}

	wg.Done()
}

func (r *BalanceReporter) handlePayload(p ledger.Payload, storage *cadenceRuntime.Storage, momentsChan chan<- moments) {
	id, err := migrations.KeyToRegisterID(p.Key)
	if err != nil {
		panic(err)
	}

	// Ignore known payload keys that are not Cadence values
	if state.IsFVMStateKey(id.Owner, id.Controller, id.Key) {
		return
	}
	if strings.HasSuffix(id.Key, "\x24") {
		// this is a slab index
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

	m := 0

	balanceVisitor := &interpreter.EmptyVisitor{
		CompositeValueVisitor: func(inter *interpreter.Interpreter, value *interpreter.CompositeValue) bool {
			if firstComposite == "" {
				firstComposite = string(value.TypeID())
			}

			if string(value.TypeID()) == "A.1654653399040a61.FlowToken.Vault" {
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

			if string(value.TypeID()) == "A.0b2a3299cc857e29.TopShot.NFT" {
				m += 1

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

	momentsChan <- moments{
		Address: owner.Hex(),
		Moments: m,
	}
}
