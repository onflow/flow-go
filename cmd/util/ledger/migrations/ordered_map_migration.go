package migrations

import (
	"fmt"
	"math"
	"os"
	"path"
	"strings"
	"time"

	"github.com/onflow/atree"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/rs/zerolog"
	"github.com/schollz/progressbar/v3"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
)

type OrderedMapMigration struct {
	Log         zerolog.Logger
	OutputDir   string
	reportFile  *os.File
	NewStorage  *runtime.Storage
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

	m.NewStorage = runtime.NewStorage(
		NewAccountsAtreeLedger(accounts),
	)
}

func (m *OrderedMapMigration) initIntepreter() {
	inter, err := interpreter.NewInterpreter(
		nil,
		nil,
		interpreter.WithStorage(m.NewStorage),
	)

	if err != nil {
		panic(fmt.Errorf(
			"failed to create interpreter: %w",
			err,
		))
	}

	m.Interpreter = inter
}

type Pair = struct {
	Key   string
	Value []byte
}

type RawStorable []byte

func (RawStorable) IsValue() {}

func (v RawStorable) Accept(interpreter *interpreter.Interpreter, visitor interpreter.Visitor) {
	panic("unreachable")
}

func (RawStorable) Walk(_ func(interpreter.Value)) {
	// NO-OP
}

func (RawStorable) DynamicType(_ *interpreter.Interpreter, _ interpreter.SeenReferences) interpreter.DynamicType {
	panic("unreachable")
}

func (RawStorable) StaticType() interpreter.StaticType {
	panic("unreachable")
}

func (RawStorable) String() string {
	panic("unreachable")
}

func (v RawStorable) RecursiveString(_ interpreter.SeenReferences) string {
	panic("unreachable")
}

func (v RawStorable) ConformsToDynamicType(
	_ *interpreter.Interpreter,
	_ func() interpreter.LocationRange,
	dynamicType interpreter.DynamicType,
	_ interpreter.TypeConformanceResults,
) bool {
	panic("unreachable")
}

func (v RawStorable) Equal(_ *interpreter.Interpreter, _ func() interpreter.LocationRange, other interpreter.Value) bool {
	panic("unreachable")
}

func (RawStorable) NeedsStoreTo(_ atree.Address) bool {
	panic("unreachable")
}

func (RawStorable) IsResourceKinded(_ *interpreter.Interpreter) bool {
	panic("unreachable")
}

func (v RawStorable) Transfer(
	interpreter *interpreter.Interpreter,
	_ func() interpreter.LocationRange,
	_ atree.Address,
	remove bool,
	storable atree.Storable,
) interpreter.Value {
	panic("unreachable")
}

func (v RawStorable) Clone(_ *interpreter.Interpreter) interpreter.Value {
	panic("unreachable")
}

func (RawStorable) DeepRemove(_ *interpreter.Interpreter) {
	// NO-OP
}

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

func (m *OrderedMapMigration) initialize(payload []ledger.Payload) ([]ledger.Payload, error) {
	fvmPayloads, storagePayloads, slabPayloads := splitPayloads(payload)

	m.ledgerView = NewView(append(fvmPayloads, slabPayloads...))
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
		if !strings.HasPrefix(entry, "storage\x1f") &&
			!strings.HasPrefix(entry, "public\x1f") &&
			!strings.HasPrefix(entry, "contract\x1f") &&
			!strings.HasPrefix(entry, "private\x1f") {
			m.Log.Warn().Msgf("Ignoring key in storage payloads: %s", entry)
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
			address, err := common.BytesToAddress([]byte(owner))
			if err != nil {
				panic(err)
			}
			storageMap := m.NewStorage.GetStorageMap(address, domain)
			for _, pair := range keyValuePairs {
				storageMap.SetValue(
					m.Interpreter,
					pair.Key,
					RawStorable(pair.Value),
				)
			}
		}
	}

	// we don't need to update any contracts in this migration
	err := m.NewStorage.Commit(m.Interpreter, false)
	if err != nil {
		return nil, fmt.Errorf("failed to migrate payloads: %w", err)
	}

	return m.ledgerView.Payloads(), nil
}
