package migrations

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"path"
	"strings"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/onflow/atree"
	"github.com/rs/zerolog"
	"github.com/schollz/progressbar/v3"

	execState "github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	newInter "github.com/onflow/cadence/runtime/interpreter"

	oldInter "github.com/onflow/cadence/v19/runtime/interpreter"
)

const cborTagStorageReference = 202

// \x1F = Information Separator One
//
const pathSeparator = "\x1F"

var storageReferenceEncodingStart = []byte{0xd8, cborTagStorageReference}

var storageMigrationV6DecMode = func() cbor.DecMode {
	decMode, err := cbor.DecOptions{
		IntDec:           cbor.IntDecConvertNone,
		MaxArrayElements: math.MaxInt32,
		MaxMapPairs:      math.MaxInt32,
		MaxNestedLevels:  256,
	}.DecMode()
	if err != nil {
		panic(err)
	}
	return decMode
}()

type storageFormatV6MigrationResult struct {
	key     ledger.Key
	payload *ledger.Payload
	err     error
}

// Base storage to be used by the persistent slab storage.
//
type encodingBaseStorage struct {
	*atree.InMemBaseStorage
	ReencodedPayloads []*ledger.Payload
}

var _ atree.BaseStorage = &encodingBaseStorage{}

func newEncodingBaseStorage() *encodingBaseStorage {
	return &encodingBaseStorage{
		InMemBaseStorage:  atree.NewInMemBaseStorage(),
		ReencodedPayloads: make([]*ledger.Payload, 0),
	}
}

func (e *encodingBaseStorage) Store(id atree.StorageID, value []byte) error {
	err := e.InMemBaseStorage.Store(id, value)
	if err != nil {
		return err
	}

	//// Sanity check: decode and see whether the values are decodable.
	//
	//decoder := newInter.CBORDecMode.NewByteStreamDecoder(value)
	//
	//_, err = newInter.DecodeStorable(decoder, id)
	//if err != nil {
	//	panic(err)
	//}

	// Add the encoded content to the payloads

	payload := ledger.Payload{

		Key: ledgerKeyFromStorageID(id),

		Value: newInter.PrependMagic(
			value,
			newInter.CurrentEncodingVersion,
		),
	}

	e.ReencodedPayloads = append(e.ReencodedPayloads, &payload)

	return nil
}

// delegationStorage is the storage implementation to be used by the
// new interpreter during value conversions. This is a delegation
// object and does not define any operations.
//
type delegationStorage struct {
	// Overrides the InMemoryStorage's storage operations (i.e: BasicSlabStorage)
	// using a PersistentSlabStorage.
	*atree.PersistentSlabStorage

	*newInter.InMemoryStorage
}

var _ newInter.Storage = &delegationStorage{}
var _ atree.SlabStorage = &delegationStorage{}

func newDelegationStorage(persistentSlabStorage *atree.PersistentSlabStorage) delegationStorage {
	inMemStorage := newInter.NewInMemoryStorage()
	return delegationStorage{
		PersistentSlabStorage: persistentSlabStorage,
		InMemoryStorage:       &inMemStorage,
	}
}

func ledgerKeyFromStorageID(id atree.StorageID) ledger.Key {
	return ledger.NewKey([]ledger.KeyPart{
		ledger.NewKeyPart(execState.KeyPartOwner, id.Address[:]),
		ledger.NewKeyPart(execState.KeyPartController, []byte{}),

		// TODO: Ude prefixed key. i.e:
		//  prefixedKey := []byte(LedgerBaseStorageSlabPrefix + string(id.Index[:]))
		ledger.NewKeyPart(execState.KeyPartKey, id.Index[:]),
	})
}

type storagePath struct {
	owner string
	key   string
}

type StorageFormatV6Migration struct {
	Log        zerolog.Logger
	OutputDir  string
	accounts   *state.Accounts
	programs   *programs.Programs
	reportFile *os.File
	storage    *atree.PersistentSlabStorage
	oldInter   *oldInter.Interpreter
	newInter   *newInter.Interpreter

	migratedPayloadPaths map[storagePath]bool
	deferredValuePaths   map[storagePath]bool
	progress             *progressbar.ProgressBar
	emptyDeferredValues  int
}

func (m *StorageFormatV6Migration) filename() string {
	return path.Join(m.OutputDir, fmt.Sprintf("migration_report_%d.csv", int32(time.Now().Unix())))
}

func (m *StorageFormatV6Migration) Migrate(payloads []ledger.Payload) ([]ledger.Payload, error) {

	filename := m.filename()
	m.Log.Info().Msgf("Running storage format V6 migration. Saving report to %s.", filename)

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

	return m.migrate(payloads)
}

func (m *StorageFormatV6Migration) migrate(payloads []ledger.Payload) ([]ledger.Payload, error) {
	m.Log.Info().Msg("Loading account contracts ...")

	m.accounts = m.getContractsOnlyAccounts(payloads)

	m.Log.Info().Msg("Loaded account contracts")

	m.programs = programs.NewEmptyPrograms()

	m.migratedPayloadPaths = make(map[storagePath]bool, 0)

	baseStorage := newEncodingBaseStorage()
	m.initPersistentSlabStorage(baseStorage)

	m.initNewInterpreter()
	m.initOldInterpreter(payloads)

	total := int64(len(payloads) * 3)
	m.progress = progressbar.Default(total, "Migrating:")

	m.deferredValuePaths = m.getDeferredKeys(payloads)

	// Convert payloads.
	//   - Cadence values are decoded and converted to new values.
	//   - Non-cadence values are added to the migrated payloads.

	m.Log.Info().Msg("Converting payloads...")

	migratedPayloads := make([]ledger.Payload, 0, len(payloads))

	for _, payload := range payloads {
		keyParts := payload.Key.KeyParts
		rawOwner := keyParts[0].Value
		rawKey := keyParts[2].Value

		result := m.migratePayload(payload)

		if result.err != nil {

			return nil, fmt.Errorf(
				"failed to migrate key: %q (owner: %x): %w",
				rawKey,
				rawOwner,
				result.err,
			)
		} else if result.payload != nil {
			migratedPayloads = append(migratedPayloads, *result.payload)
		} else {
			// Both nil means, value is decoded and converted.
			// Do the encoding at once at the end.
		}

		m.progress.Add(1)
	}
	m.progress.Clear()
	m.Log.Info().Msg("Converting payloads complete")

	// Encode the new values by calling `storage.Commit()`

	m.Log.Info().Msg("Re-encoding converted values...")
	m.progress.Add(1)

	err := m.storage.Commit()
	if err != nil {
		return nil, fmt.Errorf("failed to migrate payloads: %w", err)
	}

	// Add the encoded new values to the payloads

	currentProgress := m.progress.GetMax()
	m.progress.Clear()
	m.progress.Reset()
	m.progress.ChangeMax(len(payloads)*2 + len(baseStorage.ReencodedPayloads))
	m.progress.Set(currentProgress)

	for _, payload := range baseStorage.ReencodedPayloads {
		m.progress.Add(1)
		migratedPayloads = append(migratedPayloads, *payload)
	}
	m.progress.Finish()

	m.Log.Info().Msg("Re-encoding converted values complete")

	if m.emptyDeferredValues > 0 {
		m.progress.Clear()
		m.Log.Warn().Msgf("empty deferred values found: %d", m.emptyDeferredValues)
	}

	return migratedPayloads, nil
}

func (m *StorageFormatV6Migration) initPersistentSlabStorage(encodingStorage *encodingBaseStorage) {
	encMode, err := cbor.EncOptions{}.EncMode()
	if err != nil {
		panic(err)
	}

	decMode, err := cbor.DecOptions{}.DecMode()
	if err != nil {
		panic(err)
	}

	m.storage = atree.NewPersistentSlabStorage(
		encodingStorage,
		encMode,
		decMode,
	)
}

func (m *StorageFormatV6Migration) getContractsOnlyAccounts(payloads []ledger.Payload) *state.Accounts {
	var filteredPayloads []ledger.Payload

	for _, payload := range payloads {
		rawKey := string(payload.Key.KeyParts[2].Value)
		if strings.HasPrefix(rawKey, "contract_names") ||
			strings.HasPrefix(rawKey, "code.") ||
			rawKey == "exists" {

			filteredPayloads = append(filteredPayloads, payload)
		}
	}

	l := newView(filteredPayloads)
	st := state.NewState(l)
	sth := state.NewStateHolder(st)
	accounts := state.NewAccounts(sth)
	return accounts
}

func (m *StorageFormatV6Migration) getDeferredKeys(payloads []ledger.Payload) map[storagePath]bool {

	m.progress.Clear()
	m.Log.Info().Msgf("Collecting deferred keys...")

	deferredValuePaths := make(map[storagePath]bool, 0)
	for _, payload := range payloads {
		m.progress.Add(1)

		keyParts := payload.Key.KeyParts
		rawOwner := keyParts[0].Value
		rawController := keyParts[1].Value
		rawKey := keyParts[2].Value

		if state.IsFVMStateKey(
			string(rawOwner),
			string(rawController),
			string(rawKey),
		) {
			continue
		}

		value, version := oldInter.StripMagic(payload.Value)

		if version != oldInter.CurrentEncodingVersion {
			continue
		}

		err := storageMigrationV6DecMode.Valid(value)
		if err != nil {
			continue
		}

		// Decode the value

		rootValue, err, skip := m.decode(
			value,
			common.BytesToAddress(rawOwner),
			string(rawKey),
			version,
		)

		if skip || err != nil {
			continue
		}

		// Walk through values and find the deferred keys.

		oldInter.InspectValue(
			rootValue,
			func(inspectedValue oldInter.Value) bool {
				if dictionary, ok := inspectedValue.(*oldInter.DictionaryValue); ok {
					deferredKeys := dictionary.DeferredKeys()

					if deferredKeys != nil {
						deferredKeys.Foreach(func(key string, _ struct{}) {
							storageKey := strings.Join(
								[]string{
									dictionary.DeferredStorageKeyBase(),
									key,
								},
								pathSeparator,
							)

							deferredOwner := dictionary.DeferredOwner().Bytes()

							deferredValuePaths[storagePath{
								owner: string(deferredOwner),
								key:   storageKey,
							}] = true
						})

					}
				}

				return true
			},
		)
	}

	m.progress.Clear()
	m.Log.Info().Msgf("Deferred keys collected: %d", len(deferredValuePaths))

	return deferredValuePaths
}

func (m *StorageFormatV6Migration) migratePayload(payload ledger.Payload) storageFormatV6MigrationResult {

	migratedPayload, err := m.reencodePayload(payload)

	result := storageFormatV6MigrationResult{
		key: payload.Key,
	}

	if err != nil {
		result.err = err
	} else if migratedPayload != nil {
		if err := m.checkStorageFormat(*migratedPayload); err != nil {
			panic(fmt.Errorf("%w: key = %s", err, payload.Key.String()))
		}
		result.payload = migratedPayload
	}

	return result
}

func (m *StorageFormatV6Migration) checkStorageFormat(payload ledger.Payload) error {

	if !bytes.HasPrefix(payload.Value, []byte{0x0, 0xca, 0xde}) {
		return nil
	}

	_, version := newInter.StripMagic(payload.Value)
	if version != newInter.CurrentEncodingVersion {
		return fmt.Errorf("invalid version for key %s: %d", payload.Key.String(), version)
	}

	return nil
}

func (m *StorageFormatV6Migration) reencodePayload(payload ledger.Payload) (*ledger.Payload, error) {

	keyParts := payload.Key.KeyParts

	rawOwner := keyParts[0].Value
	rawController := keyParts[1].Value
	rawKey := keyParts[2].Value

	// Ignore known payload keys that are not Cadence values

	if state.IsFVMStateKey(
		string(rawOwner),
		string(rawController),
		string(rawKey),
	) {
		return &payload, nil
	}

	value, version := oldInter.StripMagic(payload.Value)

	if version != oldInter.CurrentEncodingVersion {
		return nil,
			fmt.Errorf(
				"invalid storage format version for key: %s: %d",
				rawKey,
				version,
			)
	}

	err := storageMigrationV6DecMode.Valid(value)
	if err != nil {
		return &payload, nil
	}

	err = m.decodeAndConvert(
		value,
		common.BytesToAddress(rawOwner),
		string(rawKey),
		version,
	)

	if err != nil {
		return nil,
			fmt.Errorf(
				"failed to decode and convert key: %s: %w\n\nvalue:\n%s\n\n%s",
				rawKey, err,
				hex.Dump(value),
				cborMeLink(value),
			)
	}

	return nil, nil
}

// Decode the value and cache it to be migrated later.
//
func (m *StorageFormatV6Migration) decodeAndConvert(
	data []byte,
	owner common.Address,
	key string,
	version uint16,
) (err error) {

	path := storagePath{
		owner: string(owner.Bytes()),
		key:   key,
	}

	// If it's a deferred value, then skip.
	if m.deferredValuePaths[path] {
		return nil
	}

	// Decode the value

	rootValue, err, skip := m.decode(data, owner, key, version)
	if skip {
		return nil
	}

	if err != nil {
		return err
	}

	converter := NewValueConverter(m.newInter, m.oldInter, m.storage)

	defer func() {
		if r := recover(); r != nil {
			m.Log.Debug().Msgf("failed to convert value: %s", r.(error).Error())
		}
	}()

	_ = converter.Convert(rootValue)

	// Mark the payload as 'migrated'.
	m.migratedPayloadPaths[path] = true

	return nil
}

func (m *StorageFormatV6Migration) initNewInterpreter() {
	inter, err := newInter.NewInterpreter(
		nil,
		nil,
		newInter.WithImportLocationHandler(
			func(inter *newInter.Interpreter, location common.Location) newInter.Import {
				program, err := m.loadProgram(location)
				if err != nil {
					panic(err)
				}

				subInter, err := inter.NewSubInterpreter(program, location)
				if err != nil {
					panic(err)
				}

				return newInter.InterpreterImport{
					Interpreter: subInter,
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

	inter.Storage = newDelegationStorage(m.storage)

	m.newInter = inter
}

func (m *StorageFormatV6Migration) initOldInterpreter(payloads []ledger.Payload) {
	// Convert old value to new value

	storageView := newView(payloads)

	inter, err := oldInter.NewInterpreter(
		nil,
		nil,
		oldInter.WithStorageReadHandler(
			func(inter *oldInter.Interpreter, owner common.Address, key string, deferred bool) oldInter.OptionalValue {

				ownerStr := string(owner.Bytes())

				if m.migratedPayloadPaths[storagePath{
					owner: ownerStr,
					key:   key,
				}] {
					panic(
						fmt.Errorf(
							"value is already migrated: owner: %s, key: %s",
							ownerStr,
							key,
						),
					)
				}

				registerValue, err := storageView.Get(ownerStr, "", key)
				if err != nil {
					panic(err)
				}

				if len(registerValue) == 0 {
					m.emptyDeferredValues += 1
					panic(&ValueNotFoundError{
						key: key,
					})
				}

				// Strip magic

				content, version := oldInter.StripMagic(registerValue)

				if version != oldInter.CurrentEncodingVersion {
					panic(fmt.Errorf(
						"invalid storage format version for key: %s (owner: %s): %d\ncontent: %b",
						key,
						owner,
						version,
						registerValue,
					))
				}

				err = storageMigrationV6DecMode.Valid(content)
				if err != nil {
					panic(fmt.Errorf(
						"invalid content for key: %s: %w\ncontent: %b",
						key,
						err,
						content,
					))
				}

				// Decode

				value, err, skip := m.decode(content, owner, key, oldInter.CurrentEncodingVersion)
				if skip || err != nil {
					panic(err)
				}

				return oldInter.NewSomeValueOwningNonCopying(value)
			},
		),
	)

	if err != nil {
		panic(fmt.Errorf(
			"failed to create interpreter: %w",
			err,
		))
	}

	m.oldInter = inter
}

func (m *StorageFormatV6Migration) decode(
	data []byte,
	owner common.Address,
	key string,
	version uint16,
) (oldInter.Value, error, bool) {

	path := []string{key}

	rootValue, err := oldInter.DecodeValue(data, &owner, path, version, nil)
	if err != nil {
		if tagErr, ok := err.(oldInter.UnsupportedTagDecodingError); ok &&
			tagErr.Tag == cborTagStorageReference &&
			bytes.Compare(data[:2], storageReferenceEncodingStart) == 0 {

			m.Log.Warn().
				Str("key", key).
				Str("owner", owner.String()).
				Msgf("DELETING unsupported storage reference")

			return nil, nil, true

		} else {
			return nil, fmt.Errorf(
				"failed to decode value: %w\n\nvalue:\n%s\n",
				err, hex.Dump(data),
			), true
		}
	}

	// Force decoding of all inner values

	oldInter.InspectValue(
		rootValue,
		func(inspectedValue oldInter.Value) bool {
			switch inspectedValue := inspectedValue.(type) {
			case *oldInter.CompositeValue:
				_ = inspectedValue.Fields()
			case *oldInter.ArrayValue:
				_ = inspectedValue.Elements()
			case *oldInter.DictionaryValue:
				_ = inspectedValue.Entries()
			}
			return true
		},
	)
	return rootValue, nil, false
}

func (m *StorageFormatV6Migration) loadProgram(
	location common.Location,
) (
	*newInter.Program,
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
			addressLocation,
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
	accounts *state.Accounts
	programs *programs.Programs
}

func (m migrationRuntimeInterface) ResolveLocation(
	identifiers []runtime.Identifier,
	location runtime.Location,
) ([]runtime.ResolvedLocation, error) {

	addressLocation, isAddress := location.(common.AddressLocation)

	// if the location is not an address location, e.g. an identifier location (`import Crypto`),
	// then return a single resolved location which declares all identifiers.
	if !isAddress {
		return []runtime.ResolvedLocation{
			{
				Location:    location,
				Identifiers: identifiers,
			},
		}, nil
	}

	// if the location is an address,
	// and no specific identifiers where requested in the import statement,
	// then fetch all identifiers at this address
	if len(identifiers) == 0 {
		address := flow.Address(addressLocation.Address)

		contractNames, err := m.accounts.GetContractNames(address)
		if err != nil {
			return nil, fmt.Errorf("ResolveLocation failed: %w", err)
		}

		// if there are no contractNames deployed,
		// then return no resolved locations
		if len(contractNames) == 0 {
			return nil, nil
		}

		identifiers = make([]runtime.Identifier, len(contractNames))

		for i := range identifiers {
			identifiers[i] = runtime.Identifier{
				Identifier: contractNames[i],
			}
		}
	}

	// return one resolved location per identifier.
	// each resolved location is an address contract location
	resolvedLocations := make([]runtime.ResolvedLocation, len(identifiers))
	for i := range resolvedLocations {
		identifier := identifiers[i]
		resolvedLocations[i] = runtime.ResolvedLocation{
			Location: common.AddressLocation{
				Address: addressLocation.Address,
				Name:    identifier.Identifier,
			},
			Identifiers: []runtime.Identifier{identifier},
		}
	}

	return resolvedLocations, nil
}

func (m migrationRuntimeInterface) GetCode(location runtime.Location) ([]byte, error) {
	contractLocation, ok := location.(common.AddressLocation)
	if !ok {
		return nil, fmt.Errorf("GetCode failed: expected AddressLocation")
	}

	add, err := m.accounts.GetContract(contractLocation.Name, flow.Address(contractLocation.Address))
	if err != nil {
		return nil, fmt.Errorf("GetCode failed: %w", err)
	}

	return add, nil
}

func (m migrationRuntimeInterface) GetProgram(location runtime.Location) (*newInter.Program, error) {
	program, _, ok := m.programs.Get(location)
	if ok {
		return program, nil
	}

	return nil, nil
}

func (m migrationRuntimeInterface) SetProgram(location runtime.Location, program *newInter.Program) error {
	m.programs.Set(location, program, nil)
	return nil
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
	return m.accounts.GetContract(name, flow.Address(address))
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

func cborMeLink(value []byte) string {
	return fmt.Sprintf("http://cbor.me/?bytes=%x", value)
}

// ValueConverter converts old cadence interpreter values
// to new cadence interpreter values.
//
type ValueConverter struct {
	result   newInter.Value
	storage  atree.SlabStorage
	newInter *newInter.Interpreter
	oldInter *oldInter.Interpreter
}

var _ oldInter.Visitor = &ValueConverter{}

func NewValueConverter(
	newInter *newInter.Interpreter,
	oldInter *oldInter.Interpreter,
	storage atree.SlabStorage,
) *ValueConverter {
	return &ValueConverter{
		storage:  storage,
		newInter: newInter,
		oldInter: oldInter,
	}
}

func (c *ValueConverter) Convert(value oldInter.Value) newInter.Value {
	prevResult := c.result
	c.result = nil

	defer func() {
		c.result = prevResult
	}()

	value.Accept(c.oldInter, c)

	if c.result == nil {
		panic("returned nil")
	}

	return c.result
}

func (c *ValueConverter) VisitValue(_ *oldInter.Interpreter, _ oldInter.Value) {
	panic("unreachable")
}

func (c *ValueConverter) VisitTypeValue(_ *oldInter.Interpreter, value oldInter.TypeValue) {
	c.result = newInter.TypeValue{
		Type: ConvertStaticType(value.Type),
	}
}

func (c *ValueConverter) VisitVoidValue(_ *oldInter.Interpreter, _ oldInter.VoidValue) {
	c.result = newInter.VoidValue{}
}

func (c *ValueConverter) VisitBoolValue(_ *oldInter.Interpreter, value oldInter.BoolValue) {
	c.result = newInter.BoolValue(value)
}

func (c *ValueConverter) VisitStringValue(_ *oldInter.Interpreter, value *oldInter.StringValue) {
	c.result = newInter.NewStringValue(value.Str)
}

func (c *ValueConverter) VisitArrayValue(_ *oldInter.Interpreter, value *oldInter.ArrayValue) bool {
	newElements := make([]newInter.Value, value.Count())

	for index, element := range value.Elements() {
		newElements[index] = c.Convert(element)
	}

	arrayStaticType := ConvertStaticType(value.StaticType()).(newInter.ArrayStaticType)

	c.result = newInter.NewArrayValueWithAddress(
		c.newInter,
		arrayStaticType,
		*value.Owner,
		newElements...,
	)

	// Do not descent. We already visited children here.
	return false
}

func (c *ValueConverter) VisitIntValue(_ *oldInter.Interpreter, value oldInter.IntValue) {
	c.result = newInter.NewIntValueFromBigInt(value.BigInt)
}

func (c *ValueConverter) VisitInt8Value(_ *oldInter.Interpreter, value oldInter.Int8Value) {
	c.result = newInter.Int8Value(value)
}

func (c *ValueConverter) VisitInt16Value(_ *oldInter.Interpreter, value oldInter.Int16Value) {
	c.result = newInter.Int16Value(value)
}

func (c *ValueConverter) VisitInt32Value(_ *oldInter.Interpreter, value oldInter.Int32Value) {
	c.result = newInter.Int32Value(value)
}

func (c *ValueConverter) VisitInt64Value(_ *oldInter.Interpreter, value oldInter.Int64Value) {
	c.result = newInter.Int64Value(value)
}

func (c *ValueConverter) VisitInt128Value(_ *oldInter.Interpreter, value oldInter.Int128Value) {
	c.result = newInter.NewInt128ValueFromBigInt(value.BigInt)
}

func (c *ValueConverter) VisitInt256Value(_ *oldInter.Interpreter, value oldInter.Int256Value) {
	c.result = newInter.NewInt256ValueFromBigInt(value.BigInt)
}

func (c *ValueConverter) VisitUIntValue(_ *oldInter.Interpreter, value oldInter.UIntValue) {
	c.result = newInter.NewUIntValueFromBigInt(value.BigInt)
}

func (c *ValueConverter) VisitUInt8Value(_ *oldInter.Interpreter, value oldInter.UInt8Value) {
	c.result = newInter.UInt8Value(value)
}

func (c *ValueConverter) VisitUInt16Value(_ *oldInter.Interpreter, value oldInter.UInt16Value) {
	c.result = newInter.UInt16Value(value)
}

func (c *ValueConverter) VisitUInt32Value(_ *oldInter.Interpreter, value oldInter.UInt32Value) {
	c.result = newInter.UInt32Value(value)
}

func (c *ValueConverter) VisitUInt64Value(_ *oldInter.Interpreter, value oldInter.UInt64Value) {
	c.result = newInter.UInt64Value(value)
}

func (c *ValueConverter) VisitUInt128Value(_ *oldInter.Interpreter, value oldInter.UInt128Value) {
	c.result = newInter.NewUInt128ValueFromBigInt(value.BigInt)
}

func (c *ValueConverter) VisitUInt256Value(_ *oldInter.Interpreter, value oldInter.UInt256Value) {
	c.result = newInter.NewUInt256ValueFromBigInt(value.BigInt)
}

func (c *ValueConverter) VisitWord8Value(_ *oldInter.Interpreter, value oldInter.Word8Value) {
	c.result = newInter.Word8Value(value)
}

func (c *ValueConverter) VisitWord16Value(_ *oldInter.Interpreter, value oldInter.Word16Value) {
	c.result = newInter.Word16Value(value)
}

func (c *ValueConverter) VisitWord32Value(_ *oldInter.Interpreter, value oldInter.Word32Value) {
	c.result = newInter.Word32Value(value)
}

func (c *ValueConverter) VisitWord64Value(_ *oldInter.Interpreter, value oldInter.Word64Value) {
	c.result = newInter.Word64Value(value)
}

func (c *ValueConverter) VisitFix64Value(_ *oldInter.Interpreter, value oldInter.Fix64Value) {
	c.result = newInter.NewFix64ValueWithInteger(int64(value.ToInt()))
}

func (c *ValueConverter) VisitUFix64Value(_ *oldInter.Interpreter, value oldInter.UFix64Value) {
	c.result = newInter.NewUFix64ValueWithInteger(uint64(value.ToInt()))
}

func (c *ValueConverter) VisitCompositeValue(_ *oldInter.Interpreter, value *oldInter.CompositeValue) bool {
	fields := newInter.NewStringValueOrderedMap()

	value.Fields().Foreach(func(key string, fieldVal oldInter.Value) {
		fields.Set(key, c.Convert(fieldVal))
	})

	c.result = newInter.NewCompositeValue(
		c.storage,
		value.Location(),
		value.QualifiedIdentifier(),
		value.Kind(),
		fields,
		*value.Owner,
	)

	// Do not descent
	return false
}

func (c *ValueConverter) VisitDictionaryValue(inter *oldInter.Interpreter, value *oldInter.DictionaryValue) bool {
	staticType := ConvertStaticType(value.StaticType()).(newInter.DictionaryStaticType)

	keysAndValues := make([]newInter.Value, 0)

	for _, key := range value.Keys().Elements() {
		entryValue, err := getValue(inter, value, key)
		if err != nil {
			continue
		}

		keysAndValues = append(keysAndValues, c.Convert(key))
		keysAndValues = append(keysAndValues, c.Convert(entryValue))
	}

	c.result = newInter.NewDictionaryValueWithAddress(
		c.newInter,
		staticType,
		*value.Owner,
		keysAndValues...,
	)

	// Do not descent
	return false
}

func getValue(
	inter *oldInter.Interpreter,
	dictionary *oldInter.DictionaryValue,
	key oldInter.Value,
) (value oldInter.Value, err error) {
	defer func() {
		if r := recover(); r != nil {
			valueNotFoundErr, ok := r.(*ValueNotFoundError)
			if !ok {
				panic(r)
			}

			err = valueNotFoundErr
		}
	}()

	value = dictionary.Get(inter, nil, key)

	if someValue, ok := value.(*oldInter.SomeValue); ok {
		value = someValue.Value
	}

	return
}

func (c *ValueConverter) VisitNilValue(_ *oldInter.Interpreter, _ oldInter.NilValue) {
	c.result = newInter.NilValue{}
}

func (c *ValueConverter) VisitSomeValue(_ *oldInter.Interpreter, value *oldInter.SomeValue) bool {
	innerValue := c.Convert(value.Value)
	c.result = newInter.NewSomeValueNonCopying(innerValue)

	// Do not descent
	return false
}

func (c *ValueConverter) VisitStorageReferenceValue(_ *oldInter.Interpreter, _ *oldInter.StorageReferenceValue) {
	panic("value not storable")
}

func (c *ValueConverter) VisitEphemeralReferenceValue(_ *oldInter.Interpreter, _ *oldInter.EphemeralReferenceValue) {
	panic("value not storable")
}

func (c *ValueConverter) VisitAddressValue(_ *oldInter.Interpreter, value oldInter.AddressValue) {
	c.result = newInter.AddressValue(value)
}

func (c *ValueConverter) VisitPathValue(_ *oldInter.Interpreter, value oldInter.PathValue) {
	c.result = newInter.PathValue{
		Domain:     value.Domain,
		Identifier: value.Identifier,
	}
}

func (c *ValueConverter) VisitCapabilityValue(_ *oldInter.Interpreter, value oldInter.CapabilityValue) {
	address := c.Convert(value.Address).(newInter.AddressValue)
	pathValue := c.Convert(value.Path).(newInter.PathValue)

	var burrowType newInter.StaticType
	if value.BorrowType != nil {
		burrowType = ConvertStaticType(value.BorrowType)
	}

	c.result = &newInter.CapabilityValue{
		Address:    address,
		Path:       pathValue,
		BorrowType: burrowType,
	}
}

func (c *ValueConverter) VisitLinkValue(_ *oldInter.Interpreter, value oldInter.LinkValue) {
	targetPath := c.Convert(value.TargetPath).(newInter.PathValue)
	c.result = newInter.LinkValue{
		TargetPath: targetPath,
		Type:       ConvertStaticType(value.Type),
	}
}

func (c *ValueConverter) VisitInterpretedFunctionValue(_ *oldInter.Interpreter, _ *oldInter.InterpretedFunctionValue) {
	panic("value not storable")
}

func (c *ValueConverter) VisitHostFunctionValue(_ *oldInter.Interpreter, _ *oldInter.HostFunctionValue) {
	panic("value not storable")
}

func (c *ValueConverter) VisitBoundFunctionValue(_ *oldInter.Interpreter, _ oldInter.BoundFunctionValue) {
	panic("value not storable")
}

func (c *ValueConverter) VisitDeployedContractValue(_ *oldInter.Interpreter, _ oldInter.DeployedContractValue) {
	panic("value not storable")
}

// Type conversions

func ConvertStaticType(staticType oldInter.StaticType) newInter.StaticType {
	switch typ := staticType.(type) {
	case oldInter.CompositeStaticType:
		return newInter.CompositeStaticType{
			Location:            typ.Location,
			QualifiedIdentifier: typ.QualifiedIdentifier,
		}
	case oldInter.InterfaceStaticType:
		return newInter.InterfaceStaticType{
			Location:            typ.Location,
			QualifiedIdentifier: typ.QualifiedIdentifier,
		}
	case oldInter.VariableSizedStaticType:
		return newInter.VariableSizedStaticType{
			Type: ConvertStaticType(typ.Type),
		}
	case oldInter.ConstantSizedStaticType:
		return newInter.ConstantSizedStaticType{
			Type: ConvertStaticType(typ.Type),
			Size: typ.Size,
		}
	case oldInter.DictionaryStaticType:
		return newInter.DictionaryStaticType{
			KeyType:   ConvertStaticType(typ.KeyType),
			ValueType: ConvertStaticType(typ.ValueType),
		}
	case oldInter.OptionalStaticType:
		return newInter.OptionalStaticType{
			Type: ConvertStaticType(typ.Type),
		}
	case *oldInter.RestrictedStaticType:
		restrictions := make([]newInter.InterfaceStaticType, 0, len(typ.Restrictions))
		for _, oldInterfaceType := range typ.Restrictions {
			newInterfaceType := ConvertStaticType(oldInterfaceType).(newInter.InterfaceStaticType)
			restrictions = append(restrictions, newInterfaceType)
		}

		return &newInter.RestrictedStaticType{
			Type:         ConvertStaticType(typ.Type),
			Restrictions: restrictions,
		}
	case oldInter.ReferenceStaticType:
		return newInter.ReferenceStaticType{
			Authorized: typ.Authorized,
			Type:       ConvertStaticType(typ.Type),
		}
	case oldInter.CapabilityStaticType:
		var burrowType newInter.StaticType

		if typ.BorrowType != nil {
			burrowType = ConvertStaticType(typ.BorrowType)
		}

		return newInter.CapabilityStaticType{
			BorrowType: burrowType,
		}
	case oldInter.PrimitiveStaticType:
		return ConvertPrimitiveStaticType(typ)
	default:
		panic(fmt.Errorf("cannot covert static type: %s", staticType))
	}
}

func ConvertPrimitiveStaticType(staticType oldInter.PrimitiveStaticType) newInter.PrimitiveStaticType {
	switch staticType {
	case oldInter.PrimitiveStaticTypeVoid:
		return newInter.PrimitiveStaticTypeVoid

	case oldInter.PrimitiveStaticTypeAny:
		return newInter.PrimitiveStaticTypeAny

	case oldInter.PrimitiveStaticTypeNever:
		return newInter.PrimitiveStaticTypeNever

	case oldInter.PrimitiveStaticTypeAnyStruct:
		return newInter.PrimitiveStaticTypeAnyStruct

	case oldInter.PrimitiveStaticTypeAnyResource:
		return newInter.PrimitiveStaticTypeAnyResource

	case oldInter.PrimitiveStaticTypeBool:
		return newInter.PrimitiveStaticTypeBool

	case oldInter.PrimitiveStaticTypeAddress:
		return newInter.PrimitiveStaticTypeAddress

	case oldInter.PrimitiveStaticTypeString:
		return newInter.PrimitiveStaticTypeString

	case oldInter.PrimitiveStaticTypeCharacter:
		return newInter.PrimitiveStaticTypeCharacter

	case oldInter.PrimitiveStaticTypeMetaType:
		return newInter.PrimitiveStaticTypeMetaType

	case oldInter.PrimitiveStaticTypeBlock:
		return newInter.PrimitiveStaticTypeBlock

	// Number

	case oldInter.PrimitiveStaticTypeNumber:
		return newInter.PrimitiveStaticTypeNumber
	case oldInter.PrimitiveStaticTypeSignedNumber:
		return newInter.PrimitiveStaticTypeSignedNumber

	// Integer
	case oldInter.PrimitiveStaticTypeInteger:
		return newInter.PrimitiveStaticTypeInteger
	case oldInter.PrimitiveStaticTypeSignedInteger:
		return newInter.PrimitiveStaticTypeSignedInteger

	// FixedPoint
	case oldInter.PrimitiveStaticTypeFixedPoint:
		return newInter.PrimitiveStaticTypeFixedPoint
	case oldInter.PrimitiveStaticTypeSignedFixedPoint:
		return newInter.PrimitiveStaticTypeSignedFixedPoint

	// Int*
	case oldInter.PrimitiveStaticTypeInt:
		return newInter.PrimitiveStaticTypeInt
	case oldInter.PrimitiveStaticTypeInt8:
		return newInter.PrimitiveStaticTypeInt8
	case oldInter.PrimitiveStaticTypeInt16:
		return newInter.PrimitiveStaticTypeInt16
	case oldInter.PrimitiveStaticTypeInt32:
		return newInter.PrimitiveStaticTypeInt32
	case oldInter.PrimitiveStaticTypeInt64:
		return newInter.PrimitiveStaticTypeInt64
	case oldInter.PrimitiveStaticTypeInt128:
		return newInter.PrimitiveStaticTypeInt128
	case oldInter.PrimitiveStaticTypeInt256:
		return newInter.PrimitiveStaticTypeInt256

	// UInt*
	case oldInter.PrimitiveStaticTypeUInt:
		return newInter.PrimitiveStaticTypeUInt
	case oldInter.PrimitiveStaticTypeUInt8:
		return newInter.PrimitiveStaticTypeUInt8
	case oldInter.PrimitiveStaticTypeUInt16:
		return newInter.PrimitiveStaticTypeUInt16
	case oldInter.PrimitiveStaticTypeUInt32:
		return newInter.PrimitiveStaticTypeUInt32
	case oldInter.PrimitiveStaticTypeUInt64:
		return newInter.PrimitiveStaticTypeUInt64
	case oldInter.PrimitiveStaticTypeUInt128:
		return newInter.PrimitiveStaticTypeUInt128
	case oldInter.PrimitiveStaticTypeUInt256:
		return newInter.PrimitiveStaticTypeUInt256

	// Word *

	case oldInter.PrimitiveStaticTypeWord8:
		return newInter.PrimitiveStaticTypeWord8
	case oldInter.PrimitiveStaticTypeWord16:
		return newInter.PrimitiveStaticTypeWord16
	case oldInter.PrimitiveStaticTypeWord32:
		return newInter.PrimitiveStaticTypeWord32
	case oldInter.PrimitiveStaticTypeWord64:
		return newInter.PrimitiveStaticTypeWord64

	// Fix*
	case oldInter.PrimitiveStaticTypeFix64:
		return newInter.PrimitiveStaticTypeFix64

	// UFix*
	case oldInter.PrimitiveStaticTypeUFix64:
		return newInter.PrimitiveStaticTypeUFix64

	// Storage

	case oldInter.PrimitiveStaticTypePath:
		return newInter.PrimitiveStaticTypePath
	case oldInter.PrimitiveStaticTypeStoragePath:
		return newInter.PrimitiveStaticTypeStoragePath
	case oldInter.PrimitiveStaticTypeCapabilityPath:
		return newInter.PrimitiveStaticTypeCapabilityPath
	case oldInter.PrimitiveStaticTypePublicPath:
		return newInter.PrimitiveStaticTypePublicPath
	case oldInter.PrimitiveStaticTypePrivatePath:
		return newInter.PrimitiveStaticTypePrivatePath
	case oldInter.PrimitiveStaticTypeCapability:
		return newInter.PrimitiveStaticTypeCapability
	case oldInter.PrimitiveStaticTypeAuthAccount:
		return newInter.PrimitiveStaticTypeAuthAccount
	case oldInter.PrimitiveStaticTypePublicAccount:
		return newInter.PrimitiveStaticTypePublicAccount
	case oldInter.PrimitiveStaticTypeDeployedContract:
		return newInter.PrimitiveStaticTypeDeployedContract
	case oldInter.PrimitiveStaticTypeAuthAccountContracts:
		return newInter.PrimitiveStaticTypeAuthAccountContracts
	default:
		panic(fmt.Errorf("cannot covert static type: %s", staticType.String()))
	}
}

type ValueNotFoundError struct {
	key string
}

func (e *ValueNotFoundError) Error() string {
	return fmt.Sprintf("value not found for key: %s", e.key)
}
