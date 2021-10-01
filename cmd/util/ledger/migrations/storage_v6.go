package migrations

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math"
	"math/bits"
	"os"
	"path"
	"strings"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/onflow/atree"
	"github.com/rs/zerolog"
	"github.com/schollz/progressbar/v3"

	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	newInter "github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/stdlib"

	oldInter "github.com/onflow/cadence/v19/runtime/interpreter"
)

// cborTagStorageReference is a duplicate of the same from cadence v0.19.0
const cborTagStorageReference = 202

// \x1F = Information Separator One
//
const pathSeparator = "\x1F"

var storageReferenceEncodingStart = []byte{0xd8, cborTagStorageReference}

// maxInt is math.MaxInt32 or math.MaxInt64 depending on arch.
const maxInt = 1<<(bits.UintSize-1) - 1

// storageMigrationV5DecMode is a duplicate of decMode from cadence v0.19.0
var storageMigrationV5DecMode = func() cbor.DecMode {
	decMode, err := cbor.DecOptions{
		IntDec:           cbor.IntDecConvertNone,
		MaxArrayElements: maxInt,
		MaxMapPairs:      maxInt,
		MaxNestedLevels:  math.MaxInt16,
	}.DecMode()
	if err != nil {
		panic(err)
	}
	return decMode
}()

type storagePath struct {
	owner string
	key   string
}

type StorageFormatV6Migration struct {
	Log        zerolog.Logger
	OutputDir  string
	accounts   state.Accounts
	programs   *programs.Programs
	reportFile *os.File
	storage    *runtime.Storage
	oldInter   *oldInter.Interpreter
	newInter   *newInter.Interpreter
	converter  *ValueConverter

	migratedPayloadPaths map[storagePath]bool
	deferredValuePaths   map[storagePath]bool
	progress             *progressbar.ProgressBar
	emptyDeferredValues  int
	skippedValues        int
}

func (m *StorageFormatV6Migration) filename() string {
	return path.Join(m.OutputDir, fmt.Sprintf("migration_report_%d.txt", int32(time.Now().Unix())))
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

	total := int64(len(payloads) * 3)
	m.progress = progressbar.Default(total, "Migrating:")

	return m.migrate(payloads)
}

func (m *StorageFormatV6Migration) migrate(payloads []ledger.Payload) ([]ledger.Payload, error) {
	m.Log.Info().Msg("Updating broken contracts ...")
	m.updateBrokenContracts(payloads)
	m.Log.Info().Msg("Broken contracts updated")

	m.Log.Info().Msg("Loading account contracts ...")
	m.accounts = m.getAccounts(payloads)
	m.Log.Info().Msg("Loaded account contracts")

	m.programs = programs.NewEmptyPrograms()

	m.migratedPayloadPaths = make(map[storagePath]bool, 0)

	fvmPayloads, storagePayloads, slabPayloads := splitPayloads(payloads)
	if len(slabPayloads) != 0 {
		return nil, fmt.Errorf(
			"slab storages are not empty: found %d",
			len(slabPayloads),
		)
	}

	ledgerView := newView(fvmPayloads)
	m.initPersistentSlabStorage(ledgerView)

	m.initNewInterpreter()
	m.initOldInterpreter(payloads)

	m.deferredValuePaths = m.getDeferredKeys(payloads)

	m.converter = NewValueConverter(m)

	// Convert payloads.
	//   - Cadence values are decoded and converted to new values.
	//   - Non-cadence values are added to the migrated payloads.

	m.Log.Info().Msg("Converting payloads...")

	for _, payload := range storagePayloads {
		keyParts := payload.Key.KeyParts
		rawOwner := keyParts[0].Value
		rawKey := keyParts[2].Value

		err := m.reencodePayload(payload)

		if err != nil {
			return nil, fmt.Errorf(
				"failed to migrate key: %q (owner: %x): %w",
				rawKey,
				rawOwner,
				err,
			)
		}

		m.incrementProgress()
	}
	m.clearProgress()
	m.Log.Info().Msg("Converting payloads complete")

	// Encode the new values by calling `storage.Commit()`

	m.Log.Info().Msg("Re-encoding converted values...")
	m.incrementProgress()

	err := m.storage.Commit()
	if err != nil {
		return nil, fmt.Errorf("failed to migrate payloads: %w", err)
	}

	m.completeProgress()

	m.Log.Info().Msg("Re-encoding converted values complete")

	if m.emptyDeferredValues > 0 {
		m.clearProgress()
		m.Log.Warn().Msgf("empty deferred values found: %d", m.emptyDeferredValues)
	}

	if m.skippedValues > 0 {
		m.clearProgress()
		m.Log.Warn().Msgf("values not migrated: %d", m.skippedValues)
	}

	return ledgerView.Payloads(), nil
}

func (m *StorageFormatV6Migration) incrementProgress() {
	if m.progress == nil {
		return
	}

	err := m.progress.Add(1)
	if err != nil {
		panic(err)
	}
}

func (m *StorageFormatV6Migration) clearProgress() {
	if m.progress == nil {
		return
	}

	err := m.progress.Clear()
	if err != nil {
		panic(err)
	}
}

func (m *StorageFormatV6Migration) completeProgress() {
	if m.progress == nil {
		return
	}

	if m.progress.IsFinished() {
		return
	}

	err := m.progress.Finish()
	if err != nil {
		panic(err)
	}
}

func (m *StorageFormatV6Migration) addProgress(progress int) {
	if m.progress == nil {
		return
	}

	err := m.progress.Add(progress)
	if err != nil {
		panic(err)
	}
}

func (m *StorageFormatV6Migration) initPersistentSlabStorage(v *view) {
	st := state.NewState(
		v,
		state.WithMaxInteractionSizeAllowed(math.MaxUint64),
	)
	stateHolder := state.NewStateHolder(st)
	accounts := state.NewAccounts(stateHolder)

	m.storage = runtime.NewStorage(
		newAccountsAtreeLedger(accounts),
		func(f func(), _ func(metrics runtime.Metrics, duration time.Duration)) {
			f()
		},
	)
}

func (m *StorageFormatV6Migration) getAccounts(payloads []ledger.Payload) state.Accounts {
	l := newView(payloads)
	st := state.NewState(l)
	sth := state.NewStateHolder(st)
	accounts := state.NewAccounts(sth)
	return accounts
}

func (m *StorageFormatV6Migration) getDeferredKeys(payloads []ledger.Payload) map[storagePath]bool {
	m.clearProgress()
	m.Log.Info().Msgf("Collecting deferred keys...")

	deferredValuePaths := make(map[storagePath]bool, 0)
	for _, payload := range payloads {
		m.incrementProgress()

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

		err := storageMigrationV5DecMode.Valid(value)
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
				dictionary, ok := inspectedValue.(*oldInter.DictionaryValue)
				if !ok {
					return true
				}

				deferredKeys := dictionary.DeferredKeys()
				if deferredKeys == nil {
					return true
				}

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

				return true
			},
		)
	}

	m.clearProgress()
	m.Log.Info().Msgf("Deferred keys collected: %d", len(deferredValuePaths))

	return deferredValuePaths
}

func (m *StorageFormatV6Migration) reencodePayload(payload ledger.Payload) error {
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
		return fmt.Errorf(
			"invalid payload for conversion: %s",
			payload.Key.String(),
		)
	}

	value, version := oldInter.StripMagic(payload.Value)

	if version != oldInter.CurrentEncodingVersion {
		return fmt.Errorf(
			"invalid storage format version for key: %s: %d",
			rawKey,
			version,
		)
	}

	err := storageMigrationV5DecMode.Valid(value)
	if err != nil {
		return err
	}

	err = m.decodeAndConvert(
		value,
		common.BytesToAddress(rawOwner),
		string(rawKey),
		version,
	)

	if err != nil {
		return fmt.Errorf(
			"failed to decode and convert key: %s: %w\n\nvalue:\n%s\n\n%s",
			rawKey, err,
			hex.Dump(value),
			cborMeLink(value),
		)
	}

	return nil
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

	result := m.converter.Convert(rootValue, nil)
	m.storage.WriteValue(nil, owner, key, newInter.NewSomeValueNonCopying(result))

	// Mark the payload as 'migrated'.
	m.migratedPayloadPaths[path] = true

	return nil
}

func (m *StorageFormatV6Migration) initNewInterpreter() {
	inter, err := newInter.NewInterpreter(
		nil,
		nil,
		newInter.WithStorage(m.storage),
		newInter.WithImportLocationHandler(
			func(inter *newInter.Interpreter, location common.Location) newInter.Import {
				var program *newInter.Program
				if location == stdlib.CryptoChecker.Location {
					program = newInter.ProgramFromChecker(stdlib.CryptoChecker)
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

				return newInter.InterpreterImport{
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

	m.newInter = inter
}

func (m *StorageFormatV6Migration) initOldInterpreter(payloads []ledger.Payload) {
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
					m.Log.Debug().Msgf("empty deferred value: owner: %s, key: %s",
						ownerStr,
						key,
					)

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

				err = storageMigrationV5DecMode.Valid(content)
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

func (m *StorageFormatV6Migration) updateBrokenContracts(payloads []ledger.Payload) {
	for index, payload := range payloads {
		rawOwner := payload.Key.KeyParts[0].Value
		key := string(payload.Key.KeyParts[2].Value)

		if !strings.HasPrefix(key, "code.") {
			continue
		}

		ownerHex := hex.EncodeToString(rawOwner)
		switch {
		case ownerHex == "ac98da57ce4dd4ef" && strings.HasSuffix(key, "MessageBoard"):
			payloads[index].Value = []byte(knownContract_ac98da57ce4dd4ef_MessageBoard)
			m.Log.Info().Msg("contract updated: ac98da57ce4dd4ef.MessageBoard")
		case ownerHex == "dee35303492e5a0b" && strings.HasSuffix(key, "FlowIDTableStaking"):
			payloads[index].Value = []byte(knownContract_dee35303492e5a0b_FlowIDTableStaking)
			m.Log.Info().Msg("contract updated: dee35303492e5a0b.FlowIDTableStaking")
		case ownerHex == "1864ff317a35af46" && strings.HasSuffix(key, "FlowIDTableStaking"):
			payloads[index].Value = []byte(knownContract_1864ff317a35af46_FlowIDTableStaking)
			m.Log.Info().Msg("contract updated: 1864ff317a35af46.FlowIDTableStaking")
		case ownerHex == "ab273f724a1625df" && strings.HasSuffix(key, "MultiMessageBoard"):
			payloads[index].Value = []byte(knownContract_ab273f724a1625df_MultiMessageBoard)
			m.Log.Info().Msg("contract updated: ab273f724a1625df.MultiMessageBoard")
		}
	}
}

func (m StorageFormatV6Migration) getLocation(location common.Location) common.Location {
	addressLocation, ok := location.(common.AddressLocation)
	if !ok {
		return location
	}

	// If any of the broken/missing types are found, update
	// the referer to use the new version of the same type.
	addressHex := addressLocation.Address.Hex()
	switch addressLocation.Name {
	case "FlowIDTableStaking":
		switch addressHex {
		case "e94f751ba094ef6a",
			"ecda6c5746d5bdf0",
			"f1a43bfd1354c9b8",
			"16a5fe3b527633d4",
			"76d9ea44cef09e20",
			"9798362e92e5539a":
			address, err := hex.DecodeString("9eca2b38b18b5dfe")
			if err != nil {
				panic(err)
			}

			location = common.AddressLocation{
				Address: common.BytesToAddress(address),
				Name:    addressLocation.Name,
			}
		}
	case "KittyItemsMarket", "KittyItems":
		switch addressHex {
		case "fcceff21d9532b58",
			"17341c7824b030be",
			"f79ee844bfa76528":
			address, err := hex.DecodeString("8c5244250369a9ce")
			if err != nil {
				panic(err)
			}

			location = common.AddressLocation{
				Address: common.BytesToAddress(address),
				Name:    addressLocation.Name,
			}
		}
	case "DisruptNowBeta":
		if addressHex == "45888dabccc5c376" {
			address, err := hex.DecodeString("849832a65c0524b5")
			if err != nil {
				panic(err)
			}

			location = common.AddressLocation{
				Address: common.BytesToAddress(address),
				Name:    addressLocation.Name,
			}
		}
	case "NonFungibleToken":
		if addressHex == "cd2fde7d198629e4" {
			address, err := hex.DecodeString("c2f5b3fb0ad43ff1")
			if err != nil {
				panic(err)
			}

			location = common.AddressLocation{
				Address: common.BytesToAddress(address),
				Name:    addressLocation.Name,
			}
		}
	case "NonFungibleBeatoken":
		if addressHex == "70239ed8e4c7367a" {
			address, err := hex.DecodeString("14f7f8198e156fb0")
			if err != nil {
				panic(err)
			}

			location = common.AddressLocation{
				Address: common.BytesToAddress(address),
				Name:    addressLocation.Name,
			}
		}
	case "Auction", "Versus":
		if addressHex == "1ff7e32d71183db0" {
			// Use the existing one if any. Update otherwise.
			code, err := m.accounts.GetContract(addressLocation.Name, flow.Address(addressLocation.Address))
			if err == nil && len(code) != 0 {
				break
			}

			location = common.AddressLocation{
				Address: testnetAuctionContractAddress,
				Name:    addressLocation.Name,
			}
		}
	case "OpenEdition":
		if addressHex == "85080f371da20cc1" {
			location = common.AddressLocation{
				Address: addressLocation.Address,
				Name:    "OpenEditionV2",
			}
		}

	// mainnet
	case "LockedTokens":
		if addressHex == "31aed847945124fd" {
			address, err := hex.DecodeString("8d0e87b65159ae63")
			if err != nil {
				panic(err)
			}

			location = common.AddressLocation{
				Address: common.BytesToAddress(address),
				Name:    addressLocation.Name,
			}
		}
	}

	return location
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
	result    newInter.Value
	newInter  *newInter.Interpreter
	oldInter  *oldInter.Interpreter
	migration *StorageFormatV6Migration
}

var _ oldInter.Visitor = &ValueConverter{}

func NewValueConverter(
	migration *StorageFormatV6Migration,
) *ValueConverter {
	return &ValueConverter{
		migration: migration,
		newInter:  migration.newInter,
		oldInter:  migration.oldInter,
	}
}

func (c *ValueConverter) Convert(value oldInter.Value, expectedType newInter.StaticType) (result newInter.Value) {
	prevResult := c.result
	c.result = nil

	defer func() {
		c.result = prevResult

		r := recover()
		if r == nil {
			return
		}

		var writeErr error
		switch err := r.(type) {
		case newInter.TypeLoadingError:
			_, writeErr = c.migration.reportFile.WriteString(
				fmt.Sprintf(
					"skipped migrating value: missing static type: %s, owner: %s\n",
					err.TypeID,
					value.GetOwner(),
				),
			)
		case newInter.ContainerMutationError:
			_, writeErr = c.migration.reportFile.WriteString(
				fmt.Sprintf(
					"skipped migrating value: %s, owner: %s\n",
					err.Error(),
					value.GetOwner(),
				),
			)
		case runtime.Error:
			if parsingCheckingErr, ok := err.Unwrap().(*runtime.ParsingCheckingError); ok {
				_, writeErr = c.migration.reportFile.WriteString(
					fmt.Sprintf(
						"skipped migrating value: broken contract type: %s, cause: %s\n",
						parsingCheckingErr.Location,
						parsingCheckingErr.Error(),
					),
				)
			} else {
				_, writeErr = c.migration.reportFile.WriteString(
					fmt.Sprintf(
						"skipped migrating value: cause: %s\n",
						err.Error(),
					),
				)
			}
		case newInter.Error:
			_, writeErr = c.migration.reportFile.WriteString(
				fmt.Sprintf(
					"skipped migrating value: cause: %s\n",
					err.Error(),
				),
			)
		default:
			panic(err)
		}

		if writeErr != nil {
			panic(writeErr)
		}

		c.migration.skippedValues += 1
		result = nil
	}()

	value.Accept(c.oldInter, c)

	if c.result == nil {
		panic("converted value is nil")
	}

	switch expectedType {
	case newInter.PrimitiveStaticTypeUInt64:
		if intValue, ok := c.result.(newInter.IntValue); ok {
			c.result = newInter.ConvertUInt64(intValue)
		}
	}
	return c.result
}

func (c *ValueConverter) VisitValue(_ *oldInter.Interpreter, _ oldInter.Value) {
	panic("unreachable")
}

func (c *ValueConverter) VisitTypeValue(_ *oldInter.Interpreter, value oldInter.TypeValue) {
	c.result = newInter.TypeValue{
		Type: c.convertStaticType(value.Type),
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
	newElements := make([]newInter.Value, 0)

	arrayStaticType := c.convertStaticType(value.StaticType()).(newInter.ArrayStaticType)
	elementType := arrayStaticType.ElementType()

	for _, element := range value.Elements() {
		newElement := c.Convert(element, elementType)
		if newElement != nil {
			newElements = append(newElements, newElement)
		}
	}

	c.result = newInter.NewArrayValue(
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
	c.result = newInter.Fix64Value(value)
}

func (c *ValueConverter) VisitUFix64Value(_ *oldInter.Interpreter, value oldInter.UFix64Value) {
	c.result = newInter.UFix64Value(value)
}

func (c *ValueConverter) VisitCompositeValue(_ *oldInter.Interpreter, value *oldInter.CompositeValue) bool {
	fields := make([]newInter.CompositeField, 0)

	value.Fields().Foreach(func(key string, fieldVal oldInter.Value) {
		newValue := c.Convert(fieldVal, nil)
		if newValue != nil {
			fields = append(
				fields,
				newInter.CompositeField{
					Name:  key,
					Value: newValue,
				},
			)
		}
	})

	location := c.migration.getLocation(value.Location())
	identifier := getIdentifier(value.QualifiedIdentifier(), location)

	c.result = newInter.NewCompositeValue(
		c.newInter,
		location,
		identifier,
		value.Kind(),
		fields,
		*value.Owner,
	)

	// Do not descent
	return false
}

func (c *ValueConverter) VisitDictionaryValue(inter *oldInter.Interpreter, value *oldInter.DictionaryValue) bool {
	staticType := c.convertStaticType(value.StaticType()).(newInter.DictionaryStaticType)

	keysAndValues := make([]newInter.Value, 0)

	for _, key := range value.Keys().Elements() {
		entryValue, err := getValue(inter, value, key)
		if err != nil {
			continue
		}

		newValue := c.Convert(entryValue, staticType.ValueType)
		if newValue != nil {
			keysAndValues = append(keysAndValues, c.Convert(key, staticType.KeyType))
			keysAndValues = append(keysAndValues, newValue)
		}
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

	value = dictionary.Get(inter, oldInter.ReturnEmptyLocationRange, key)

	if someValue, ok := value.(*oldInter.SomeValue); ok {
		value = someValue.Value
	}

	return
}

func (c *ValueConverter) VisitNilValue(_ *oldInter.Interpreter, _ oldInter.NilValue) {
	c.result = newInter.NilValue{}
}

func (c *ValueConverter) VisitSomeValue(_ *oldInter.Interpreter, value *oldInter.SomeValue) bool {
	innerValue := c.Convert(value.Value, nil)
	if innerValue == nil {
		panic("value cannot be nil")
	}

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
	address := c.Convert(value.Address, nil).(newInter.AddressValue)
	pathValue := c.Convert(value.Path, nil).(newInter.PathValue)

	var borrowType newInter.StaticType
	if value.BorrowType != nil {
		borrowType = c.convertStaticType(value.BorrowType)
	}

	c.result = &newInter.CapabilityValue{
		Address:    address,
		Path:       pathValue,
		BorrowType: borrowType,
	}
}

func (c *ValueConverter) VisitLinkValue(_ *oldInter.Interpreter, value oldInter.LinkValue) {
	targetPath := c.Convert(value.TargetPath, nil).(newInter.PathValue)
	c.result = newInter.LinkValue{
		TargetPath: targetPath,
		Type:       c.convertStaticType(value.Type),
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

var testnetNonFungibleTokenContractAddress = func() common.Address {
	address, err := hex.DecodeString("631e88ae7f1d7c20")
	if err != nil {
		panic(err)
	}

	return common.BytesToAddress(address)
}()

var testnetAuctionContractAddress = func() common.Address {
	address, err := hex.DecodeString("99ca04281098b33d")
	if err != nil {
		panic(err)
	}

	return common.BytesToAddress(address)
}()

func (c *ValueConverter) convertStaticType(staticType oldInter.StaticType) newInter.StaticType {
	switch typ := staticType.(type) {
	case oldInter.CompositeStaticType:
		location := c.migration.getLocation(typ.Location)
		identifier := getIdentifier(typ.QualifiedIdentifier, location)
		return newInter.NewCompositeStaticType(location, identifier)

	case oldInter.InterfaceStaticType:
		location := c.migration.getLocation(typ.Location)

		// Following types are struct/resource, but is stored as an interface type.
		// Rectify this by returning a composite static type.
		if location, ok := location.(common.AddressLocation); ok {
			if location.Address == testnetNonFungibleTokenContractAddress &&
				typ.QualifiedIdentifier == "NonFungibleToken.NFT" {
				return newInter.NewCompositeStaticType(location, typ.QualifiedIdentifier)
			} else if location.Address == testnetAuctionContractAddress &&
				typ.QualifiedIdentifier == "Auction.AuctionItem" {
				return newInter.NewCompositeStaticType(location, typ.QualifiedIdentifier)
			}
		}

		return newInter.InterfaceStaticType{
			Location:            location,
			QualifiedIdentifier: typ.QualifiedIdentifier,
		}

	case oldInter.VariableSizedStaticType:
		return newInter.VariableSizedStaticType{
			Type: c.convertStaticType(typ.Type),
		}

	case oldInter.ConstantSizedStaticType:
		return newInter.ConstantSizedStaticType{
			Type: c.convertStaticType(typ.Type),
			Size: typ.Size,
		}

	case oldInter.DictionaryStaticType:
		return newInter.DictionaryStaticType{
			KeyType:   c.convertStaticType(typ.KeyType),
			ValueType: c.convertStaticType(typ.ValueType),
		}

	case oldInter.OptionalStaticType:
		return newInter.OptionalStaticType{
			Type: c.convertStaticType(typ.Type),
		}

	case *oldInter.RestrictedStaticType:
		restrictions := make([]newInter.InterfaceStaticType, 0, len(typ.Restrictions))
		for _, oldInterfaceType := range typ.Restrictions {
			newInterfaceType := c.convertStaticType(oldInterfaceType).(newInter.InterfaceStaticType)
			restrictions = append(restrictions, newInterfaceType)
		}

		return &newInter.RestrictedStaticType{
			Type:         c.convertStaticType(typ.Type),
			Restrictions: restrictions,
		}

	case oldInter.ReferenceStaticType:
		return newInter.ReferenceStaticType{
			Authorized: typ.Authorized,
			Type:       c.convertStaticType(typ.Type),
		}

	case oldInter.CapabilityStaticType:
		var borrowType newInter.StaticType

		if typ.BorrowType != nil {
			borrowType = c.convertStaticType(typ.BorrowType)
		}

		return newInter.CapabilityStaticType{
			BorrowType: borrowType,
		}

	case oldInter.PrimitiveStaticType:
		return ConvertPrimitiveStaticType(typ)

	default:
		panic(fmt.Errorf("cannot covert static type: %s", staticType))
	}
}

func getIdentifier(identifier string, location common.Location) string {
	addressLocation, ok := location.(common.AddressLocation)

	if !ok {
		return identifier
	}

	if addressLocation.Name == "OpenEditionV2" &&
		addressLocation.Address.Hex() == "85080f371da20cc1" &&
		identifier == "OpenEdition.OpenEditionItem" {
		identifier = "OpenEditionV2.OpenEditionItem"
	}

	return identifier
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

	// Word*

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

//nolint:gosimple
const knownContract_1864ff317a35af46_FlowIDTableStaking = `
/*

    FlowIDTableStaking

    The Flow ID Table and Staking contract manages
    node operators' and delegators' information
    and flow tokens that are staked as part of the Flow Protocol.

    It is recommended to check out the staking page on the Flow Docs site
    before looking at the smart contract. It will help with understanding
    https://docs.onflow.org/token/staking/

    Nodes submit their stake to the public addNodeInfo Function
    during the staking auction phase.

    This records their info and committd tokens. They also will get a Node
    Object that they can use to stake, unstake, and withdraw rewards.

    Each node has multiple token buckets that hold their tokens
    based on their status. committed, staked, unstaked, unlocked, and rewarded.

    The Admin has the authority to remove node records,
    refund insufficiently staked nodes, pay rewards,
    and move tokens between buckets. These will happen once every epoch.

    All the node info and staking info is publicly accessible
    to any transaction in the network

    Node Roles are represented by integers:
        1 = collection
        2 = consensus
        3 = execution
        4 = verification
        5 = access

 */

import FungibleToken from 0xf233dcee88fe0abe
import FlowToken from 0x1654653399040a61

pub contract FlowIDTableStaking {

    /********************* ID Table and Staking Events **********************/
    pub event NewNodeCreated(nodeID: String, role: UInt8, amountCommitted: UFix64)
    pub event TokensCommitted(nodeID: String, amount: UFix64)
    pub event TokensStaked(nodeID: String, amount: UFix64)
    pub event TokensUnStaked(nodeID: String, amount: UFix64)
    pub event NodeRemovedAndRefunded(nodeID: String, amount: UFix64)
    pub event RewardsPaid(nodeID: String, amount: UFix64)
    pub event UnlockedTokensWithdrawn(nodeID: String, amount: UFix64)
    pub event RewardTokensWithdrawn(nodeID: String, amount: UFix64)
    pub event NewDelegatorCutPercentage(newCutPercentage: UFix64)

    /// Delegator Events
    pub event NewDelegatorCreated(nodeID: String, delegatorID: UInt32)
    pub event DelegatorRewardsPaid(nodeID: String, delegatorID: UInt32, amount: UFix64)
    pub event DelegatorUnlockedTokensWithdrawn(nodeID: String, delegatorID: UInt32, amount: UFix64)
    pub event DelegatorRewardTokensWithdrawn(nodeID: String, delegatorID: UInt32, amount: UFix64)

    /// Holds the identity table for all the nodes in the network.
    /// Includes nodes that aren't actively participating
    /// key = node ID
    /// value = the record of that node's info, tokens, and delegators
    access(contract) var nodes: @{String: NodeRecord}

    /// The minimum amount of tokens that each node type has to stake
    /// in order to be considered valid
    /// key = node role
    /// value = amount of tokens
    access(contract) var minimumStakeRequired: {UInt8: UFix64}

    /// The total amount of tokens that are staked for all the nodes
    /// of each node type during the current epoch
    /// key = node role
    /// value = amount of tokens
    access(contract) var totalTokensStakedByNodeType: {UInt8: UFix64}

    /// The total amount of tokens that are paid as rewards every epoch
    /// could be manually changed by the admin resource
    pub var epochTokenPayout: UFix64

    /// The ratio of the weekly awards that each node type gets
    /// key = node role
    /// value = decimal number between 0 and 1 indicating a percentage
    access(contract) var rewardRatios: {UInt8: UFix64}

    /// The percentage of rewards that every node operator takes from
    /// the users that are delegating to it
    pub var nodeDelegatingRewardCut: UFix64

    /// Paths for storing staking resources
    pub let NodeStakerStoragePath: Path
    pub let NodeStakerPublicPath: Path
    pub let StakingAdminStoragePath: StoragePath
    pub let DelegatorStoragePath: Path

    /*********** ID Table and Staking Composite Type Definitions *************/

    /// Contains information that is specific to a node in Flow
    /// only lives in this contract
    pub resource NodeRecord {

        /// The unique ID of the node
        /// Set when the node is created
        pub let id: String

        /// The type of node:
        /// 1 = collection
        /// 2 = consensus
        /// 3 = execution
        /// 4 = verification
        /// 5 = access
        pub var role: UInt8

        /// The address used for networking
        pub(set) var networkingAddress: String

        /// the public key for networking
        pub(set) var networkingKey: String

        /// the public key for staking
        pub(set) var stakingKey: String

        /// The total tokens that this node currently has staked, including delegators
        /// This value must always be above the minimum requirement to stay staked
        /// or accept delegators
        pub var tokensStaked: @FlowToken.Vault

        /// The tokens that this node has committed to stake for the next epoch.
        pub var tokensCommitted: @FlowToken.Vault

        /// The tokens that this node has unstaked from the previous epoch
        /// Moves to the tokensUnlocked bucket at the end of the epoch.
        pub var tokensUnstaked: @FlowToken.Vault

        /// Tokens that this node is able to withdraw whenever they want
        /// Staking rewards are paid to this bucket
        pub var tokensUnlocked: @FlowToken.Vault

        /// Staking rewards are paid to this bucket
        /// Can be withdrawn whenever
        pub var tokensRewarded: @FlowToken.Vault

        /// list of delegators for this node operator
        pub let delegators: @{UInt32: DelegatorRecord}

        /// The incrementing ID used to register new delegators
        pub(set) var delegatorIDCounter: UInt32

        /// The amount of tokens that this node has requested to unstake
        /// for the next epoch
        pub(set) var tokensRequestedToUnstake: UFix64

        /// weight as determined by the amount staked after the staking auction
        pub(set) var initialWeight: UInt64

        init(id: String,
             role: UInt8,  /// role that the node will have for future epochs
             networkingAddress: String,
             networkingKey: String,
             stakingKey: String,
             tokensCommitted: @FungibleToken.Vault
        ) {
            pre {
                id.length == 64: "Node ID length must be 32 bytes (64 hex characters)"
                FlowIDTableStaking.nodes[id] == nil: "The ID cannot already exist in the record"
                role >= UInt8(1) && role <= UInt8(5): "The role must be 1, 2, 3, 4, or 5"
                networkingAddress.length > 0: "The networkingAddress cannot be empty"
            }

            /// Assert that the addresses and keys are not already in use
            /// They must be unique
            for nodeID in FlowIDTableStaking.nodes.keys {
                assert (
                    networkingAddress != FlowIDTableStaking.nodes[nodeID]?.networkingAddress,
                    message: "Networking Address is already in use!"
                )
                assert (
                    networkingKey != FlowIDTableStaking.nodes[nodeID]?.networkingKey,
                    message: "Networking Key is already in use!"
                )
                assert (
                    stakingKey != FlowIDTableStaking.nodes[nodeID]?.stakingKey,
                    message: "Staking Key is already in use!"
                )
            }

            self.id = id
            self.role = role
            self.networkingAddress = networkingAddress
            self.networkingKey = networkingKey
            self.stakingKey = stakingKey
            self.initialWeight = 0
            self.delegators <- {}
            self.delegatorIDCounter = 0

            self.tokensCommitted <- tokensCommitted as! @FlowToken.Vault
            self.tokensStaked <- FlowToken.createEmptyVault() as! @FlowToken.Vault
            self.tokensUnstaked <- FlowToken.createEmptyVault() as! @FlowToken.Vault
            self.tokensUnlocked <- FlowToken.createEmptyVault() as! @FlowToken.Vault
            self.tokensRewarded <- FlowToken.createEmptyVault() as! @FlowToken.Vault
            self.tokensRequestedToUnstake = 0.0

            emit NewNodeCreated(nodeID: self.id, role: self.role, amountCommitted: self.tokensCommitted.balance)
        }

        destroy() {
            let flowTokenRef = FlowIDTableStaking.account.borrow<&FlowToken.Vault>(from: /storage/flowTokenVault)!
            if self.tokensStaked.balance > 0.0 {
                FlowIDTableStaking.totalTokensStakedByNodeType[self.role] = FlowIDTableStaking.totalTokensStakedByNodeType[self.role]! - self.tokensStaked.balance
                flowTokenRef.deposit(from: <-self.tokensStaked)
            } else { destroy self.tokensStaked }
            if self.tokensCommitted.balance > 0.0 {
                flowTokenRef.deposit(from: <-self.tokensCommitted)
            } else { destroy  self.tokensCommitted }
            if self.tokensUnstaked.balance > 0.0 {
                flowTokenRef.deposit(from: <-self.tokensUnstaked)
            } else { destroy  self.tokensUnstaked }
            if self.tokensUnlocked.balance > 0.0 {
                flowTokenRef.deposit(from: <-self.tokensUnlocked)
            } else { destroy  self.tokensUnlocked }
            if self.tokensRewarded.balance > 0.0 {
                flowTokenRef.deposit(from: <-self.tokensRewarded)
            } else { destroy  self.tokensRewarded }

            // Return all of the delegators' funds
            for delegator in self.delegators.keys {
                let delRecord = self.borrowDelegatorRecord(delegator)
                if delRecord.tokensCommitted.balance > 0.0 {
                    flowTokenRef.deposit(from: <-delRecord.tokensCommitted.withdraw(amount: delRecord.tokensCommitted.balance))
                }
                if delRecord.tokensStaked.balance > 0.0 {
                    flowTokenRef.deposit(from: <-delRecord.tokensStaked.withdraw(amount: delRecord.tokensStaked.balance))
                }
                if delRecord.tokensUnlocked.balance > 0.0 {
                    flowTokenRef.deposit(from: <-delRecord.tokensUnlocked.withdraw(amount: delRecord.tokensUnlocked.balance))
                }
                if delRecord.tokensRewarded.balance > 0.0 {
                    flowTokenRef.deposit(from: <-delRecord.tokensRewarded.withdraw(amount: delRecord.tokensRewarded.balance))
                }
                if delRecord.tokensUnstaked.balance > 0.0 {
                    flowTokenRef.deposit(from: <-delRecord.tokensUnstaked.withdraw(amount: delRecord.tokensUnstaked.balance))
                }
            }

            destroy self.delegators
        }

        /// borrow a reference to to one of the delegators for a node in the record
        /// This gives the caller access to all the public fields on the
        /// object and is basically as if the caller owned the object
        /// The only thing they cannot do is destroy it or move it
        /// This will only be used by the other epoch contracts
        access(contract) fun borrowDelegatorRecord(_ delegatorID: UInt32): &DelegatorRecord {
            pre {
                self.delegators[delegatorID] != nil:
                    "Specified delegator ID does not exist in the record"
            }
            return &self.delegators[delegatorID] as! &DelegatorRecord
        }
    }

    // Struct to create to get read-only info about a node
    pub struct NodeInfo {
        pub let id: String
        pub let role: UInt8
        pub let networkingAddress: String
        pub let networkingKey: String
        pub let stakingKey: String
        pub let tokensStaked: UFix64
        pub let totalTokensStaked: UFix64
        pub let tokensCommitted: UFix64
        pub let tokensUnstaked: UFix64
        pub let tokensUnlocked: UFix64
        pub let tokensRewarded: UFix64

        /// list of delegator IDs for this node operator
        pub let delegators: [UInt32]
        pub let delegatorIDCounter: UInt32
        pub let tokensRequestedToUnstake: UFix64
        pub let initialWeight: UInt64

        init(nodeID: String) {
            let nodeRecord = FlowIDTableStaking.borrowNodeRecord(nodeID)

            self.id = nodeRecord.id
            self.role = nodeRecord.role
            self.networkingAddress = nodeRecord.networkingAddress
            self.networkingKey = nodeRecord.networkingKey
            self.stakingKey = nodeRecord.stakingKey
            self.tokensStaked = nodeRecord.tokensStaked.balance
            self.totalTokensStaked = FlowIDTableStaking.getTotalCommittedBalance(nodeID)
            self.tokensCommitted = nodeRecord.tokensCommitted.balance
            self.tokensUnstaked = nodeRecord.tokensUnstaked.balance
            self.tokensUnlocked = nodeRecord.tokensUnlocked.balance
            self.tokensRewarded = nodeRecord.tokensRewarded.balance
            self.delegators = nodeRecord.delegators.keys
            self.delegatorIDCounter = nodeRecord.delegatorIDCounter
            self.tokensRequestedToUnstake = nodeRecord.tokensRequestedToUnstake
            self.initialWeight = nodeRecord.initialWeight
        }
    }

    /// Records the staking info associated with a delegator
    /// Stored in the NodeRecord resource for the node that a delegator
    /// is associated with
    pub resource DelegatorRecord {

        /// Tokens this delegator has committed for the next epoch
        /// The actual tokens are stored in the node's committed bucket
        pub(set) var tokensCommitted: @FlowToken.Vault

        /// Tokens this delegator has staked for the current epoch
        /// The actual tokens are stored in the node's staked bucket
        pub(set) var tokensStaked: @FlowToken.Vault

        /// Tokens this delegator has unstaked and is locked for the current epoch
        pub(set) var tokensUnstaked: @FlowToken.Vault

        /// Tokens this delegator has been rewarded and can withdraw
        pub let tokensRewarded: @FlowToken.Vault

        /// Tokens that this delegator unstaked and can withdraw
        pub let tokensUnlocked: @FlowToken.Vault

        /// Tokens that the delegator has requested to unstake
        pub(set) var tokensRequestedToUnstake: UFix64

        init() {
            self.tokensCommitted <- FlowToken.createEmptyVault() as! @FlowToken.Vault
            self.tokensStaked <- FlowToken.createEmptyVault() as! @FlowToken.Vault
            self.tokensUnstaked <- FlowToken.createEmptyVault() as! @FlowToken.Vault
            self.tokensRewarded <- FlowToken.createEmptyVault() as! @FlowToken.Vault
            self.tokensUnlocked <- FlowToken.createEmptyVault() as! @FlowToken.Vault
            self.tokensRequestedToUnstake = 0.0
        }

        destroy () {
            destroy self.tokensCommitted
            destroy self.tokensStaked
            destroy self.tokensUnstaked
            destroy self.tokensRewarded
            destroy self.tokensUnlocked
        }
    }

    /// Struct that can be returned to show all the info about a delegator
    pub struct DelegatorInfo {

        pub let id: UInt32
        pub let nodeID: String
        pub let tokensCommitted: UFix64
        pub let tokensStaked: UFix64
        pub let tokensUnstaked: UFix64
        pub let tokensRewarded: UFix64
        pub let tokensUnlocked: UFix64
        pub let tokensRequestedToUnstake: UFix64

        init(nodeID: String, delegatorID: UInt32) {
            let nodeRecord = FlowIDTableStaking.borrowNodeRecord(nodeID)

            let delegatorRecord = nodeRecord.borrowDelegatorRecord(delegatorID)

            self.id = delegatorID
            self.nodeID = nodeID
            self.tokensCommitted = delegatorRecord.tokensCommitted.balance
            self.tokensStaked = delegatorRecord.tokensStaked.balance
            self.tokensUnstaked = delegatorRecord.tokensUnstaked.balance
            self.tokensUnlocked = delegatorRecord.tokensUnlocked.balance
            self.tokensRewarded = delegatorRecord.tokensRewarded.balance
            self.tokensRequestedToUnstake = delegatorRecord.tokensRequestedToUnstake
        }

    }

    /// Resource that the node operator controls for staking
    pub resource NodeStaker {

        /// Unique ID for the node operator
        pub let id: String

        init(id: String) {
            self.id = id
        }

        /// Add new tokens to the system to stake during the next epoch
        pub fun stakeNewTokens(_ tokens: @FungibleToken.Vault) {

            let nodeRecord = FlowIDTableStaking.borrowNodeRecord(self.id)

            emit TokensCommitted(nodeID: nodeRecord.id, amount: tokens.balance)

            /// Add the new tokens to tokens committed
            nodeRecord.tokensCommitted.deposit(from: <-tokens)
        }

        /// Stake tokens that are in the tokensUnlocked bucket
        /// but haven't been officially staked
        pub fun stakeUnlockedTokens(amount: UFix64) {

            let nodeRecord = FlowIDTableStaking.borrowNodeRecord(self.id)

            /// Add the removed tokens to tokens committed
            nodeRecord.tokensCommitted.deposit(from: <-nodeRecord.tokensUnlocked.withdraw(amount: amount))

            emit TokensCommitted(nodeID: nodeRecord.id, amount: amount)
        }

        /// Stake tokens that are in the tokensRewarded bucket
        /// but haven't been officially staked
        pub fun stakeRewardedTokens(amount: UFix64) {

            let nodeRecord = FlowIDTableStaking.borrowNodeRecord(self.id)

            /// Add the removed tokens to tokens committed
            nodeRecord.tokensCommitted.deposit(from: <-nodeRecord.tokensRewarded.withdraw(amount: amount))

            emit TokensCommitted(nodeID: nodeRecord.id, amount: amount)
        }

        /// Request amount tokens to be removed from staking
        /// at the end of the next epoch
        pub fun requestUnStaking(amount: UFix64) {

            let nodeRecord = FlowIDTableStaking.borrowNodeRecord(self.id)

            assert (
                nodeRecord.tokensStaked.balance +
                nodeRecord.tokensCommitted.balance
                >= amount + nodeRecord.tokensRequestedToUnstake,
                message: "Not enough tokens to unstake!"
            )

            assert (
                nodeRecord.delegators.length == 0 ||
                nodeRecord.tokensStaked.balance + nodeRecord.tokensCommitted.balance  - amount >= FlowIDTableStaking.getMinimumStakeRequirements()[nodeRecord.role]!,
                message: "Cannot unstake below the minimum if there are delegators"
            )

            /// Get the balance of the tokens that are currently committed
            let amountCommitted = nodeRecord.tokensCommitted.balance

            /// If the request can come from committed, withdraw from committed to unlocked
            if amountCommitted >= amount {

                /// withdraw the requested tokens from committed since they have not been staked yet
                nodeRecord.tokensUnlocked.deposit(from: <-nodeRecord.tokensCommitted.withdraw(amount: amount))

            } else {
                /// Get the balance of the tokens that are currently committed
                let amountCommitted = nodeRecord.tokensCommitted.balance

                if amountCommitted > 0.0 {
                    nodeRecord.tokensUnlocked.deposit(from: <-nodeRecord.tokensCommitted.withdraw(amount: amountCommitted))
                }

                /// update request to show that leftover amount is requested to be unstaked
                /// at the end of the current epoch
                nodeRecord.tokensRequestedToUnstake = nodeRecord.tokensRequestedToUnstake + (amount - amountCommitted)
            }
        }

        /// Requests to unstake all of the node operators staked and committed tokens,
        /// as well as all the staked and committed tokens of all of their delegators
        pub fun unstakeAll() {
            let nodeRecord = FlowIDTableStaking.borrowNodeRecord(self.id)

            // iterate through all their delegators, uncommit their tokens
            // and request to unstake their staked tokens
            for delegator in nodeRecord.delegators.keys {
                let delRecord = nodeRecord.borrowDelegatorRecord(delegator)

                if delRecord.tokensCommitted.balance > 0.0 {
                    delRecord.tokensUnlocked.deposit(from: <-delRecord.tokensCommitted.withdraw(amount: delRecord.tokensCommitted.balance))
                }

                delRecord.tokensRequestedToUnstake = delRecord.tokensStaked.balance
            }

            /// if the request can come from committed, withdraw from committed to unlocked
            if nodeRecord.tokensCommitted.balance >= 0.0 {

                /// withdraw the requested tokens from committed since they have not been staked yet
                nodeRecord.tokensUnlocked.deposit(from: <-nodeRecord.tokensCommitted.withdraw(amount: nodeRecord.tokensCommitted.balance))

            }

            /// update request to show that leftover amount is requested to be unstaked
            /// at the end of the current epoch
            nodeRecord.tokensRequestedToUnstake = nodeRecord.tokensStaked.balance
        }

        /// Withdraw tokens from the unlocked bucket
        pub fun withdrawUnlockedTokens(amount: UFix64): @FungibleToken.Vault {

            let nodeRecord = FlowIDTableStaking.borrowNodeRecord(self.id)

            emit UnlockedTokensWithdrawn(nodeID: nodeRecord.id, amount: amount)

            return <- nodeRecord.tokensUnlocked.withdraw(amount: amount)
        }

        /// Withdraw tokens from the rewarded bucket
        pub fun withdrawRewardedTokens(amount: UFix64): @FungibleToken.Vault {

            let nodeRecord = FlowIDTableStaking.borrowNodeRecord(self.id)

            emit RewardTokensWithdrawn(nodeID: nodeRecord.id, amount: amount)

            return <- nodeRecord.tokensRewarded.withdraw(amount: amount)
        }

    }

    /// Resource object that the delegator stores in their account
    /// to perform staking actions
    pub resource NodeDelegator {

        /// Each delegator for a node operator has a unique ID
        pub let id: UInt32

        /// The ID of the node operator that this delegator delegates to
        pub let nodeID: String

        init(id: UInt32, nodeID: String) {
            self.id = id
            self.nodeID = nodeID
        }

        /// Delegate new tokens to the node operator
        pub fun delegateNewTokens(from: @FungibleToken.Vault) {

            let nodeRecord = FlowIDTableStaking.borrowNodeRecord(self.nodeID)

            let delRecord = nodeRecord.borrowDelegatorRecord(self.id)

            delRecord.tokensCommitted.deposit(from: <-from)

        }

        /// Delegate tokens from the unlocked bucket to the node operator
        pub fun delegateUnlockedTokens(amount: UFix64) {

            let nodeRecord = FlowIDTableStaking.borrowNodeRecord(self.nodeID)

            let delRecord = nodeRecord.borrowDelegatorRecord(self.id)

            delRecord.tokensCommitted.deposit(from: <-delRecord.tokensUnlocked.withdraw(amount: amount))

        }

        /// Delegate tokens from the rewards bucket to the node operator
        pub fun delegateRewardedTokens(amount: UFix64) {

            let nodeRecord = FlowIDTableStaking.borrowNodeRecord(self.nodeID)

            let delRecord = nodeRecord.borrowDelegatorRecord(self.id)

            delRecord.tokensCommitted.deposit(from: <-delRecord.tokensRewarded.withdraw(amount: amount))

        }

        /// Request to unstake delegated tokens during the next epoch
        pub fun requestUnstaking(amount: UFix64) {

            let nodeRecord = FlowIDTableStaking.borrowNodeRecord(self.nodeID)

            let delRecord = nodeRecord.borrowDelegatorRecord(self.id)

            assert (
                delRecord.tokensStaked.balance +
                delRecord.tokensCommitted.balance
                >= amount + delRecord.tokensRequestedToUnstake,
                message: "Not enough tokens to unstake!"
            )

            /// if the request can come from committed, withdraw from committed to unlocked
            if delRecord.tokensCommitted.balance >= amount {

                /// withdraw the requested tokens from committed since they have not been staked yet
                delRecord.tokensUnlocked.deposit(from: <-delRecord.tokensCommitted.withdraw(amount: amount))

            } else {
                /// Get the balance of the tokens that are currently committed
                let amountCommitted = delRecord.tokensCommitted.balance

                if amountCommitted > 0.0 {
                    delRecord.tokensUnlocked.deposit(from: <-delRecord.tokensCommitted.withdraw(amount: amountCommitted))
                }

                /// update request to show that leftover amount is requested to be unstaked
                /// at the end of the current epoch
                delRecord.tokensRequestedToUnstake = delRecord.tokensRequestedToUnstake + (amount - amountCommitted)
            }
        }

        /// Withdraw tokens from the unlocked bucket
        pub fun withdrawUnlockedTokens(amount: UFix64): @FungibleToken.Vault {

            let nodeRecord = FlowIDTableStaking.borrowNodeRecord(self.nodeID)

            let delRecord = nodeRecord.borrowDelegatorRecord(self.id)

            emit DelegatorUnlockedTokensWithdrawn(nodeID: nodeRecord.id, delegatorID: self.id, amount: amount)

            /// remove the tokens from the unlocked bucket
            return <- delRecord.tokensUnlocked.withdraw(amount: amount)
        }

        /// Withdraw tokens from the rewarded bucket
        pub fun withdrawRewardedTokens(amount: UFix64): @FungibleToken.Vault {

            let nodeRecord = FlowIDTableStaking.borrowNodeRecord(self.nodeID)

            let delRecord = nodeRecord.borrowDelegatorRecord(self.id)

            emit DelegatorRewardTokensWithdrawn(nodeID: nodeRecord.id, delegatorID: self.id, amount: amount)

            /// remove the tokens from the unlocked bucket
            return <- delRecord.tokensRewarded.withdraw(amount: amount)
        }
    }

    /// Admin resource that has the ability to create new staker objects,
    /// remove insufficiently staked nodes at the end of the staking auction,
    /// and pay rewards to nodes at the end of an epoch
    pub resource Admin {

        /// Remove a node from the record
        pub fun removeNode(_ nodeID: String): @NodeRecord {

            // Remove the node from the table
            let node <- FlowIDTableStaking.nodes.remove(key: nodeID)
                ?? panic("Could not find a node with the specified ID")

            return <-node
        }

        /// Iterates through all the registered nodes and if it finds
        /// a node that has insufficient tokens committed for the next epoch
        /// it moves their committed tokens to their unlocked bucket
        /// This will only be called once per epoch
        /// after the staking auction phase
        ///
        /// Also sets the initial weight of all the accepted nodes
        pub fun endStakingAuction(approvedNodeIDs: {String: Bool}) {

            let allNodeIDs = FlowIDTableStaking.getNodeIDs()

            /// remove nodes that have insufficient stake
            for nodeID in allNodeIDs {

                let nodeRecord = FlowIDTableStaking.borrowNodeRecord(nodeID)

                let totalTokensCommitted = FlowIDTableStaking.getTotalCommittedBalance(nodeID)

                /// If the tokens that they have committed for the next epoch
                /// do not meet the minimum requirements
                if (totalTokensCommitted < FlowIDTableStaking.minimumStakeRequired[nodeRecord.role]!) || approvedNodeIDs[nodeID] == nil {

                    emit NodeRemovedAndRefunded(nodeID: nodeRecord.id, amount: nodeRecord.tokensCommitted.balance + nodeRecord.tokensStaked.balance)

                    if nodeRecord.tokensCommitted.balance > 0.0 {
                        /// move their committed tokens back to their unlocked tokens
                        nodeRecord.tokensUnlocked.deposit(from: <-nodeRecord.tokensCommitted.withdraw(amount: nodeRecord.tokensCommitted.balance))
                    }

                    /// Set their request to unstake equal to all their staked tokens
                    /// since they are forced to unstake
                    nodeRecord.tokensRequestedToUnstake = nodeRecord.tokensStaked.balance

                    nodeRecord.initialWeight = 0

                } else {
                    /// Set initial weight of all the committed nodes
                    /// TODO: Figure out how to calculate the initial weight for each node
                    nodeRecord.initialWeight = 100
                }
            }
        }

        /// Called at the end of the epoch to pay rewards to node operators
        /// based on the tokens that they have staked
        pub fun payRewards() {

            let allNodeIDs = FlowIDTableStaking.getNodeIDs()

            let flowTokenMinter = FlowIDTableStaking.account.borrow<&FlowToken.Minter>(from: /storage/flowTokenMinter)
                ?? panic("Could not borrow minter reference")

            // calculate total reward sum for each node type
            // by multiplying the total amount of rewards by the ratio for each node type
            var rewardsForNodeTypes: {UInt8: UFix64} = {}
            rewardsForNodeTypes[UInt8(1)] = FlowIDTableStaking.epochTokenPayout * FlowIDTableStaking.rewardRatios[UInt8(1)]!
            rewardsForNodeTypes[UInt8(2)] = FlowIDTableStaking.epochTokenPayout * FlowIDTableStaking.rewardRatios[UInt8(2)]!
            rewardsForNodeTypes[UInt8(3)] = FlowIDTableStaking.epochTokenPayout * FlowIDTableStaking.rewardRatios[UInt8(3)]!
            rewardsForNodeTypes[UInt8(4)] = FlowIDTableStaking.epochTokenPayout * FlowIDTableStaking.rewardRatios[UInt8(4)]!
            rewardsForNodeTypes[UInt8(5)] = 0.0

            /// iterate through all the nodes
            for nodeID in allNodeIDs {

                let nodeRecord = FlowIDTableStaking.borrowNodeRecord(nodeID)

                if nodeRecord.tokensStaked.balance == 0.0 { continue }

                /// Calculate the amount of tokens that this node operator receives
                let rewardAmount = rewardsForNodeTypes[nodeRecord.role]! * (nodeRecord.tokensStaked.balance / FlowIDTableStaking.totalTokensStakedByNodeType[nodeRecord.role]!)

                /// Mint the tokens to reward the operator
                let tokenReward <- flowTokenMinter.mintTokens(amount: rewardAmount)

                emit RewardsPaid(nodeID: nodeRecord.id, amount: tokenReward.balance)

                // Iterate through all delegators and reward them their share
                // of the rewards for the tokens they have staked for this node
                for delegator in nodeRecord.delegators.keys {
                    let delRecord = nodeRecord.borrowDelegatorRecord(delegator)

                    if delRecord.tokensStaked.balance == 0.0 { continue }

                    let delegatorRewardAmount = (rewardsForNodeTypes[nodeRecord.role]! * (delRecord.tokensStaked.balance / FlowIDTableStaking.totalTokensStakedByNodeType[nodeRecord.role]!))

                    let delegatorReward <- flowTokenMinter.mintTokens(amount: delegatorRewardAmount)

                    // take the node operator's cut
                    tokenReward.deposit(from: <-delegatorReward.withdraw(amount: delegatorReward.balance * FlowIDTableStaking.nodeDelegatingRewardCut))

                    emit DelegatorRewardsPaid(nodeID: nodeRecord.id, delegatorID: delegator, amount: delegatorRewardAmount)

                    if delegatorReward.balance > 0.0 {
                        delRecord.tokensRewarded.deposit(from: <-delegatorReward)
                    } else {
                        destroy delegatorReward
                    }
                }

                /// Deposit the node Rewards into their tokensRewarded bucket
                nodeRecord.tokensRewarded.deposit(from: <-tokenReward)
            }
        }

        /// Called at the end of the epoch to move tokens between buckets
        /// for stakers
        /// Tokens that have been committed are moved to the staked bucket
        /// Tokens that were unstaked during the last epoch are fully unlocked
        /// Unstaking requests are filled by moving those tokens from staked to unstaked
        pub fun moveTokens() {

            let allNodeIDs = FlowIDTableStaking.getNodeIDs()

            for nodeID in allNodeIDs {

                let nodeRecord = FlowIDTableStaking.borrowNodeRecord(nodeID)

                // Update total number of tokens staked by all the nodes of each type
                FlowIDTableStaking.totalTokensStakedByNodeType[nodeRecord.role] = FlowIDTableStaking.totalTokensStakedByNodeType[nodeRecord.role]! + nodeRecord.tokensCommitted.balance

                if nodeRecord.tokensCommitted.balance > 0.0 {
                    emit TokensStaked(nodeID: nodeRecord.id, amount: nodeRecord.tokensCommitted.balance)
                    nodeRecord.tokensStaked.deposit(from: <-nodeRecord.tokensCommitted.withdraw(amount: nodeRecord.tokensCommitted.balance))
                }
                if nodeRecord.tokensUnstaked.balance > 0.0 {
                    nodeRecord.tokensUnlocked.deposit(from: <-nodeRecord.tokensUnstaked.withdraw(amount: nodeRecord.tokensUnstaked.balance))
                }
                if nodeRecord.tokensRequestedToUnstake > 0.0 {
                    emit TokensUnStaked(nodeID: nodeRecord.id, amount: nodeRecord.tokensRequestedToUnstake)
                    nodeRecord.tokensUnstaked.deposit(from: <-nodeRecord.tokensStaked.withdraw(amount: nodeRecord.tokensRequestedToUnstake))
                }

                // move all the delegators' tokens between buckets
                for delegator in nodeRecord.delegators.keys {

                    let delRecord = nodeRecord.borrowDelegatorRecord(delegator)

                    FlowIDTableStaking.totalTokensStakedByNodeType[nodeRecord.role] = FlowIDTableStaking.totalTokensStakedByNodeType[nodeRecord.role]! + delRecord.tokensCommitted.balance

                    // mark their committed tokens as staked
                    if delRecord.tokensCommitted.balance > 0.0 {
                        delRecord.tokensStaked.deposit(from: <-delRecord.tokensCommitted.withdraw(amount: delRecord.tokensCommitted.balance))
                    }

                    if delRecord.tokensUnstaked.balance > 0.0 {
                        delRecord.tokensUnlocked.deposit(from: <-delRecord.tokensUnstaked.withdraw(amount: delRecord.tokensUnstaked.balance))
                    }

                    if delRecord.tokensRequestedToUnstake > 0.0 {
                        delRecord.tokensUnstaked.deposit(from: <-delRecord.tokensStaked.withdraw(amount: delRecord.tokensRequestedToUnstake))
                        emit TokensUnStaked(nodeID: nodeRecord.id, amount: delRecord.tokensRequestedToUnstake)
                    }

                    // subtract their requested tokens from the total staked for their node type
                    FlowIDTableStaking.totalTokensStakedByNodeType[nodeRecord.role] = FlowIDTableStaking.totalTokensStakedByNodeType[nodeRecord.role]! - delRecord.tokensRequestedToUnstake

                    delRecord.tokensRequestedToUnstake = 0.0
                }

                // subtract their requested tokens from the total staked for their node type
                FlowIDTableStaking.totalTokensStakedByNodeType[nodeRecord.role] = FlowIDTableStaking.totalTokensStakedByNodeType[nodeRecord.role]! - nodeRecord.tokensRequestedToUnstake

                // Reset the tokens requested field so it can be used for the next epoch
                nodeRecord.tokensRequestedToUnstake = 0.0
            }
        }

        // Changes the total weekly payout to a new value
        pub fun updateEpochTokenPayout(_ newPayout: UFix64) {
            FlowIDTableStaking.epochTokenPayout = newPayout
        }

        /// Admin calls this to change the percentage
        /// of delegator rewards every node operator takes
        pub fun changeCutPercentage(_ newCutPercentage: UFix64) {
            pre {
                newCutPercentage > 0.0 && newCutPercentage < 1.0:
                    "Cut percentage must be between 0 and 1!"
            }

            FlowIDTableStaking.nodeDelegatingRewardCut = newCutPercentage

            emit NewDelegatorCutPercentage(newCutPercentage: FlowIDTableStaking.nodeDelegatingRewardCut)
        }
    }

    /// Any node can call this function to register a new Node
    /// It returns the resource for nodes that they can store in
    /// their account storage
    pub fun addNodeRecord(id: String, role: UInt8, networkingAddress: String, networkingKey: String, stakingKey: String, tokensCommitted: @FungibleToken.Vault): @NodeStaker {

        let initialBalance = tokensCommitted.balance

        let newNode <- create NodeRecord(id: id, role: role, networkingAddress: networkingAddress, networkingKey: networkingKey, stakingKey: stakingKey, tokensCommitted: <-tokensCommitted)

        // Insert the node to the table
        FlowIDTableStaking.nodes[id] <-! newNode

        // return a new NodeStaker object that the node operator stores in their account
        return <-create NodeStaker(id: id)

    }

    /// Registers a new delegator with a unique ID for the specified node operator
    /// and returns a delegator object to the caller
    /// The node operator would make a public capability for potential delegators
    /// to access this function
    pub fun registerNewDelegator(nodeID: String): @NodeDelegator {

        let nodeRecord = FlowIDTableStaking.borrowNodeRecord(nodeID)

        assert (
            FlowIDTableStaking.getTotalCommittedBalance(nodeID) > FlowIDTableStaking.minimumStakeRequired[nodeRecord.role]!,
            message: "Cannot register a delegator if the node operator is below the minimum stake"
        )

        nodeRecord.delegatorIDCounter = nodeRecord.delegatorIDCounter + UInt32(1)

        nodeRecord.delegators[nodeRecord.delegatorIDCounter] <-! create DelegatorRecord()

        emit NewDelegatorCreated(nodeID: nodeRecord.id, delegatorID: nodeRecord.delegatorIDCounter)

        return <-create NodeDelegator(id: nodeRecord.delegatorIDCounter, nodeID: nodeRecord.id)
    }

    /// borrow a reference to to one of the nodes in the record
    /// This gives the caller access to all the public fields on the
    /// objects and is basically as if the caller owned the object
    /// The only thing they cannot do is destroy it or move it
    /// This will only be used by the other epoch contracts
    access(contract) fun borrowNodeRecord(_ nodeID: String): &NodeRecord {
        pre {
            FlowIDTableStaking.nodes[nodeID] != nil:
                "Specified node ID does not exist in the record"
        }
        return &FlowIDTableStaking.nodes[nodeID] as! &NodeRecord
    }

    /****************** Getter Functions for the staking Info *******************/

    /// Gets an array of the node IDs that are proposed for the next epoch
    /// Nodes that are proposed are nodes that have enough tokens staked + committed
    /// for the next epoch
    pub fun getProposedNodeIDs(): [String] {
        var proposedNodes: [String] = []

        for nodeID in FlowIDTableStaking.getNodeIDs() {
            let delRecord = FlowIDTableStaking.borrowNodeRecord(nodeID)

            if self.getTotalCommittedBalance(nodeID) >= self.minimumStakeRequired[delRecord.role]!  {
                proposedNodes.append(nodeID)
            }
        }

        return proposedNodes
    }

    /// Gets an array of all the nodeIDs that are staked.
    /// Only nodes that are participating in the current epoch
    /// can be staked, so this is an array of all the active
    /// node operators
    pub fun getStakedNodeIDs(): [String] {
        var stakedNodes: [String] = []

        for nodeID in FlowIDTableStaking.getNodeIDs() {
            let nodeRecord = FlowIDTableStaking.borrowNodeRecord(nodeID)

            if nodeRecord.tokensStaked.balance >= self.minimumStakeRequired[nodeRecord.role]!  {
                stakedNodes.append(nodeID)
            }
        }

        return stakedNodes
    }

    /// Gets an array of all the node IDs that have ever applied
    pub fun getNodeIDs(): [String] {
        return FlowIDTableStaking.nodes.keys
    }

    /// Gets the total amount of tokens that have been staked and
    /// committed for a node. The sum from the node operator and all
    /// its delegators
    pub fun getTotalCommittedBalance(_ nodeID: String): UFix64 {
        let nodeRecord = self.borrowNodeRecord(nodeID)

        if (nodeRecord.tokensCommitted.balance + nodeRecord.tokensStaked.balance) < nodeRecord.tokensRequestedToUnstake {
            return 0.0
        } else {
            var sum: UFix64 = 0.0

            sum = nodeRecord.tokensCommitted.balance + nodeRecord.tokensStaked.balance - nodeRecord.tokensRequestedToUnstake

            for delegator in nodeRecord.delegators.keys {
                let delRecord = nodeRecord.borrowDelegatorRecord(delegator)
                sum = sum + delRecord.tokensCommitted.balance + delRecord.tokensStaked.balance - delRecord.tokensRequestedToUnstake
            }

            return sum
        }
    }

    /// Functions to return contract fields

    pub fun getMinimumStakeRequirements(): {UInt8: UFix64} {
        return self.minimumStakeRequired
    }

    pub fun getTotalTokensStakedByNodeType(): {UInt8: UFix64} {
        return self.totalTokensStakedByNodeType
    }

    pub fun getEpochTokenPayout(): UFix64 {
        return self.epochTokenPayout
    }

    pub fun getRewardRatios(): {UInt8: UFix64} {
        return self.rewardRatios
    }

    init() {
        self.nodes <- {}

        self.NodeStakerStoragePath = /storage/flowStaker
        self.NodeStakerPublicPath = /public/flowStaker
        self.StakingAdminStoragePath = /storage/flowStakingAdmin
        self.DelegatorStoragePath = /storage/flowStakingDelegator

        // minimum stakes for each node types
        self.minimumStakeRequired = {UInt8(1): 250000.0, UInt8(2): 500000.0, UInt8(3): 1250000.0, UInt8(4): 135000.0, UInt8(5): 0.0}

        self.totalTokensStakedByNodeType = {UInt8(1): 0.0, UInt8(2): 0.0, UInt8(3): 0.0, UInt8(4): 0.0, UInt8(5): 0.0}

        // 1.25M FLOW paid out in the first week. Decreasing in subsequent weeks
        self.epochTokenPayout = 1250000.0

        // initialize the cut of rewards that node operators take to 3%
        self.nodeDelegatingRewardCut = 0.03

        // The preliminary percentage of rewards that go to each node type every epoch
        // subject to change
        self.rewardRatios = {UInt8(1): 0.168, UInt8(2): 0.518, UInt8(3): 0.078, UInt8(4): 0.236, UInt8(5): 0.0}

        // save the admin object to storage
        self.account.save(<-create Admin(), to: self.StakingAdminStoragePath)
    }
}
`

//nolint:gosimple
const knownContract_dee35303492e5a0b_FlowIDTableStaking = `
import FungibleToken from 0xf233dcee88fe0abe
import FlowToken from 0x1654653399040a61

pub contract FlowIDTableStaking {

    /********************* ID Table and Staking Events **********************/
    pub event NewNodeCreated(nodeID: String, role: UInt8, amountCommitted: UFix64)
    pub event TokensCommitted(nodeID: String, amount: UFix64)
    pub event TokensStaked(nodeID: String, amount: UFix64)
    pub event TokensUnstaking(nodeID: String, amount: UFix64)
    pub event NodeRemovedAndRefunded(nodeID: String, amount: UFix64)
    pub event RewardsPaid(nodeID: String, amount: UFix64)
    pub event UnstakedTokensWithdrawn(nodeID: String, amount: UFix64)
    pub event RewardTokensWithdrawn(nodeID: String, amount: UFix64)
    pub event NewDelegatorCutPercentage(newCutPercentage: UFix64)

    /// Delegator Events
    pub event NewDelegatorCreated(nodeID: String, delegatorID: UInt32)
    pub event DelegatorRewardsPaid(nodeID: String, delegatorID: UInt32, amount: UFix64)
    pub event DelegatorUnstakedTokensWithdrawn(nodeID: String, delegatorID: UInt32, amount: UFix64)
    pub event DelegatorRewardTokensWithdrawn(nodeID: String, delegatorID: UInt32, amount: UFix64)

    /// Holds the identity table for all the nodes in the network.
    /// Includes nodes that aren't actively participating
    /// key = node ID
    /// value = the record of that node's info, tokens, and delegators
    access(contract) var nodes: @{String: NodeRecord}

    /// The minimum amount of tokens that each node type has to stake
    /// in order to be considered valid
    /// key = node role
    /// value = amount of tokens
    access(contract) var minimumStakeRequired: {UInt8: UFix64}

    /// The total amount of tokens that are staked for all the nodes
    /// of each node type during the current epoch
    /// key = node role
    /// value = amount of tokens
    access(contract) var totalTokensStakedByNodeType: {UInt8: UFix64}

    /// The total amount of tokens that are paid as rewards every epoch
    /// could be manually changed by the admin resource
    pub var epochTokenPayout: UFix64

    /// The ratio of the weekly awards that each node type gets
    /// key = node role
    /// value = decimal number between 0 and 1 indicating a percentage
    access(contract) var rewardRatios: {UInt8: UFix64}

    /// The percentage of rewards that every node operator takes from
    /// the users that are delegating to it
    pub var nodeDelegatingRewardCut: UFix64

    /// Paths for storing staking resources
    pub let NodeStakerStoragePath: Path
    pub let NodeStakerPublicPath: Path
    pub let StakingAdminStoragePath: StoragePath
    pub let DelegatorStoragePath: Path

    /*********** ID Table and Staking Composite Type Definitions *************/

    /// Contains information that is specific to a node in Flow
    /// only lives in this contract
    pub resource NodeRecord {

        /// The unique ID of the node
        /// Set when the node is created
        pub let id: String

        /// The type of node:
        /// 1 = collection
        /// 2 = consensus
        /// 3 = execution
        /// 4 = verification
        /// 5 = access
        pub var role: UInt8

        /// The address used for networking
        pub(set) var networkingAddress: String

        /// the public key for networking
        pub(set) var networkingKey: String

        /// the public key for staking
        pub(set) var stakingKey: String

        /// The total tokens that this node currently has staked, including delegators
        /// This value must always be above the minimum requirement to stay staked
        /// or accept delegators
        pub var tokensStaked: @FlowToken.Vault

        /// The tokens that this node has committed to stake for the next epoch.
        pub var tokensCommitted: @FlowToken.Vault

        /// The tokens that this node has unstaked from the previous epoch
        /// Moves to the tokensUnstaked bucket at the end of the epoch.
        pub var tokensUnstaking: @FlowToken.Vault

        /// Tokens that this node is able to withdraw whenever they want
        /// Staking rewards are paid to this bucket
        pub var tokensUnstaked: @FlowToken.Vault

        /// Staking rewards are paid to this bucket
        /// Can be withdrawn whenever
        pub var tokensRewarded: @FlowToken.Vault

        /// list of delegators for this node operator
        pub let delegators: @{UInt32: DelegatorRecord}

        /// The incrementing ID used to register new delegators
        pub(set) var delegatorIDCounter: UInt32

        /// The amount of tokens that this node has requested to unstake
        /// for the next epoch
        pub(set) var tokensRequestedToUnstake: UFix64

        /// weight as determined by the amount staked after the staking auction
        pub(set) var initialWeight: UInt64

        init(id: String,
             role: UInt8,  /// role that the node will have for future epochs
             networkingAddress: String,
             networkingKey: String,
             stakingKey: String,
             tokensCommitted: @FungibleToken.Vault
        ) {
            pre {
                id.length == 64: "Node ID length must be 32 bytes (64 hex characters)"
                FlowIDTableStaking.nodes[id] == nil: "The ID cannot already exist in the record"
                role >= UInt8(1) && role <= UInt8(5): "The role must be 1, 2, 3, 4, or 5"
                networkingAddress.length > 0: "The networkingAddress cannot be empty"
            }

            /// Assert that the addresses and keys are not already in use
            /// They must be unique
            for nodeID in FlowIDTableStaking.nodes.keys {
                assert (
                    networkingAddress != FlowIDTableStaking.nodes[nodeID]?.networkingAddress,
                    message: "Networking Address is already in use!"
                )
                assert (
                    networkingKey != FlowIDTableStaking.nodes[nodeID]?.networkingKey,
                    message: "Networking Key is already in use!"
                )
                assert (
                    stakingKey != FlowIDTableStaking.nodes[nodeID]?.stakingKey,
                    message: "Staking Key is already in use!"
                )
            }

            self.id = id
            self.role = role
            self.networkingAddress = networkingAddress
            self.networkingKey = networkingKey
            self.stakingKey = stakingKey
            self.initialWeight = 0
            self.delegators <- {}
            self.delegatorIDCounter = 0

            self.tokensCommitted <- tokensCommitted as! @FlowToken.Vault
            self.tokensStaked <- FlowToken.createEmptyVault() as! @FlowToken.Vault
            self.tokensUnstaking <- FlowToken.createEmptyVault() as! @FlowToken.Vault
            self.tokensUnstaked <- FlowToken.createEmptyVault() as! @FlowToken.Vault
            self.tokensRewarded <- FlowToken.createEmptyVault() as! @FlowToken.Vault
            self.tokensRequestedToUnstake = 0.0

            emit NewNodeCreated(nodeID: self.id, role: self.role, amountCommitted: self.tokensCommitted.balance)
        }

        destroy() {
            let flowTokenRef = FlowIDTableStaking.account.borrow<&FlowToken.Vault>(from: /storage/flowTokenVault)!
            if self.tokensStaked.balance > 0.0 {
                FlowIDTableStaking.totalTokensStakedByNodeType[self.role] = FlowIDTableStaking.totalTokensStakedByNodeType[self.role]! - self.tokensStaked.balance
                flowTokenRef.deposit(from: <-self.tokensStaked)
            } else { destroy self.tokensStaked }
            if self.tokensCommitted.balance > 0.0 {
                flowTokenRef.deposit(from: <-self.tokensCommitted)
            } else { destroy  self.tokensCommitted }
            if self.tokensUnstaking.balance > 0.0 {
                flowTokenRef.deposit(from: <-self.tokensUnstaking)
            } else { destroy  self.tokensUnstaking }
            if self.tokensUnstaked.balance > 0.0 {
                flowTokenRef.deposit(from: <-self.tokensUnstaked)
            } else { destroy  self.tokensUnstaked }
            if self.tokensRewarded.balance > 0.0 {
                flowTokenRef.deposit(from: <-self.tokensRewarded)
            } else { destroy  self.tokensRewarded }

            // Return all of the delegators' funds
            for delegator in self.delegators.keys {
                let delRecord = self.borrowDelegatorRecord(delegator)
                if delRecord.tokensCommitted.balance > 0.0 {
                    flowTokenRef.deposit(from: <-delRecord.tokensCommitted.withdraw(amount: delRecord.tokensCommitted.balance))
                }
                if delRecord.tokensStaked.balance > 0.0 {
                    flowTokenRef.deposit(from: <-delRecord.tokensStaked.withdraw(amount: delRecord.tokensStaked.balance))
                }
                if delRecord.tokensUnstaked.balance > 0.0 {
                    flowTokenRef.deposit(from: <-delRecord.tokensUnstaked.withdraw(amount: delRecord.tokensUnstaked.balance))
                }
                if delRecord.tokensRewarded.balance > 0.0 {
                    flowTokenRef.deposit(from: <-delRecord.tokensRewarded.withdraw(amount: delRecord.tokensRewarded.balance))
                }
                if delRecord.tokensUnstaking.balance > 0.0 {
                    flowTokenRef.deposit(from: <-delRecord.tokensUnstaking.withdraw(amount: delRecord.tokensUnstaking.balance))
                }
            }

            destroy self.delegators
        }

        /// borrow a reference to to one of the delegators for a node in the record
        /// This gives the caller access to all the public fields on the
        /// object and is basically as if the caller owned the object
        /// The only thing they cannot do is destroy it or move it
        /// This will only be used by the other epoch contracts
        access(contract) fun borrowDelegatorRecord(_ delegatorID: UInt32): &DelegatorRecord {
            pre {
                self.delegators[delegatorID] != nil:
                    "Specified delegator ID does not exist in the record"
            }
            return &self.delegators[delegatorID] as! &DelegatorRecord
        }
    }

    // Struct to create to get read-only info about a node
    pub struct NodeInfo {
        pub let id: String
        pub let role: UInt8
        pub let networkingAddress: String
        pub let networkingKey: String
        pub let stakingKey: String
        pub let tokensStaked: UFix64
        pub let totalTokensStaked: UFix64
        pub let tokensCommitted: UFix64
        pub let tokensUnstaking: UFix64
        pub let tokensUnstaked: UFix64
        pub let tokensRewarded: UFix64

        /// list of delegator IDs for this node operator
        pub let delegators: [UInt32]
        pub let delegatorIDCounter: UInt32
        pub let tokensRequestedToUnstake: UFix64
        pub let initialWeight: UInt64

        init(nodeID: String) {
            let nodeRecord = FlowIDTableStaking.borrowNodeRecord(nodeID)

            self.id = nodeRecord.id
            self.role = nodeRecord.role
            self.networkingAddress = nodeRecord.networkingAddress
            self.networkingKey = nodeRecord.networkingKey
            self.stakingKey = nodeRecord.stakingKey
            self.tokensStaked = nodeRecord.tokensStaked.balance
            self.totalTokensStaked = FlowIDTableStaking.getTotalCommittedBalance(nodeID)
            self.tokensCommitted = nodeRecord.tokensCommitted.balance
            self.tokensUnstaking = nodeRecord.tokensUnstaking.balance
            self.tokensUnstaked = nodeRecord.tokensUnstaked.balance
            self.tokensRewarded = nodeRecord.tokensRewarded.balance
            self.delegators = nodeRecord.delegators.keys
            self.delegatorIDCounter = nodeRecord.delegatorIDCounter
            self.tokensRequestedToUnstake = nodeRecord.tokensRequestedToUnstake
            self.initialWeight = nodeRecord.initialWeight
        }
    }

    /// Records the staking info associated with a delegator
    /// Stored in the NodeRecord resource for the node that a delegator
    /// is associated with
    pub resource DelegatorRecord {

        /// Tokens this delegator has committed for the next epoch
        /// The actual tokens are stored in the node's committed bucket
        pub(set) var tokensCommitted: @FlowToken.Vault

        /// Tokens this delegator has staked for the current epoch
        /// The actual tokens are stored in the node's staked bucket
        pub(set) var tokensStaked: @FlowToken.Vault

        /// Tokens this delegator has requested to unstake and is locked for the current epoch
        pub(set) var tokensUnstaking: @FlowToken.Vault

        /// Tokens this delegator has been rewarded and can withdraw
        pub let tokensRewarded: @FlowToken.Vault

        /// Tokens that this delegator unstaked and can withdraw
        pub let tokensUnstaked: @FlowToken.Vault

        /// Tokens that the delegator has requested to unstake
        pub(set) var tokensRequestedToUnstake: UFix64

        init() {
            self.tokensCommitted <- FlowToken.createEmptyVault() as! @FlowToken.Vault
            self.tokensStaked <- FlowToken.createEmptyVault() as! @FlowToken.Vault
            self.tokensUnstaking <- FlowToken.createEmptyVault() as! @FlowToken.Vault
            self.tokensRewarded <- FlowToken.createEmptyVault() as! @FlowToken.Vault
            self.tokensUnstaked <- FlowToken.createEmptyVault() as! @FlowToken.Vault
            self.tokensRequestedToUnstake = 0.0
        }

        destroy () {
            destroy self.tokensCommitted
            destroy self.tokensStaked
            destroy self.tokensUnstaking
            destroy self.tokensRewarded
            destroy self.tokensUnstaked
        }
    }

    /// Struct that can be returned to show all the info about a delegator
    pub struct DelegatorInfo {

        pub let id: UInt32
        pub let nodeID: String
        pub let tokensCommitted: UFix64
        pub let tokensStaked: UFix64
        pub let tokensUnstaking: UFix64
        pub let tokensRewarded: UFix64
        pub let tokensUnstaked: UFix64
        pub let tokensRequestedToUnstake: UFix64

        init(nodeID: String, delegatorID: UInt32) {
            let nodeRecord = FlowIDTableStaking.borrowNodeRecord(nodeID)

            let delegatorRecord = nodeRecord.borrowDelegatorRecord(delegatorID)

            self.id = delegatorID
            self.nodeID = nodeID
            self.tokensCommitted = delegatorRecord.tokensCommitted.balance
            self.tokensStaked = delegatorRecord.tokensStaked.balance
            self.tokensUnstaking = delegatorRecord.tokensUnstaking.balance
            self.tokensUnstaked = delegatorRecord.tokensUnstaked.balance
            self.tokensRewarded = delegatorRecord.tokensRewarded.balance
            self.tokensRequestedToUnstake = delegatorRecord.tokensRequestedToUnstake
        }

    }

    /// Resource that the node operator controls for staking
    pub resource NodeStaker {

        /// Unique ID for the node operator
        pub let id: String

        init(id: String) {
            self.id = id
        }

        /// Add new tokens to the system to stake during the next epoch
        pub fun stakeNewTokens(_ tokens: @FungibleToken.Vault) {

            let nodeRecord = FlowIDTableStaking.borrowNodeRecord(self.id)

            emit TokensCommitted(nodeID: nodeRecord.id, amount: tokens.balance)

            /// Add the new tokens to tokens committed
            nodeRecord.tokensCommitted.deposit(from: <-tokens)
        }

        /// Stake tokens that are in the tokensUnstaked bucket
        /// but haven't been officially staked
        pub fun stakeUnstakedTokens(amount: UFix64) {

            let nodeRecord = FlowIDTableStaking.borrowNodeRecord(self.id)

            /// Add the removed tokens to tokens committed
            nodeRecord.tokensCommitted.deposit(from: <-nodeRecord.tokensUnstaked.withdraw(amount: amount))

            emit TokensCommitted(nodeID: nodeRecord.id, amount: amount)
        }

        /// Stake tokens that are in the tokensRewarded bucket
        /// but haven't been officially staked
        pub fun stakeRewardedTokens(amount: UFix64) {

            let nodeRecord = FlowIDTableStaking.borrowNodeRecord(self.id)

            /// Add the removed tokens to tokens committed
            nodeRecord.tokensCommitted.deposit(from: <-nodeRecord.tokensRewarded.withdraw(amount: amount))

            emit TokensCommitted(nodeID: nodeRecord.id, amount: amount)
        }

        /// Request amount tokens to be removed from staking
        /// at the end of the next epoch
        pub fun requestUnstaking(amount: UFix64) {

            let nodeRecord = FlowIDTableStaking.borrowNodeRecord(self.id)

            assert (
                nodeRecord.tokensStaked.balance +
                nodeRecord.tokensCommitted.balance
                >= amount + nodeRecord.tokensRequestedToUnstake,
                message: "Not enough tokens to unstake!"
            )

            assert (
                nodeRecord.delegators.length == 0 ||
                nodeRecord.tokensStaked.balance + nodeRecord.tokensCommitted.balance  - amount >= FlowIDTableStaking.getMinimumStakeRequirements()[nodeRecord.role]!,
                message: "Cannot unstake below the minimum if there are delegators"
            )

            /// Get the balance of the tokens that are currently committed
            let amountCommitted = nodeRecord.tokensCommitted.balance

            /// If the request can come from committed, withdraw from committed to unstaked
            if amountCommitted >= amount {

                /// withdraw the requested tokens from committed since they have not been staked yet
                nodeRecord.tokensUnstaked.deposit(from: <-nodeRecord.tokensCommitted.withdraw(amount: amount))

            } else {
                /// Get the balance of the tokens that are currently committed
                let amountCommitted = nodeRecord.tokensCommitted.balance

                if amountCommitted > 0.0 {
                    nodeRecord.tokensUnstaked.deposit(from: <-nodeRecord.tokensCommitted.withdraw(amount: amountCommitted))
                }

                /// update request to show that leftover amount is requested to be unstaked
                /// at the end of the current epoch
                nodeRecord.tokensRequestedToUnstake = nodeRecord.tokensRequestedToUnstake + (amount - amountCommitted)
            }
        }

        /// Requests to unstake all of the node operators staked and committed tokens,
        /// as well as all the staked and committed tokens of all of their delegators
        pub fun unstakeAll() {
            let nodeRecord = FlowIDTableStaking.borrowNodeRecord(self.id)

            // iterate through all their delegators, uncommit their tokens
            // and request to unstake their staked tokens
            for delegator in nodeRecord.delegators.keys {
                let delRecord = nodeRecord.borrowDelegatorRecord(delegator)

                if delRecord.tokensCommitted.balance > 0.0 {
                    delRecord.tokensUnstaked.deposit(from: <-delRecord.tokensCommitted.withdraw(amount: delRecord.tokensCommitted.balance))
                }

                delRecord.tokensRequestedToUnstake = delRecord.tokensStaked.balance
            }

            /// if the request can come from committed, withdraw from committed to unstaked
            if nodeRecord.tokensCommitted.balance >= 0.0 {

                /// withdraw the requested tokens from committed since they have not been staked yet
                nodeRecord.tokensUnstaked.deposit(from: <-nodeRecord.tokensCommitted.withdraw(amount: nodeRecord.tokensCommitted.balance))

            }

            /// update request to show that leftover amount is requested to be unstaked
            /// at the end of the current epoch
            nodeRecord.tokensRequestedToUnstake = nodeRecord.tokensStaked.balance
        }

        /// Withdraw tokens from the unstaked bucket
        pub fun withdrawUnstakedTokens(amount: UFix64): @FungibleToken.Vault {

            let nodeRecord = FlowIDTableStaking.borrowNodeRecord(self.id)

            emit UnstakedTokensWithdrawn(nodeID: nodeRecord.id, amount: amount)

            return <- nodeRecord.tokensUnstaked.withdraw(amount: amount)
        }

        /// Withdraw tokens from the rewarded bucket
        pub fun withdrawRewardedTokens(amount: UFix64): @FungibleToken.Vault {

            let nodeRecord = FlowIDTableStaking.borrowNodeRecord(self.id)

            emit RewardTokensWithdrawn(nodeID: nodeRecord.id, amount: amount)

            return <- nodeRecord.tokensRewarded.withdraw(amount: amount)
        }

    }

    /// Resource object that the delegator stores in their account
    /// to perform staking actions
    pub resource NodeDelegator {

        /// Each delegator for a node operator has a unique ID
        pub let id: UInt32

        /// The ID of the node operator that this delegator delegates to
        pub let nodeID: String

        init(id: UInt32, nodeID: String) {
            self.id = id
            self.nodeID = nodeID
        }

        /// Delegate new tokens to the node operator
        pub fun delegateNewTokens(from: @FungibleToken.Vault) {

            let nodeRecord = FlowIDTableStaking.borrowNodeRecord(self.nodeID)

            let delRecord = nodeRecord.borrowDelegatorRecord(self.id)

            delRecord.tokensCommitted.deposit(from: <-from)

        }

        /// Delegate tokens from the unstaked bucket to the node operator
        pub fun delegateUnstakedTokens(amount: UFix64) {

            let nodeRecord = FlowIDTableStaking.borrowNodeRecord(self.nodeID)

            let delRecord = nodeRecord.borrowDelegatorRecord(self.id)

            delRecord.tokensCommitted.deposit(from: <-delRecord.tokensUnstaked.withdraw(amount: amount))

        }

        /// Delegate tokens from the rewards bucket to the node operator
        pub fun delegateRewardedTokens(amount: UFix64) {

            let nodeRecord = FlowIDTableStaking.borrowNodeRecord(self.nodeID)

            let delRecord = nodeRecord.borrowDelegatorRecord(self.id)

            delRecord.tokensCommitted.deposit(from: <-delRecord.tokensRewarded.withdraw(amount: amount))

        }

        /// Request to unstake delegated tokens during the next epoch
        pub fun requestUnstaking(amount: UFix64) {

            let nodeRecord = FlowIDTableStaking.borrowNodeRecord(self.nodeID)

            let delRecord = nodeRecord.borrowDelegatorRecord(self.id)

            assert (
                delRecord.tokensStaked.balance +
                delRecord.tokensCommitted.balance
                >= amount + delRecord.tokensRequestedToUnstake,
                message: "Not enough tokens to unstake!"
            )

            /// if the request can come from committed, withdraw from committed to unstaked
            if delRecord.tokensCommitted.balance >= amount {

                /// withdraw the requested tokens from committed since they have not been staked yet
                delRecord.tokensUnstaked.deposit(from: <-delRecord.tokensCommitted.withdraw(amount: amount))

            } else {
                /// Get the balance of the tokens that are currently committed
                let amountCommitted = delRecord.tokensCommitted.balance

                if amountCommitted > 0.0 {
                    delRecord.tokensUnstaked.deposit(from: <-delRecord.tokensCommitted.withdraw(amount: amountCommitted))
                }

                /// update request to show that leftover amount is requested to be unstaked
                /// at the end of the current epoch
                delRecord.tokensRequestedToUnstake = delRecord.tokensRequestedToUnstake + (amount - amountCommitted)
            }
        }

        /// Withdraw tokens from the unstaked bucket
        pub fun withdrawUnstakedTokens(amount: UFix64): @FungibleToken.Vault {

            let nodeRecord = FlowIDTableStaking.borrowNodeRecord(self.nodeID)

            let delRecord = nodeRecord.borrowDelegatorRecord(self.id)

            emit DelegatorUnstakedTokensWithdrawn(nodeID: nodeRecord.id, delegatorID: self.id, amount: amount)

            /// remove the tokens from the unstaked bucket
            return <- delRecord.tokensUnstaked.withdraw(amount: amount)
        }

        /// Withdraw tokens from the rewarded bucket
        pub fun withdrawRewardedTokens(amount: UFix64): @FungibleToken.Vault {

            let nodeRecord = FlowIDTableStaking.borrowNodeRecord(self.nodeID)

            let delRecord = nodeRecord.borrowDelegatorRecord(self.id)

            emit DelegatorRewardTokensWithdrawn(nodeID: nodeRecord.id, delegatorID: self.id, amount: amount)

            /// remove the tokens from the rewarded bucket
            return <- delRecord.tokensRewarded.withdraw(amount: amount)
        }
    }

    /// Admin resource that has the ability to create new staker objects,
    /// remove insufficiently staked nodes at the end of the staking auction,
    /// and pay rewards to nodes at the end of an epoch
    pub resource Admin {

        /// Remove a node from the record
        pub fun removeNode(_ nodeID: String): @NodeRecord {

            // Remove the node from the table
            let node <- FlowIDTableStaking.nodes.remove(key: nodeID)
                ?? panic("Could not find a node with the specified ID")

            return <-node
        }

        /// Iterates through all the registered nodes and if it finds
        /// a node that has insufficient tokens committed for the next epoch
        /// it moves their committed tokens to their unstaked bucket
        /// This will only be called once per epoch
        /// after the staking auction phase
        ///
        /// Also sets the initial weight of all the accepted nodes
        pub fun endStakingAuction(approvedNodeIDs: {String: Bool}) {

            let allNodeIDs = FlowIDTableStaking.getNodeIDs()

            /// remove nodes that have insufficient stake
            for nodeID in allNodeIDs {

                let nodeRecord = FlowIDTableStaking.borrowNodeRecord(nodeID)

                let totalTokensCommitted = FlowIDTableStaking.getTotalCommittedBalance(nodeID)

                /// If the tokens that they have committed for the next epoch
                /// do not meet the minimum requirements
                if (totalTokensCommitted < FlowIDTableStaking.minimumStakeRequired[nodeRecord.role]!) || approvedNodeIDs[nodeID] == nil {

                    emit NodeRemovedAndRefunded(nodeID: nodeRecord.id, amount: nodeRecord.tokensCommitted.balance + nodeRecord.tokensStaked.balance)

                    if nodeRecord.tokensCommitted.balance > 0.0 {
                        /// move their committed tokens back to their unstaked tokens
                        nodeRecord.tokensUnstaked.deposit(from: <-nodeRecord.tokensCommitted.withdraw(amount: nodeRecord.tokensCommitted.balance))
                    }

                    /// Set their request to unstake equal to all their staked tokens
                    /// since they are forced to unstake
                    nodeRecord.tokensRequestedToUnstake = nodeRecord.tokensStaked.balance

                    nodeRecord.initialWeight = 0

                } else {
                    /// Set initial weight of all the committed nodes
                    /// TODO: Figure out how to calculate the initial weight for each node
                    nodeRecord.initialWeight = 100
                }
            }
        }

        /// Called at the end of the epoch to pay rewards to node operators
        /// based on the tokens that they have staked
        pub fun payRewards() {

            let allNodeIDs = FlowIDTableStaking.getNodeIDs()

            let flowTokenMinter = FlowIDTableStaking.account.borrow<&FlowToken.Minter>(from: /storage/flowTokenMinter)
                ?? panic("Could not borrow minter reference")

            // calculate total reward sum for each node type
            // by multiplying the total amount of rewards by the ratio for each node type
            var rewardsForNodeTypes: {UInt8: UFix64} = {}
            rewardsForNodeTypes[UInt8(1)] = FlowIDTableStaking.epochTokenPayout * FlowIDTableStaking.rewardRatios[UInt8(1)]!
            rewardsForNodeTypes[UInt8(2)] = FlowIDTableStaking.epochTokenPayout * FlowIDTableStaking.rewardRatios[UInt8(2)]!
            rewardsForNodeTypes[UInt8(3)] = FlowIDTableStaking.epochTokenPayout * FlowIDTableStaking.rewardRatios[UInt8(3)]!
            rewardsForNodeTypes[UInt8(4)] = FlowIDTableStaking.epochTokenPayout * FlowIDTableStaking.rewardRatios[UInt8(4)]!
            rewardsForNodeTypes[UInt8(5)] = 0.0

            /// iterate through all the nodes
            for nodeID in allNodeIDs {

                let nodeRecord = FlowIDTableStaking.borrowNodeRecord(nodeID)

                if nodeRecord.tokensStaked.balance == 0.0 { continue }

                /// Calculate the amount of tokens that this node operator receives
                let rewardAmount = rewardsForNodeTypes[nodeRecord.role]! * (nodeRecord.tokensStaked.balance / FlowIDTableStaking.totalTokensStakedByNodeType[nodeRecord.role]!)

                /// Mint the tokens to reward the operator
                let tokenReward <- flowTokenMinter.mintTokens(amount: rewardAmount)

                emit RewardsPaid(nodeID: nodeRecord.id, amount: tokenReward.balance)

                // Iterate through all delegators and reward them their share
                // of the rewards for the tokens they have staked for this node
                for delegator in nodeRecord.delegators.keys {
                    let delRecord = nodeRecord.borrowDelegatorRecord(delegator)

                    if delRecord.tokensStaked.balance == 0.0 { continue }

                    let delegatorRewardAmount = (rewardsForNodeTypes[nodeRecord.role]! * (delRecord.tokensStaked.balance / FlowIDTableStaking.totalTokensStakedByNodeType[nodeRecord.role]!))

                    let delegatorReward <- flowTokenMinter.mintTokens(amount: delegatorRewardAmount)

                    // take the node operator's cut
                    tokenReward.deposit(from: <-delegatorReward.withdraw(amount: delegatorReward.balance * FlowIDTableStaking.nodeDelegatingRewardCut))

                    emit DelegatorRewardsPaid(nodeID: nodeRecord.id, delegatorID: delegator, amount: delegatorRewardAmount)

                    if delegatorReward.balance > 0.0 {
                        delRecord.tokensRewarded.deposit(from: <-delegatorReward)
                    } else {
                        destroy delegatorReward
                    }
                }

                /// Deposit the node Rewards into their tokensRewarded bucket
                nodeRecord.tokensRewarded.deposit(from: <-tokenReward)
            }
        }

        /// Called at the end of the epoch to move tokens between buckets
        /// for stakers
        /// Tokens that have been committed are moved to the staked bucket
        /// Tokens that were unstaking during the last epoch are fully unstaked
        /// Unstaking requests are filled by moving those tokens from staked to unstaking
        pub fun moveTokens() {

            let allNodeIDs = FlowIDTableStaking.getNodeIDs()

            for nodeID in allNodeIDs {

                let nodeRecord = FlowIDTableStaking.borrowNodeRecord(nodeID)

                // Update total number of tokens staked by all the nodes of each type
                FlowIDTableStaking.totalTokensStakedByNodeType[nodeRecord.role] = FlowIDTableStaking.totalTokensStakedByNodeType[nodeRecord.role]! + nodeRecord.tokensCommitted.balance

                if nodeRecord.tokensCommitted.balance > 0.0 {
                    emit TokensStaked(nodeID: nodeRecord.id, amount: nodeRecord.tokensCommitted.balance)
                    nodeRecord.tokensStaked.deposit(from: <-nodeRecord.tokensCommitted.withdraw(amount: nodeRecord.tokensCommitted.balance))
                }
                if nodeRecord.tokensUnstaking.balance > 0.0 {
                    nodeRecord.tokensUnstaked.deposit(from: <-nodeRecord.tokensUnstaking.withdraw(amount: nodeRecord.tokensUnstaking.balance))
                }
                if nodeRecord.tokensRequestedToUnstake > 0.0 {
                    emit TokensUnstaking(nodeID: nodeRecord.id, amount: nodeRecord.tokensRequestedToUnstake)
                    nodeRecord.tokensUnstaking.deposit(from: <-nodeRecord.tokensStaked.withdraw(amount: nodeRecord.tokensRequestedToUnstake))
                }

                // move all the delegators' tokens between buckets
                for delegator in nodeRecord.delegators.keys {

                    let delRecord = nodeRecord.borrowDelegatorRecord(delegator)

                    FlowIDTableStaking.totalTokensStakedByNodeType[nodeRecord.role] = FlowIDTableStaking.totalTokensStakedByNodeType[nodeRecord.role]! + delRecord.tokensCommitted.balance

                    // mark their committed tokens as staked
                    if delRecord.tokensCommitted.balance > 0.0 {
                        delRecord.tokensStaked.deposit(from: <-delRecord.tokensCommitted.withdraw(amount: delRecord.tokensCommitted.balance))
                    }

                    if delRecord.tokensUnstaking.balance > 0.0 {
                        delRecord.tokensUnstaked.deposit(from: <-delRecord.tokensUnstaking.withdraw(amount: delRecord.tokensUnstaking.balance))
                    }

                    if delRecord.tokensRequestedToUnstake > 0.0 {
                        delRecord.tokensUnstaking.deposit(from: <-delRecord.tokensStaked.withdraw(amount: delRecord.tokensRequestedToUnstake))
                        emit TokensUnstaking(nodeID: nodeRecord.id, amount: delRecord.tokensRequestedToUnstake)
                    }

                    // subtract their requested tokens from the total staked for their node type
                    FlowIDTableStaking.totalTokensStakedByNodeType[nodeRecord.role] = FlowIDTableStaking.totalTokensStakedByNodeType[nodeRecord.role]! - delRecord.tokensRequestedToUnstake

                    delRecord.tokensRequestedToUnstake = 0.0
                }

                // subtract their requested tokens from the total staked for their node type
                FlowIDTableStaking.totalTokensStakedByNodeType[nodeRecord.role] = FlowIDTableStaking.totalTokensStakedByNodeType[nodeRecord.role]! - nodeRecord.tokensRequestedToUnstake

                // Reset the tokens requested field so it can be used for the next epoch
                nodeRecord.tokensRequestedToUnstake = 0.0
            }
        }

        // Changes the total weekly payout to a new value
        pub fun updateEpochTokenPayout(_ newPayout: UFix64) {
            FlowIDTableStaking.epochTokenPayout = newPayout
        }

        /// Admin calls this to change the percentage
        /// of delegator rewards every node operator takes
        pub fun changeCutPercentage(_ newCutPercentage: UFix64) {
            pre {
                newCutPercentage > 0.0 && newCutPercentage < 1.0:
                    "Cut percentage must be between 0 and 1!"
            }

            FlowIDTableStaking.nodeDelegatingRewardCut = newCutPercentage

            emit NewDelegatorCutPercentage(newCutPercentage: FlowIDTableStaking.nodeDelegatingRewardCut)
        }
    }

    /// Any node can call this function to register a new Node
    /// It returns the resource for nodes that they can store in
    /// their account storage
    pub fun addNodeRecord(id: String, role: UInt8, networkingAddress: String, networkingKey: String, stakingKey: String, tokensCommitted: @FungibleToken.Vault): @NodeStaker {

        let initialBalance = tokensCommitted.balance

        let newNode <- create NodeRecord(id: id, role: role, networkingAddress: networkingAddress, networkingKey: networkingKey, stakingKey: stakingKey, tokensCommitted: <-tokensCommitted)

        // Insert the node to the table
        FlowIDTableStaking.nodes[id] <-! newNode

        // return a new NodeStaker object that the node operator stores in their account
        return <-create NodeStaker(id: id)

    }

    /// Registers a new delegator with a unique ID for the specified node operator
    /// and returns a delegator object to the caller
    /// The node operator would make a public capability for potential delegators
    /// to access this function
    pub fun registerNewDelegator(nodeID: String): @NodeDelegator {

        let nodeRecord = FlowIDTableStaking.borrowNodeRecord(nodeID)

        assert (
            FlowIDTableStaking.getTotalCommittedBalance(nodeID) > FlowIDTableStaking.minimumStakeRequired[nodeRecord.role]!,
            message: "Cannot register a delegator if the node operator is below the minimum stake"
        )

        nodeRecord.delegatorIDCounter = nodeRecord.delegatorIDCounter + UInt32(1)

        nodeRecord.delegators[nodeRecord.delegatorIDCounter] <-! create DelegatorRecord()

        emit NewDelegatorCreated(nodeID: nodeRecord.id, delegatorID: nodeRecord.delegatorIDCounter)

        return <-create NodeDelegator(id: nodeRecord.delegatorIDCounter, nodeID: nodeRecord.id)
    }

    /// borrow a reference to to one of the nodes in the record
    /// This gives the caller access to all the public fields on the
    /// objects and is basically as if the caller owned the object
    /// The only thing they cannot do is destroy it or move it
    /// This will only be used by the other epoch contracts
    access(contract) fun borrowNodeRecord(_ nodeID: String): &NodeRecord {
        pre {
            FlowIDTableStaking.nodes[nodeID] != nil:
                "Specified node ID does not exist in the record"
        }
        return &FlowIDTableStaking.nodes[nodeID] as! &NodeRecord
    }

    /****************** Getter Functions for the staking Info *******************/

    /// Gets an array of the node IDs that are proposed for the next epoch
    /// Nodes that are proposed are nodes that have enough tokens staked + committed
    /// for the next epoch
    pub fun getProposedNodeIDs(): [String] {
        var proposedNodes: [String] = []

        for nodeID in FlowIDTableStaking.getNodeIDs() {
            let delRecord = FlowIDTableStaking.borrowNodeRecord(nodeID)

            if self.getTotalCommittedBalance(nodeID) >= self.minimumStakeRequired[delRecord.role]!  {
                proposedNodes.append(nodeID)
            }
        }

        return proposedNodes
    }

    /// Gets an array of all the nodeIDs that are staked.
    /// Only nodes that are participating in the current epoch
    /// can be staked, so this is an array of all the active
    /// node operators
    pub fun getStakedNodeIDs(): [String] {
        var stakedNodes: [String] = []

        for nodeID in FlowIDTableStaking.getNodeIDs() {
            let nodeRecord = FlowIDTableStaking.borrowNodeRecord(nodeID)

            if nodeRecord.tokensStaked.balance >= self.minimumStakeRequired[nodeRecord.role]!  {
                stakedNodes.append(nodeID)
            }
        }

        return stakedNodes
    }

    /// Gets an array of all the node IDs that have ever applied
    pub fun getNodeIDs(): [String] {
        return FlowIDTableStaking.nodes.keys
    }

    /// Gets the total amount of tokens that have been staked and
    /// committed for a node. The sum from the node operator and all
    /// its delegators
    pub fun getTotalCommittedBalance(_ nodeID: String): UFix64 {
        let nodeRecord = self.borrowNodeRecord(nodeID)

        if (nodeRecord.tokensCommitted.balance + nodeRecord.tokensStaked.balance) < nodeRecord.tokensRequestedToUnstake {
            return 0.0
        } else {
            var sum: UFix64 = 0.0

            sum = nodeRecord.tokensCommitted.balance + nodeRecord.tokensStaked.balance - nodeRecord.tokensRequestedToUnstake

            for delegator in nodeRecord.delegators.keys {
                let delRecord = nodeRecord.borrowDelegatorRecord(delegator)
                sum = sum + delRecord.tokensCommitted.balance + delRecord.tokensStaked.balance - delRecord.tokensRequestedToUnstake
            }

            return sum
        }
    }

    /// Functions to return contract fields

    pub fun getMinimumStakeRequirements(): {UInt8: UFix64} {
        return self.minimumStakeRequired
    }

    pub fun getTotalTokensStakedByNodeType(): {UInt8: UFix64} {
        return self.totalTokensStakedByNodeType
    }

    pub fun getEpochTokenPayout(): UFix64 {
        return self.epochTokenPayout
    }

    pub fun getRewardRatios(): {UInt8: UFix64} {
        return self.rewardRatios
    }

    init() {
        self.nodes <- {}

        self.NodeStakerStoragePath = /storage/flowStaker
        self.NodeStakerPublicPath = /public/flowStaker
        self.StakingAdminStoragePath = /storage/flowStakingAdmin
        self.DelegatorStoragePath = /storage/flowStakingDelegator

        // minimum stakes for each node types
        self.minimumStakeRequired = {UInt8(1): 250000.0, UInt8(2): 500000.0, UInt8(3): 1250000.0, UInt8(4): 135000.0, UInt8(5): 0.0}

        self.totalTokensStakedByNodeType = {UInt8(1): 0.0, UInt8(2): 0.0, UInt8(3): 0.0, UInt8(4): 0.0, UInt8(5): 0.0}

        // 1.25M FLOW paid out in the first week. Decreasing in subsequent weeks
        self.epochTokenPayout = 1250000.0

        // initialize the cut of rewards that node operators take to 3%
        self.nodeDelegatingRewardCut = 0.03

        // The preliminary percentage of rewards that go to each node type every epoch
        // subject to change
        self.rewardRatios = {UInt8(1): 0.168, UInt8(2): 0.518, UInt8(3): 0.078, UInt8(4): 0.236, UInt8(5): 0.0}

        // save the admin object to storage
        self.account.save(<-create Admin(), to: self.StakingAdminStoragePath)
    }
}
`

//nolint:gosimple
const knownContract_ac98da57ce4dd4ef_MessageBoard = `
pub contract MessageBoard {
  // The path to the Admin object in this contract's storage
  pub let adminStoragePath: StoragePath

  pub struct Post {
    pub let timestamp: UFix64
    pub let message: String
    pub let from: Address

    init(timestamp: UFix64, message: String, from: Address) {
      self.timestamp = timestamp
      self.message = message
      self.from = from
    }
  }

  // Records 100 latest messages
  pub var posts: [Post]

  // Emitted when a post is made
  pub event Posted(timestamp: UFix64, message: String, from: Address)

  pub fun post(message: String, from: Address) {
    pre {
      message.length <= 140: "Message too long"
    }

    let post = Post(timestamp: getCurrentBlock().timestamp, message: message, from: from)
    self.posts.append(post)

    // Keeps only the latest 100 messages
    if (self.posts.length > 100) {
      self.posts.removeFirst()
    }

    emit Posted(timestamp: getCurrentBlock().timestamp, message: message, from: from)
  }

  // Check current messages
  pub fun getPosts(): [Post] {
    return self.posts
  }

  pub resource Admin {
      pub fun deletePost(index: UInt64) {
          MessageBoard.posts.remove(at: index)
      }
  }

  init() {
    self.adminStoragePath = /storage/admin
    self.posts = []
    self.account.save(<-create Admin(), to: self.adminStoragePath)
  }
}
`

//nolint:gosimple
const knownContract_ab273f724a1625df_MultiMessageBoard = `
pub contract MultiMessageBoard {
  // The path to the Admin object in this contract's storage
  pub let AdminStoragePath: StoragePath

  // Maximum allowed message length
  pub var maxMessageLength: Int

  // Maximum allowed messages on a board
  pub var maxMessageCount: Int

  pub struct Post {
    pub let timestamp: UFix64
    pub let message: String
    pub let from: Address

    init(timestamp: UFix64, message: String, from: Address) {
      self.timestamp = timestamp
      self.message = message
      self.from = from
    }
  }

  // Records "maxMessageCount" latest messages
  pub var posts: [[Post]]

  // Emitted when a post is made
  pub event Posted(timestamp: UFix64, message: String, from: Address, boardID: Int)

  pub fun post(message: String, sender: AuthAccount, boardID: Int) {
    pre {
      message.length <= self.maxMessageLength: "Message too long"
      boardID < self.posts.length: "Invalid board ID"
    }

    let from = sender.address
    let post = Post(timestamp: getCurrentBlock().timestamp, message: message, from: from)
    self.posts[boardID].append(post)

    // Keeps only the latest "maxMessageCount" messages on the board
    if (self.posts[boardID].length > self.maxMessageCount) {
      self.posts[boardID].removeFirst()
    }

    emit Posted(timestamp: getCurrentBlock().timestamp, message: message, from: from, boardID: boardID)
  }

  // Check current messages
  pub fun getPosts(boardID: Int): [Post] {
    pre {
      boardID < self.posts.length: "Invalid board ID"
    }

    return self.posts[boardID]
  }

  pub resource Admin {
    // Removes the specified post
    pub fun deletePost(boardID: Int, index: Int) {
      MultiMessageBoard.posts[boardID].remove(at: index)
    }

    // Create a message board
    pub fun createBoard() {
      MultiMessageBoard.posts.append([])
    }

    // Remove a message board
    pub fun removeBoard(boardID: Int) {
      MultiMessageBoard.posts.remove(at: boardID)
    }

    pub fun updateMaxMessageLength(maxMessageLength: Int) {
      MultiMessageBoard.maxMessageLength = maxMessageLength
    }

    pub fun updateMaxMessageCount(maxMessageCount: Int) {
      MultiMessageBoard.maxMessageCount = maxMessageCount
    }
  }

  init() {
    self.AdminStoragePath = /storage/multiMessageBoardAdmin
    self.posts = []
    self.maxMessageLength = 140
    self.maxMessageCount = 100
    
    // Store an Admin object in this contract's account storage
    self.account.save(<-create Admin(), to: self.AdminStoragePath)
  }
}
`
