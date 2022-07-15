package reporters

import (
	"fmt"
	"runtime"
	"sync"

	"github.com/rs/zerolog"
	"github.com/schollz/progressbar/v3"

	cadenceRuntime "github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

const ResourceUUIDReporterReportPrefix = "resource_uuid_report"

// ResourceUUIDReporter iterates through stored cadence values over all accounts
// and collects some data one resource uuids
type ResourceUUIDReporter struct {
	log      zerolog.Logger
	chain    flow.Chain
	rwf      ReportWriterFactory
	rw       ReportWriter
	progress *progressbar.ProgressBar
}

func NewResourceUUIDReporter(logger zerolog.Logger, rwf ReportWriterFactory, chain flow.Chain) *ResourceUUIDReporter {
	return &ResourceUUIDReporter{
		log:   logger,
		rwf:   rwf,
		chain: chain,
	}
}

func (r *ResourceUUIDReporter) Name() string {
	return "Resource uuid reporter"
}

type ResourcePoint struct {
	// Path is the storage path of the composite the vault was found in
	Path string `json:"path"`
	// Address is the owner of the composite the vault was found in
	Address string `json:"address"`
	// token type
	TypeID string `json:"type_id"`
	// UUID is the uuit of the resource.
	UUID uint64 `json:"uuid"`
}

func (r *ResourceUUIDReporter) Report(payloads []ledger.Payload, commit ledger.State) error {
	r.rw = r.rwf.ReportWriter(FungibleTokenTrackerReportPrefix)
	defer r.rw.Close()

	wg := &sync.WaitGroup{}

	// we need to shard by owner, otherwise ledger won't be thread-safe
	addressCount := 0
	payloadsByOwner := make(map[flow.Address][]ledger.Payload)

	for _, pay := range payloads {
		owner := flow.BytesToAddress(pay.Key.KeyParts[0].Value)
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

func (r *ResourceUUIDReporter) worker(
	jobs <-chan job,
	wg *sync.WaitGroup) {
	for j := range jobs {

		view := migrations.NewView(j.payloads)
		st := state.NewState(view)
		sth := state.NewStateHolder(st)
		accounts := state.NewAccounts(sth)
		storage := cadenceRuntime.NewStorage(
			&migrations.AccountsAtreeLedger{Accounts: accounts},
			nil,
		)

		owner, err := common.BytesToAddress(j.owner[:])
		if err != nil {
			panic(err)
		}

		inter := &interpreter.Interpreter{}
		for _, domain := range domains {
			storageMap := storage.GetStorageMap(owner, domain, true)
			itr := storageMap.Iterator(inter)
			key, value := itr.Next()
			for value != nil {
				r.iterateChildren(append([]string{domain}, key), j.owner, value)
				key, value = itr.Next()
			}
		}

		err = r.progress.Add(1)
		if err != nil {
			panic(err)
		}
	}

	wg.Done()
}

func (r *ResourceUUIDReporter) iterateChildren(tr trace, addr flow.Address, value interpreter.Value) {

	compValue, ok := value.(*interpreter.CompositeValue)
	if !ok {
		return
	}

	// because compValue.Kind == common.CompositeKindResource
	// we could pass nil to the IsResourceKinded method
	inter := &interpreter.Interpreter{}
	if compValue.IsResourceKinded(nil) {
		var uuid uint64
		uuidData := compValue.GetField(inter, nil, "uuid")
		uuidValue, ok := uuidData.(interpreter.UInt64Value)
		if ok {
			uuid = uint64(uuidValue)
		}
		fmt.Println("uuid", uuid)
		// uuid := uint64(compValue.GetField(inter, nil, "uuid").(interpreter.UInt64Value))
		r.rw.Write(ResourcePoint{
			Path:    tr.String(),
			Address: addr.Hex(),
			TypeID:  string(compValue.TypeID()),
			UUID:    uuid,
		})

		// iterate over fields of the composite value (skip the ones that are not resource typed)
		compValue.ForEachField(inter,
			func(key string, value interpreter.Value) {
				r.iterateChildren(append(tr, key), addr, value)
			})
	}
}
