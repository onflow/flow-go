package migrations

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"path"
	"reflect"
	"time"

	"github.com/onflow/atree"
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/stdlib"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
	"github.com/opentracing/opentracing-go"
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
	interpreter *interpreter.Interpreter

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

	total := int64(len(payloads) * 3)
	m.progress = progressbar.Default(total, "Migrating:")

	return m.migrate(payloads)
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

	m.interpreter = inter
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

// migrationRuntimeInterface

type migrationRuntimeInterface struct {
	accounts state.Accounts
	programs *programs.Programs
}

func (m migrationRuntimeInterface) ResolveLocation(
	identifiers []runtime.Identifier,
	location runtime.Location,
) ([]runtime.ResolvedLocation, error) {
	panic("unexpected ResolveLocation call")
}

func (m migrationRuntimeInterface) GetCode(location runtime.Location) ([]byte, error) {
	panic("unexpected GetCode call")
}

func (m migrationRuntimeInterface) GetProgram(location runtime.Location) (*interpreter.Program, error) {
	panic("unexpected GetProgram call")
}

func (m migrationRuntimeInterface) SetProgram(location runtime.Location, program *interpreter.Program) error {
	panic("unexpected SetProgram call")
}

func (m migrationRuntimeInterface) GetValue(_, _ []byte) (value []byte, err error) {
	panic("unexpected GetValue call")
}

func (m migrationRuntimeInterface) SetValue(_, _, _ []byte) (err error) {
	panic("unexpected SetValue call")
}

func (m migrationRuntimeInterface) CreateAccount(_ runtime.Address) (address runtime.Address, err error) {
	panic("unexpected CreateAccount call")
}

func (m migrationRuntimeInterface) AddEncodedAccountKey(_ runtime.Address, _ []byte) error {
	panic("unexpected AddEncodedAccountKey call")
}

func (m migrationRuntimeInterface) RevokeEncodedAccountKey(_ runtime.Address, _ int) (publicKey []byte, err error) {
	panic("unexpected RevokeEncodedAccountKey call")
}

func (m migrationRuntimeInterface) AddAccountKey(
	_ runtime.Address,
	_ *runtime.PublicKey,
	_ runtime.HashAlgorithm,
	_ int,
) (*runtime.AccountKey, error) {
	panic("unexpected AddAccountKey call")
}

func (m migrationRuntimeInterface) GetAccountKey(_ runtime.Address, _ int) (*runtime.AccountKey, error) {
	panic("unexpected GetAccountKey call")
}

func (m migrationRuntimeInterface) RevokeAccountKey(_ runtime.Address, _ int) (*runtime.AccountKey, error) {
	panic("unexpected RevokeAccountKey call")
}

func (m migrationRuntimeInterface) UpdateAccountContractCode(_ runtime.Address, _ string, _ []byte) (err error) {
	panic("unexpected UpdateAccountContractCode call")
}

func (m migrationRuntimeInterface) GetAccountContractCode(
	address runtime.Address,
	name string,
) (code []byte, err error) {
	panic("unexpected GetAccountContractCode call")
}

func (m migrationRuntimeInterface) RemoveAccountContractCode(_ runtime.Address, _ string) (err error) {
	panic("unexpected RemoveAccountContractCode call")
}

func (m migrationRuntimeInterface) GetSigningAccounts() ([]runtime.Address, error) {
	panic("unexpected GetSigningAccounts call")
}

func (m migrationRuntimeInterface) ProgramLog(_ string) error {
	panic("unexpected ProgramLog call")
}

func (m migrationRuntimeInterface) EmitEvent(_ cadence.Event) error {
	panic("unexpected EmitEvent call")
}

func (m migrationRuntimeInterface) ValueExists(_, _ []byte) (exists bool, err error) {
	panic("unexpected ValueExists call")
}

func (m migrationRuntimeInterface) GenerateUUID() (uint64, error) {
	panic("unexpected GenerateUUID call")
}

func (m migrationRuntimeInterface) GetComputationLimit() uint64 {
	panic("unexpected GetComputationLimit call")
}

func (m migrationRuntimeInterface) SetComputationUsed(_ uint64) error {
	panic("unexpected SetComputationUsed call")
}

func (m migrationRuntimeInterface) DecodeArgument(_ []byte, _ cadence.Type) (cadence.Value, error) {
	panic("unexpected DecodeArgument call")
}

func (m migrationRuntimeInterface) GetCurrentBlockHeight() (uint64, error) {
	panic("unexpected GetCurrentBlockHeight call")
}

func (m migrationRuntimeInterface) GetBlockAtHeight(_ uint64) (block runtime.Block, exists bool, err error) {
	panic("unexpected GetBlockAtHeight call")
}

func (m migrationRuntimeInterface) UnsafeRandom() (uint64, error) {
	panic("unexpected UnsafeRandom call")
}

func (m migrationRuntimeInterface) VerifySignature(
	_ []byte,
	_ string,
	_ []byte,
	_ []byte,
	_ runtime.SignatureAlgorithm,
	_ runtime.HashAlgorithm,
) (bool, error) {
	panic("unexpected VerifySignature call")
}

func (m migrationRuntimeInterface) Hash(_ []byte, _ string, _ runtime.HashAlgorithm) ([]byte, error) {
	panic("unexpected Hash call")
}

func (m migrationRuntimeInterface) GetAccountBalance(_ common.Address) (value uint64, err error) {
	panic("unexpected GetAccountBalance call")
}

func (m migrationRuntimeInterface) GetAccountAvailableBalance(_ common.Address) (value uint64, err error) {
	panic("unexpected GetAccountAvailableBalance call")
}

func (m migrationRuntimeInterface) GetStorageUsed(_ runtime.Address) (value uint64, err error) {
	panic("unexpected GetStorageUsed call")
}

func (m migrationRuntimeInterface) GetStorageCapacity(_ runtime.Address) (value uint64, err error) {
	panic("unexpected GetStorageCapacity call")
}

func (m migrationRuntimeInterface) ImplementationDebugLog(_ string) error {
	panic("unexpected ImplementationDebugLog call")
}

func (m migrationRuntimeInterface) ValidatePublicKey(_ *runtime.PublicKey) (bool, error) {
	panic("unexpected ValidatePublicKey call")
}

func (m migrationRuntimeInterface) GetAccountContractNames(_ runtime.Address) ([]string, error) {
	panic("unexpected GetAccountContractNames call")
}

func (m migrationRuntimeInterface) AllocateStorageIndex(_ []byte) (atree.StorageIndex, error) {
	panic("unexpected AllocateStorageIndex call")
}

func (m migrationRuntimeInterface) AggregateBLSPublicKeys(_ []*runtime.PublicKey) (*runtime.PublicKey, error) {
	panic("unexpected AggregateBLSPublicKeys call")
}

func (m migrationRuntimeInterface) BLSVerifyPOP(pk *runtime.PublicKey, s []byte) (bool, error) {
	panic("unexpected BLSVerifyPOP call")
}

func (m migrationRuntimeInterface) AggregateBLSSignatures(sigs [][]byte) ([]byte, error) {
	panic("unexpected AggregateBLSSignatures call")
}

func (m migrationRuntimeInterface) ResourceOwnerChanged(resource *interpreter.CompositeValue, oldOwner common.Address, newOwner common.Address) {
	panic("unexpected ResourceOwnerChanged call")
}

func (m migrationRuntimeInterface) RecordTrace(operation string, location common.Location, duration time.Duration, logs []opentracing.LogRecord) {
	panic("unexpected RecordTrace call")
}

type Pair = struct {
	Key   string
	Value []byte
}

type RawStorable []byte

func (r RawStorable) Encode(enc *atree.Encoder) error {
	return enc.CBOR.EncodeRawBytes(r)
}

func getUintCBORSize(v uint64) uint32 {
	if v <= 23 {
		return 1
	}
	if v <= math.MaxUint8 {
		return 2
	}
	if v <= math.MaxUint16 {
		return 3
	}
	if v <= math.MaxUint32 {
		return 5
	}
	return 9
}

func (r RawStorable) ByteSize() uint32 {
	length := len(r)
	if length == 0 {
		return 1
	}
	return getUintCBORSize(uint64(length)) + uint32(length)
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

func rawStorableHashInput(v atree.Value, _ []byte) ([]byte, error) {
	return []byte(v.(RawStorable)), nil
}

func rawStorableComparator(storage atree.SlabStorage, value atree.Value, otherStorable atree.Storable) (bool, error) {
	otherValue, err := otherStorable.StoredValue(storage)
	if err != nil {
		return false, err
	}

	result := bytes.Compare(value.(RawStorable), otherValue.(RawStorable))

	return result == 0, nil
}

func (m *OrderedMapMigration) migrate(payload []ledger.Payload) ([]ledger.Payload, error) {
	ledgerView := NewView(payload)
	m.initPersistentSlabStorage(ledgerView)
	m.initIntepreter()

	sortedByOwnerAndDomain := make(map[string](map[string][]Pair))

	for _, p := range payload {
		owner, domain, key :=
			string(p.Key.KeyParts[0].Value),
			string(p.Key.KeyParts[1].Value),
			string(p.Key.KeyParts[2].Value)
		value := p.Value

		domainMap, domainOk := sortedByOwnerAndDomain[owner]
		if !domainOk {
			domainMap = make(map[string][]Pair)
		}
		keyValuePairs, orderedOk := domainMap[domain]
		if !orderedOk {
			keyValuePairs = make([]Pair, 0)
		}
		domainMap[domain] = append(keyValuePairs, Pair{Key: key, Value: value})
		sortedByOwnerAndDomain[owner] = domainMap
	}

	for owner, domainMaps := range sortedByOwnerAndDomain {
		for domain, keyValuePairs := range domainMaps {
			for _, pair := range keyValuePairs {
				storageMap := m.newStorage.GetStorageMap(common.BytesToAddress([]byte(owner)), domain)
				orderedMap := reflect.ValueOf(storageMap).Elem().FieldByName("orderedMap").Interface().(*atree.OrderedMap)
				orderedMap.Set(
					rawStorableComparator,
					rawStorableHashInput,
					RawStorable(pair.Key),
					RawStorable(pair.Value),
				)
			}
		}
	}

	// we don't need to update any contracts in this migration
	err := m.newStorage.Commit(m.interpreter, false)
	if err != nil {
		return nil, fmt.Errorf("failed to migrate payloads: %w", err)
	}

	return ledgerView.Payloads(), nil
}
