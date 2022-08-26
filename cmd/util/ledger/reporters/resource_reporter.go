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
	"github.com/onflow/cadence/runtime/sema"

	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

const ConstantSizeReporterReportPrefix = "constant_size_report"

var numberOfCompositeTypes uint64
var numberOfCompositeTypesWithConstantSizeChildren uint64

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
	return "Resource Reporter"
}

func (r *ConstantSizeReporter) Report(payloads []ledger.Payload, commit ledger.State) error {
	r.rw = r.rwf.ReportWriter(ConstantSizeReporterReportPrefix)
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

	r.log.Info().Msgf(">>>>>> %d of %d", numberOfCompositeTypesWithConstantSizeChildren, numberOfCompositeTypes)

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
							compositeType := composite.StaticType(inter).(interpreter.CompositeStaticType)
							if allFieldsAreConstantSized(r.log, inter, compositeType) {
								atomic.AddUint64(&numberOfCompositeTypesWithConstantSizeChildren, 1)
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

func allFieldsAreConstantSized(logger zerolog.Logger, inter *interpreter.Interpreter, typ interpreter.CompositeStaticType) bool {
	semaType, err := inter.ConvertStaticToSemaType(typ)
	if err != nil {
		// something wrong
		return false
	}

	compositeSemaType := semaType.(*sema.CompositeType)
	for _, fieldName := range compositeSemaType.Fields {
		field, ok := compositeSemaType.Members.Get(fieldName)
		if !ok {
			continue
		}

		fieldSemaType := field.TypeAnnotation.Type
		fieldStaticType := interpreter.ConvertSemaToStaticType(nil, fieldSemaType)

		if !isConstantSize(logger, inter, fieldStaticType) {
			return false
		}
	}

	return true
}

func isConstantSize(logger zerolog.Logger, inter *interpreter.Interpreter, typ interpreter.StaticType) bool {
	// only for composite types you could iterate over fields
	// for regular maps and array return false
	switch typ := typ.(type) {
	case interpreter.CompositeStaticType:
		return allFieldsAreConstantSized(logger, inter, typ)
	case interpreter.DictionaryStaticType,
		interpreter.ArrayStaticType:
		return false
	case interpreter.OptionalStaticType:
		// check the inner type
		return isConstantSize(logger, inter, typ.Type)
	case interpreter.PrimitiveStaticType:
		switch typ {
		case interpreter.PrimitiveStaticTypeString,
			interpreter.PrimitiveStaticTypeInt,
			interpreter.PrimitiveStaticTypeUInt,
			interpreter.PrimitiveStaticTypePath,
			interpreter.PrimitiveStaticTypeCapability,
			interpreter.PrimitiveStaticTypeMetaType:
			return false
		case interpreter.PrimitiveStaticTypeBool,
			interpreter.PrimitiveStaticTypeCharacter,
			interpreter.PrimitiveStaticTypeAddress,
			interpreter.PrimitiveStaticTypeVoid,
			interpreter.PrimitiveStaticTypeInt8,
			interpreter.PrimitiveStaticTypeInt16,
			interpreter.PrimitiveStaticTypeInt32,
			interpreter.PrimitiveStaticTypeInt64,
			interpreter.PrimitiveStaticTypeInt128,
			interpreter.PrimitiveStaticTypeInt256,
			interpreter.PrimitiveStaticTypeUInt8,
			interpreter.PrimitiveStaticTypeUInt16,
			interpreter.PrimitiveStaticTypeUInt32,
			interpreter.PrimitiveStaticTypeUInt64,
			interpreter.PrimitiveStaticTypeUInt128,
			interpreter.PrimitiveStaticTypeUInt256,
			interpreter.PrimitiveStaticTypeWord8,
			interpreter.PrimitiveStaticTypeWord16,
			interpreter.PrimitiveStaticTypeWord32,
			interpreter.PrimitiveStaticTypeWord64,
			interpreter.PrimitiveStaticTypeFix64,
			interpreter.PrimitiveStaticTypeUFix64:
			return true
		default:
			logger.Warn().Msgf("unknown type found %T", typ)
			return false
		}
	default:
		logger.Warn().Msgf("unknown type found %T", typ)
		return false
		// panic(fmt.Errorf("unknown type found %T", value))
	}
}
