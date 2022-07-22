package reporters

import (
	"math"
	"runtime"
	"sync"
	"sync/atomic"

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

const ConstantSizeReporterReportPrefix = "constant_size_report"

var numberOfCompositeTypes uint64
var numberOfCompositeTypesWithConstantChildren uint64

type ConstantSizeReporter struct {
	log      zerolog.Logger
	rwf      ReportWriterFactory
	rw       ReportWriter
	progress *progressbar.ProgressBar
}

func NewConstantSizeReporter(logger zerolog.Logger, rwf ReportWriterFactory) *ConstantSizeReporter {
	return &ConstantSizeReporter{
		log: logger,
		rwf: rwf,
	}
}

func (r *ConstantSizeReporter) Name() string {
	return "Constant Size Reporter"
}

func (r *ConstantSizeReporter) Report(payloads []ledger.Payload, commit ledger.State) error {
	r.rw = r.rwf.ReportWriter(ResourceUUIDReporterReportPrefix)
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

	r.log.Info().Msgf(">>>>>> %d of %d", numberOfCompositeTypesWithConstantChildren, numberOfCompositeTypes)

	err := r.progress.Finish()
	if err != nil {
		panic(err)
	}

	return nil
}

func (r *ConstantSizeReporter) worker(
	jobs <-chan job,
	wg *sync.WaitGroup) {
	for j := range jobs {

		view := migrations.NewView(j.payloads)
		st := state.NewState(view, state.WithMaxInteractionSizeAllowed(math.MaxUint64))
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
			_, value := itr.Next()
			for value != nil {
				interpreter.InspectValue(
					inter,
					value,
					func(v interpreter.Value) bool {
						if composite, ok := v.(*interpreter.CompositeValue); ok {
							atomic.AddUint64(&numberOfCompositeTypes, 1)
							if allFieldsAreConstantSize(r.log, inter, composite) {
								atomic.AddUint64(&numberOfCompositeTypesWithConstantChildren, 1)
							}
						}

						return true
					},
				)

				_, value = itr.Next()
			}
		}

		err = r.progress.Add(1)
		if err != nil {
			panic(err)
		}
	}

	wg.Done()
}

func allFieldsAreConstantSize(logger zerolog.Logger, inter *interpreter.Interpreter, value *interpreter.CompositeValue) bool {
	// if any of children is not true return false
	foundANonConstant := false
	value.ForEachField(inter,
		func(_ string, value interpreter.Value) {
			if !IsConstantSize(logger, inter, value) {
				foundANonConstant = true
			}
		})

	return !foundANonConstant
}

func IsConstantSize(logger zerolog.Logger, inter *interpreter.Interpreter, value interpreter.Value) bool {
	// only for composite types you could iterate over feilds
	// for regular maps and array return false
	switch value.(type) {
	case *interpreter.CompositeValue:
		return allFieldsAreConstantSize(logger, inter, value.(*interpreter.CompositeValue))
	case *interpreter.DictionaryValue:
		return false
	case *interpreter.ArrayValue:
		return false
	case *interpreter.StringValue, interpreter.IntValue, interpreter.UIntValue,
		*interpreter.SomeValue, interpreter.PathValue, *interpreter.CapabilityValue,
		interpreter.LinkValue, interpreter.TypeValue:
		return false
	case interpreter.AddressValue:
		return true
	case interpreter.BoolValue, interpreter.CharacterValue, interpreter.NilValue, interpreter.VoidValue,
		interpreter.Int8Value, interpreter.Int16Value, interpreter.Int32Value, interpreter.Int64Value, interpreter.Int128Value, interpreter.Int256Value,
		interpreter.UInt8Value, interpreter.UInt16Value, interpreter.UInt32Value, interpreter.UInt64Value, interpreter.UInt128Value, interpreter.UInt256Value,
		interpreter.Word8Value, interpreter.Word16Value, interpreter.Word32Value, interpreter.Word64Value, interpreter.Fix64Value, interpreter.UFix64Value:
		return true

	default:
		logger.Warn().Msgf("unknown type found %T", value)
		return false
		// panic(fmt.Errorf("unknown type found %T", value))
	}
}
