package migrations

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"path"
	"reflect"
	"strings"
	"time"
	"unsafe"

	"github.com/onflow/atree"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/stdlib"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
	"github.com/rs/zerolog"
	"github.com/schollz/progressbar/v3"
)

type OrderedMapMigration struct {
	Log         zerolog.Logger
	OutputDir   string
	accounts    state.Accounts
	programs    *programs.Programs
	reportFile  *os.File
	newStorage  *runtime.Storage
	Interpreter *interpreter.Interpreter
	ledgerView  *view

	progress *progressbar.ProgressBar
}

func (m *OrderedMapMigration) filename() string {
	return path.Join(m.OutputDir, fmt.Sprintf("migration_report_%d.txt", int32(time.Now().Unix())))
}

func (m *OrderedMapMigration) Migrate(payloads []ledger.Payload) ([]ledger.Payload, error) {

	filename := m.filename()
	m.Log.Info().Msgf("Running ordered map storage migration. Saving report to %s.", filename)

	reportFile, err := os.Create(filename)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = reportFile.Close()
		if err != nil {
			panic(err)
		}
	}()

	m.reportFile = reportFile

	total := int64(len(payloads))
	m.progress = progressbar.Default(total, "Migrating:")

	storagePayloads, err := m.initialize(payloads)
	if err != nil {
		panic(err)
	}
	return m.migrate(storagePayloads)
}

func (m *OrderedMapMigration) initPersistentSlabStorage(v *view) {
	st := state.NewState(
		v,
		state.WithMaxInteractionSizeAllowed(math.MaxUint64),
	)
	stateHolder := state.NewStateHolder(st)
	accounts := state.NewAccounts(stateHolder)

	m.newStorage = runtime.NewStorage(
		NewAccountsAtreeLedger(accounts),
	)
}

func (m *OrderedMapMigration) initIntepreter() {
	inter, err := interpreter.NewInterpreter(
		nil,
		nil,
		interpreter.WithStorage(m.newStorage),
		interpreter.WithImportLocationHandler(
			func(inter *interpreter.Interpreter, location common.Location) interpreter.Import {
				var program *interpreter.Program
				if location == stdlib.CryptoChecker.Location {
					program = interpreter.ProgramFromChecker(stdlib.CryptoChecker)
				} else {
					var err error
					program, err = m.loadProgram(location)
					if err != nil {
						panic(err)
					}
				}

				subInterpreter, err := inter.NewSubInterpreter(program, location)
				if err != nil {
					panic(err)
				}

				return interpreter.InterpreterImport{
					Interpreter: subInterpreter,
				}
			},
		),
	)

	if err != nil {
		panic(fmt.Errorf(
			"failed to create interpreter: %w",
			err,
		))
	}

	m.Interpreter = inter
}

func (m *OrderedMapMigration) loadProgram(
	location common.Location,
) (
	*interpreter.Program,
	error,
) {
	program, _, ok := m.programs.Get(location)
	if ok {
		return program, nil
	}

	addressLocation, ok := location.(common.AddressLocation)
	if !ok {
		return nil, fmt.Errorf(
			"cannot load program for unsupported non-address location: %s",
			location,
		)
	}

	contractCode, err := m.accounts.GetContract(
		addressLocation.Name,
		flow.Address(addressLocation.Address),
	)
	if err != nil {
		return nil, err
	}

	rt := runtime.NewInterpreterRuntime()
	program, err = rt.ParseAndCheckProgram(
		contractCode,
		runtime.Context{
			Interface: migrationRuntimeInterface{
				m.accounts,
				m.programs,
			},
			Location: location,
		},
	)
	if err != nil {
		return nil, err
	}

	m.programs.Set(location, program, nil)

	return program, nil
}

type Pair = struct {
	Key   string
	Value []byte
}

type RawStorable []byte

func (r RawStorable) Encode(enc *atree.Encoder) error {
	return enc.CBOR.EncodeRawBytes(r)
}

func (r RawStorable) ByteSize() uint32 {
	return uint32(len(r))
}

func (r RawStorable) StoredValue(storage atree.SlabStorage) (atree.Value, error) {
	return r, nil
}

func (r RawStorable) ChildStorables() []atree.Storable {
	return nil
}

func (r RawStorable) Storable(_ atree.SlabStorage, _ atree.Address, _ uint64) (atree.Storable, error) {
	return r, nil
}

func splitPayloads(inp []ledger.Payload) (fvmPayloads []ledger.Payload, storagePayloads []ledger.Payload, slabPayloads []ledger.Payload) {
	for _, p := range inp {
		if state.IsFVMStateKey(
			string(p.Key.KeyParts[0].Value),
			string(p.Key.KeyParts[1].Value),
			string(p.Key.KeyParts[2].Value),
		) {
			fvmPayloads = append(fvmPayloads, p)
			continue
		}
		if bytes.HasPrefix(p.Key.KeyParts[2].Value, []byte(atree.LedgerBaseStorageSlabPrefix)) {
			slabPayloads = append(slabPayloads, p)
			continue
		}
		// otherwise this is a storage payload
		storagePayloads = append(storagePayloads, p)
	}
	return
}

func (m *OrderedMapMigration) initialize(payload []ledger.Payload) ([]ledger.Payload, error) {
	fvmPayloads, storagePayloads, slabPayloads := splitPayloads(payload)
	if len(slabPayloads) != 0 {
		return nil, fmt.Errorf(
			"slab storages are not empty: found %d",
			len(slabPayloads),
		)
	}

	m.ledgerView = NewView(fvmPayloads)
	m.initPersistentSlabStorage(m.ledgerView)
	m.initIntepreter()
	return storagePayloads, nil
}

func (m *OrderedMapMigration) migrate(storagePayloads []ledger.Payload) ([]ledger.Payload, error) {
	groupedByOwnerAndDomain := make(map[string](map[string][]Pair))

	for _, p := range storagePayloads {
		owner, entry :=
			string(p.Key.KeyParts[0].Value),
			string(p.Key.KeyParts[2].Value)
		// if the entry doesn't contain a separator, we just ignore it
		// since it is storage metadata and does not need migration
		if !strings.Contains(entry, "\x1f") {
			continue
		}
		splitEntry := strings.Split(entry, "\x1f")
		domain, key := splitEntry[0], splitEntry[1]
		value := p.Value

		domainMap, domainOk := groupedByOwnerAndDomain[owner]
		if !domainOk {
			domainMap = make(map[string][]Pair)
		}
		keyValuePairs, orderedOk := domainMap[domain]
		if !orderedOk {
			keyValuePairs = make([]Pair, 0)
		}
		domainMap[domain] = append(keyValuePairs, Pair{Key: key, Value: value})
		groupedByOwnerAndDomain[owner] = domainMap
	}

	for owner, domainMaps := range groupedByOwnerAndDomain {
		for domain, keyValuePairs := range domainMaps {
			storageMap := m.newStorage.GetStorageMap(common.MustBytesToAddress([]byte(owner)), domain)
			// in the interest of not having to update Cadence and break its abstractions to allow
			// this one-time migration, just use reflection to grab the unexported field
			unsafeOrderedMap := reflect.ValueOf(storageMap).Elem().FieldByName("orderedMap")
			orderedMap := reflect.NewAt(
				unsafeOrderedMap.Type(),
				unsafe.Pointer(unsafeOrderedMap.UnsafeAddr()),
			).Elem().Interface().(*atree.OrderedMap)
			for _, pair := range keyValuePairs {
				orderedMap.Set(
					// these should be rawstorables too probably
					interpreter.StringAtreeComparator,
					interpreter.StringAtreeHashInput,
					interpreter.StringAtreeValue(pair.Key),
					RawStorable(pair.Value),
				)
			}
		}
	}

	// we don't need to update any contracts in this migration
	err := m.newStorage.Commit(m.Interpreter, false)
	if err != nil {
		return nil, fmt.Errorf("failed to migrate payloads: %w", err)
	}

	return m.ledgerView.Payloads(), nil
}
