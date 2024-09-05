package reporters

import (
	"runtime"
	"sync"

	"github.com/rs/zerolog"
	"github.com/schollz/progressbar/v3"

	cadenceRuntime "github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

const LinkReportPrefix = "link_report"

type LinkTracker struct {
	log          zerolog.Logger
	chain        flow.Chain
	rwf          ReportWriterFactory
	rw           ReportWriter
	progress     *progressbar.ProgressBar
	vaultTypeIDs map[string]bool
}

// func FlowTokenTypeID(chain flow.Chain) string {
// 	sc := systemcontracts.SystemContractsForChain(chain.ChainID())
// 	return fmt.Sprintf("A.%s.FlowToken.Vault", sc.FlowToken.Address.Hex())
// }

func NewLinkTracker(logger zerolog.Logger, rwf ReportWriterFactory, chain flow.Chain, vaultTypeIDs []string) *LinkTracker {
	ftt := &LinkTracker{
		log:          logger,
		rwf:          rwf,
		chain:        chain,
		vaultTypeIDs: make(map[string]bool),
	}
	for _, vt := range vaultTypeIDs {
		ftt.vaultTypeIDs[vt] = true
	}
	return ftt
}

func (r *LinkTracker) Name() string {
	return "Link Tracker"
}

type LinkDataPoint struct {
	Path    string `json:"path"`
	Address string `json:"address"`
	TypeID  string `json:"type_id"`
}

func (r *LinkTracker) Report(payloads []ledger.Payload, commit ledger.State) error {
	r.rw = r.rwf.ReportWriter(LinkReportPrefix)
	defer r.rw.Close()

	wg := &sync.WaitGroup{}

	// we need to shard by owner, otherwise ledger won't be thread-safe
	addressCount := 0
	payloadsByOwner := make(map[flow.Address][]ledger.Payload)

	for _, pay := range payloads {
		k, err := pay.Key()
		if err != nil {
			return nil
		}
		owner := flow.BytesToAddress(k.KeyParts[0].Value)
		if len(owner) > 0 { // ignoring payloads without ownership (fvm ones)
			m, ok := payloadsByOwner[owner]
			if !ok {
				payloadsByOwner[owner] = make([]ledger.Payload, 0)
				addressCount++
			}
			payloadsByOwner[owner] = append(m, pay)
		}
	}

	jobs := make(chan job, addressCount)
	r.progress = progressbar.Default(int64(addressCount), "Processing:")

	for k, v := range payloadsByOwner {
		jobs <- job{k, v}
	}

	close(jobs)

	workerCount := runtime.NumCPU()
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go r.worker(jobs, wg)
	}

	wg.Wait()

	err := r.progress.Finish()
	if err != nil {
		panic(err)
	}

	return nil
}

func (r *LinkTracker) worker(
	jobs <-chan job,
	wg *sync.WaitGroup) {
	for j := range jobs {

		txnState := state.NewTransactionState(
			NewStorageSnapshotFromPayload(j.payloads),
			state.DefaultParameters())
		accounts := environment.NewAccounts(txnState)
		storage := cadenceRuntime.NewStorage(
			&util.AccountsAtreeLedger{Accounts: accounts},
			nil,
		)

		owner, err := common.BytesToAddress(j.owner[:])
		if err != nil {
			panic(err)
		}

		inter, err := interpreter.NewInterpreter(nil, nil, &interpreter.Config{})
		if err != nil {
			panic(err)
		}

		domain := common.PathDomainPublic.Identifier()
		storageMap := storage.GetStorageMap(owner, domain, false)
		itr := storageMap.Iterator(inter)
		key, value := itr.Next()
		for value != nil {
			identifier := string(key.(interpreter.StringAtreeValue))
			r.iterateChildren(append([]string{domain}, identifier), j.owner, value)
			key, value = itr.Next()
		}

		err = r.progress.Add(1)
		if err != nil {
			panic(err)
		}
	}

	wg.Done()
}

func (r *LinkTracker) iterateChildren(tr trace, addr flow.Address, value interpreter.Value) {

	compValue, ok := value.(*interpreter.CompositeValue)
	if !ok {
		return
	}

	// because compValue.Kind == common.CompositeKindResource
	// we could pass nil to the IsResourceKinded method
	inter, err := interpreter.NewInterpreter(nil, nil, &interpreter.Config{})
	if err != nil {
		panic(err)
	}
	if compValue.IsResourceKinded(nil) {
		typeIDStr := string(compValue.TypeID())
		if _, ok := r.vaultTypeIDs[typeIDStr]; ok {
			b := uint64(compValue.GetField(
				inter,
				interpreter.EmptyLocationRange,
				"balance",
			).(interpreter.UFix64Value))
			if b > 0 {
				r.rw.Write(LinkDataPoint{
					Path:    tr.String(),
					Address: addr.Hex(),
					TypeID:  string(compValue.TypeID()),
				})
			}
		}

		// // iterate over fields of the composite value (skip the ones that are not resource typed)
		// compValue.ForEachField(inter,
		// 	func(key string, value interpreter.Value) (resume bool) {
		// 		r.iterateChildren(append(tr, key), addr, value)

		// 		// continue iteration
		// 		return true
		// 	},
		// 	interpreter.EmptyLocationRange,
		// )
	}
}
