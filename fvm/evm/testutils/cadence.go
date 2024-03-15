package testutils

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/onflow/atree"
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/sema"
	cadenceStdlib "github.com/onflow/cadence/runtime/stdlib"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
)

// TODO: replace with Cadence runtime testing utils once available https://github.com/onflow/cadence/pull/2800

func SingleIdentifierLocationResolver(t testing.TB) func(
	identifiers []runtime.Identifier,
	location runtime.Location,
) (
	[]runtime.ResolvedLocation,
	error,
) {
	return func(identifiers []runtime.Identifier, location runtime.Location) ([]runtime.ResolvedLocation, error) {
		require.Len(t, identifiers, 1)
		require.IsType(t, common.AddressLocation{}, location)

		return []runtime.ResolvedLocation{
			{
				Location: common.AddressLocation{
					Address: location.(common.AddressLocation).Address,
					Name:    identifiers[0].Identifier,
				},
				Identifiers: identifiers,
			},
		}, nil
	}
}

func newLocationGenerator[T ~[32]byte]() func() T {
	var count uint64
	return func() T {
		t := T{}
		newCount := atomic.AddUint64(&count, 1)
		binary.LittleEndian.PutUint64(t[:], newCount)
		return t
	}
}

func NewTransactionLocationGenerator() func() common.TransactionLocation {
	return newLocationGenerator[common.TransactionLocation]()
}

func NewScriptLocationGenerator() func() common.ScriptLocation {
	return newLocationGenerator[common.ScriptLocation]()
}

func EncodeArgs(argValues []cadence.Value) [][]byte {
	args := make([][]byte, len(argValues))
	for i, arg := range argValues {
		var err error
		args[i], err = json.Encode(arg)
		if err != nil {
			panic(fmt.Errorf("broken test: invalid argument: %w", err))
		}
	}
	return args
}

type TestLedger struct {
	StoredValues           map[string][]byte
	OnValueExists          func(owner, key []byte) (exists bool, err error)
	OnGetValue             func(owner, key []byte) (value []byte, err error)
	OnSetValue             func(owner, key, value []byte) (err error)
	OnAllocateStorageIndex func(owner []byte) (atree.SlabIndex, error)
}

var _ atree.Ledger = TestLedger{}

func (s TestLedger) GetValue(owner, key []byte) (value []byte, err error) {
	return s.OnGetValue(owner, key)
}

func (s TestLedger) SetValue(owner, key, value []byte) (err error) {
	return s.OnSetValue(owner, key, value)
}

func (s TestLedger) ValueExists(owner, key []byte) (exists bool, err error) {
	return s.OnValueExists(owner, key)
}

func (s TestLedger) AllocateSlabIndex(owner []byte) (atree.SlabIndex, error) {
	return s.OnAllocateStorageIndex(owner)
}

func (s TestLedger) Dump() {
	// Only used for testing/debugging purposes
	for key, data := range s.StoredValues { //nolint:maprange
		fmt.Printf("%s:\n", strconv.Quote(key))
		fmt.Printf("%s\n", hex.Dump(data))
		println()
	}
}

func NewTestLedger(
	onRead func(owner, key, value []byte),
	onWrite func(owner, key, value []byte),
) TestLedger {

	storageKey := func(owner, key string) string {
		return strings.Join([]string{owner, key}, "|")
	}

	storedValues := map[string][]byte{}

	storageIndices := map[string]uint64{}

	return TestLedger{
		StoredValues: storedValues,
		OnValueExists: func(owner, key []byte) (bool, error) {
			value := storedValues[storageKey(string(owner), string(key))]
			return len(value) > 0, nil
		},
		OnGetValue: func(owner, key []byte) (value []byte, err error) {
			value = storedValues[storageKey(string(owner), string(key))]
			if onRead != nil {
				onRead(owner, key, value)
			}
			return value, nil
		},
		OnSetValue: func(owner, key, value []byte) (err error) {
			storedValues[storageKey(string(owner), string(key))] = value
			if onWrite != nil {
				onWrite(owner, key, value)
			}
			return nil
		},
		OnAllocateStorageIndex: func(owner []byte) (result atree.SlabIndex, err error) {
			index := storageIndices[string(owner)] + 1
			storageIndices[string(owner)] = index
			binary.BigEndian.PutUint64(result[:], index)
			return
		},
	}
}

type TestRuntimeInterface struct {
	Storage atree.Ledger

	OnResolveLocation func(
		identifiers []runtime.Identifier,
		location runtime.Location,
	) (
		[]runtime.ResolvedLocation,
		error,
	)
	OnGetCode          func(_ runtime.Location) ([]byte, error)
	OnGetAndSetProgram func(
		location runtime.Location,
		load func() (*interpreter.Program, error),
	) (*interpreter.Program, error)
	OnSetInterpreterSharedState func(state *interpreter.SharedState)
	OnGetInterpreterSharedState func() *interpreter.SharedState
	OnCreateAccount             func(payer runtime.Address) (address runtime.Address, err error)
	OnAddEncodedAccountKey      func(address runtime.Address, publicKey []byte) error
	OnRemoveEncodedAccountKey   func(address runtime.Address, index int) (publicKey []byte, err error)
	OnAddAccountKey             func(
		address runtime.Address,
		publicKey *cadenceStdlib.PublicKey,
		hashAlgo runtime.HashAlgorithm,
		weight int,
	) (*cadenceStdlib.AccountKey, error)
	OnGetAccountKey             func(address runtime.Address, index int) (*cadenceStdlib.AccountKey, error)
	OnRemoveAccountKey          func(address runtime.Address, index int) (*cadenceStdlib.AccountKey, error)
	OnAccountKeysCount          func(address runtime.Address) (uint64, error)
	OnUpdateAccountContractCode func(location common.AddressLocation, code []byte) error
	OnGetAccountContractCode    func(location common.AddressLocation) (code []byte, err error)
	OnRemoveAccountContractCode func(location common.AddressLocation) (err error)
	OnGetSigningAccounts        func() ([]runtime.Address, error)
	OnProgramLog                func(string)
	OnEmitEvent                 func(cadence.Event) error
	OnResourceOwnerChanged      func(
		interpreter *interpreter.Interpreter,
		resource *interpreter.CompositeValue,
		oldAddress common.Address,
		newAddress common.Address,
	)
	OnGenerateUUID       func() (uint64, error)
	OnMeterComputation   func(compKind common.ComputationKind, intensity uint) error
	OnDecodeArgument     func(b []byte, t cadence.Type) (cadence.Value, error)
	OnProgramParsed      func(location runtime.Location, duration time.Duration)
	OnProgramChecked     func(location runtime.Location, duration time.Duration)
	OnProgramInterpreted func(location runtime.Location, duration time.Duration)
	OnReadRandom         func([]byte) error
	OnVerifySignature    func(
		signature []byte,
		tag string,
		signedData []byte,
		publicKey []byte,
		signatureAlgorithm runtime.SignatureAlgorithm,
		hashAlgorithm runtime.HashAlgorithm,
	) (bool, error)
	OnHash func(
		data []byte,
		tag string,
		hashAlgorithm runtime.HashAlgorithm,
	) ([]byte, error)
	OnSetCadenceValue            func(owner runtime.Address, key string, value cadence.Value) (err error)
	OnGetAccountBalance          func(_ runtime.Address) (uint64, error)
	OnGetAccountAvailableBalance func(_ runtime.Address) (uint64, error)
	OnGetStorageUsed             func(_ runtime.Address) (uint64, error)
	OnGetStorageCapacity         func(_ runtime.Address) (uint64, error)
	Programs                     map[runtime.Location]*interpreter.Program
	OnImplementationDebugLog     func(message string) error
	OnValidatePublicKey          func(publicKey *cadenceStdlib.PublicKey) error
	OnBLSVerifyPOP               func(pk *cadenceStdlib.PublicKey, s []byte) (bool, error)
	OnBLSAggregateSignatures     func(sigs [][]byte) ([]byte, error)
	OnBLSAggregatePublicKeys     func(keys []*cadenceStdlib.PublicKey) (*cadenceStdlib.PublicKey, error)
	OnGetAccountContractNames    func(address runtime.Address) ([]string, error)
	OnRecordTrace                func(
		operation string,
		location runtime.Location,
		duration time.Duration,
		attrs []attribute.KeyValue,
	)
	OnMeterMemory       func(usage common.MemoryUsage) error
	OnComputationUsed   func() (uint64, error)
	OnMemoryUsed        func() (uint64, error)
	OnInteractionUsed   func() (uint64, error)
	OnGenerateAccountID func(address common.Address) (uint64, error)

	lastUUID            uint64
	accountIDs          map[common.Address]uint64
	updatedContractCode bool
}

// TestRuntimeInterface should implement Interface
var _ runtime.Interface = &TestRuntimeInterface{}

func (i *TestRuntimeInterface) ResolveLocation(
	identifiers []runtime.Identifier,
	location runtime.Location,
) ([]runtime.ResolvedLocation, error) {
	if i.OnResolveLocation == nil {
		return []runtime.ResolvedLocation{
			{
				Location:    location,
				Identifiers: identifiers,
			},
		}, nil
	}
	return i.OnResolveLocation(identifiers, location)
}

func (i *TestRuntimeInterface) GetCode(location runtime.Location) ([]byte, error) {
	if i.OnGetCode == nil {
		return nil, nil
	}
	return i.OnGetCode(location)
}

func (i *TestRuntimeInterface) GetOrLoadProgram(
	location runtime.Location,
	load func() (*interpreter.Program, error),
) (
	program *interpreter.Program,
	err error,
) {
	if i.OnGetAndSetProgram == nil {
		if i.Programs == nil {
			i.Programs = map[runtime.Location]*interpreter.Program{}
		}

		var ok bool
		program, ok = i.Programs[location]
		if ok {
			return
		}

		program, err = load()

		// NOTE: important: still set empty program,
		// even if error occurred

		i.Programs[location] = program

		return
	}

	return i.OnGetAndSetProgram(location, load)
}

func (i *TestRuntimeInterface) SetInterpreterSharedState(state *interpreter.SharedState) {
	if i.OnSetInterpreterSharedState == nil {
		return
	}

	i.OnSetInterpreterSharedState(state)
}

func (i *TestRuntimeInterface) GetInterpreterSharedState() *interpreter.SharedState {
	if i.OnGetInterpreterSharedState == nil {
		return nil
	}

	return i.OnGetInterpreterSharedState()
}

func (i *TestRuntimeInterface) ValueExists(owner, key []byte) (exists bool, err error) {
	return i.Storage.ValueExists(owner, key)
}

func (i *TestRuntimeInterface) GetValue(owner, key []byte) (value []byte, err error) {
	return i.Storage.GetValue(owner, key)
}

func (i *TestRuntimeInterface) SetValue(owner, key, value []byte) (err error) {
	return i.Storage.SetValue(owner, key, value)
}

func (i *TestRuntimeInterface) AllocateSlabIndex(owner []byte) (atree.SlabIndex, error) {
	return i.Storage.AllocateSlabIndex(owner)
}

func (i *TestRuntimeInterface) CreateAccount(payer runtime.Address) (address runtime.Address, err error) {
	if i.OnCreateAccount == nil {
		panic("must specify TestRuntimeInterface.OnCreateAccount")
	}
	return i.OnCreateAccount(payer)
}

func (i *TestRuntimeInterface) AddEncodedAccountKey(address runtime.Address, publicKey []byte) error {
	if i.OnAddEncodedAccountKey == nil {
		panic("must specify TestRuntimeInterface.OnAddEncodedAccountKey")
	}
	return i.OnAddEncodedAccountKey(address, publicKey)
}

func (i *TestRuntimeInterface) RevokeEncodedAccountKey(address runtime.Address, index int) ([]byte, error) {
	if i.OnRemoveEncodedAccountKey == nil {
		panic("must specify TestRuntimeInterface.OnRemoveEncodedAccountKey")
	}
	return i.OnRemoveEncodedAccountKey(address, index)
}

func (i *TestRuntimeInterface) AddAccountKey(
	address runtime.Address,
	publicKey *cadenceStdlib.PublicKey,
	hashAlgo runtime.HashAlgorithm,
	weight int,
) (*cadenceStdlib.AccountKey, error) {
	if i.OnAddAccountKey == nil {
		panic("must specify TestRuntimeInterface.OnAddAccountKey")
	}
	return i.OnAddAccountKey(address, publicKey, hashAlgo, weight)
}

func (i *TestRuntimeInterface) GetAccountKey(address runtime.Address, index int) (*cadenceStdlib.AccountKey, error) {
	if i.OnGetAccountKey == nil {
		panic("must specify TestRuntimeInterface.OnGetAccountKey")
	}
	return i.OnGetAccountKey(address, index)
}

func (i *TestRuntimeInterface) AccountKeysCount(address runtime.Address) (uint64, error) {
	if i.OnAccountKeysCount == nil {
		panic("must specify TestRuntimeInterface.OnAccountKeysCount")
	}
	return i.OnAccountKeysCount(address)
}

func (i *TestRuntimeInterface) RevokeAccountKey(address runtime.Address, index int) (*cadenceStdlib.AccountKey, error) {
	if i.OnRemoveAccountKey == nil {
		panic("must specify TestRuntimeInterface.OnRemoveAccountKey")
	}
	return i.OnRemoveAccountKey(address, index)
}

func (i *TestRuntimeInterface) UpdateAccountContractCode(location common.AddressLocation, code []byte) (err error) {
	if i.OnUpdateAccountContractCode == nil {
		panic("must specify TestRuntimeInterface.OnUpdateAccountContractCode")
	}

	err = i.OnUpdateAccountContractCode(location, code)
	if err != nil {
		return err
	}

	i.updatedContractCode = true

	return nil
}

func (i *TestRuntimeInterface) GetAccountContractCode(location common.AddressLocation) (code []byte, err error) {
	if i.OnGetAccountContractCode == nil {
		panic("must specify TestRuntimeInterface.OnGetAccountContractCode")
	}
	return i.OnGetAccountContractCode(location)
}

func (i *TestRuntimeInterface) RemoveAccountContractCode(location common.AddressLocation) (err error) {
	if i.OnRemoveAccountContractCode == nil {
		panic("must specify TestRuntimeInterface.OnRemoveAccountContractCode")
	}
	return i.OnRemoveAccountContractCode(location)
}

func (i *TestRuntimeInterface) GetSigningAccounts() ([]runtime.Address, error) {
	if i.OnGetSigningAccounts == nil {
		return nil, nil
	}
	return i.OnGetSigningAccounts()
}

func (i *TestRuntimeInterface) ProgramLog(message string) error {
	i.OnProgramLog(message)
	return nil
}

func (i *TestRuntimeInterface) EmitEvent(event cadence.Event) error {
	return i.OnEmitEvent(event)
}

func (i *TestRuntimeInterface) ResourceOwnerChanged(
	interpreter *interpreter.Interpreter,
	resource *interpreter.CompositeValue,
	oldOwner common.Address,
	newOwner common.Address,
) {
	if i.OnResourceOwnerChanged != nil {
		i.OnResourceOwnerChanged(
			interpreter,
			resource,
			oldOwner,
			newOwner,
		)
	}
}

func (i *TestRuntimeInterface) GenerateUUID() (uint64, error) {
	if i.OnGenerateUUID == nil {
		i.lastUUID++
		return i.lastUUID, nil
	}
	return i.OnGenerateUUID()
}

func (i *TestRuntimeInterface) MeterComputation(compKind common.ComputationKind, intensity uint) error {
	if i.OnMeterComputation == nil {
		return nil
	}
	return i.OnMeterComputation(compKind, intensity)
}

func (i *TestRuntimeInterface) DecodeArgument(b []byte, t cadence.Type) (cadence.Value, error) {
	if i.OnDecodeArgument == nil {
		panic("must specify TestRuntimeInterface.OnDecodeArgument")
	}
	return i.OnDecodeArgument(b, t)
}

func (i *TestRuntimeInterface) ProgramParsed(location runtime.Location, duration time.Duration) {
	if i.OnProgramParsed == nil {
		return
	}
	i.OnProgramParsed(location, duration)
}

func (i *TestRuntimeInterface) ProgramChecked(location runtime.Location, duration time.Duration) {
	if i.OnProgramChecked == nil {
		return
	}
	i.OnProgramChecked(location, duration)
}

func (i *TestRuntimeInterface) ProgramInterpreted(location runtime.Location, duration time.Duration) {
	if i.OnProgramInterpreted == nil {
		return
	}
	i.OnProgramInterpreted(location, duration)
}

func (i *TestRuntimeInterface) GetCurrentBlockHeight() (uint64, error) {
	return 1, nil
}

func (i *TestRuntimeInterface) GetBlockAtHeight(height uint64) (block cadenceStdlib.Block, exists bool, err error) {

	buf := new(bytes.Buffer)
	err = binary.Write(buf, binary.BigEndian, height)
	if err != nil {
		panic(err)
	}

	encoded := buf.Bytes()
	var hash cadenceStdlib.BlockHash
	copy(hash[sema.BlockTypeIdFieldType.Size-int64(len(encoded)):], encoded)

	block = cadenceStdlib.Block{
		Height:    height,
		View:      height,
		Hash:      hash,
		Timestamp: time.Unix(int64(height), 0).UnixNano(),
	}
	return block, true, nil
}

func (i *TestRuntimeInterface) ReadRandom(buffer []byte) error {
	if i.OnReadRandom == nil {
		return nil
	}
	return i.OnReadRandom(buffer)
}

func (i *TestRuntimeInterface) VerifySignature(
	signature []byte,
	tag string,
	signedData []byte,
	publicKey []byte,
	signatureAlgorithm runtime.SignatureAlgorithm,
	hashAlgorithm runtime.HashAlgorithm,
) (bool, error) {
	if i.OnVerifySignature == nil {
		return false, nil
	}
	return i.OnVerifySignature(
		signature,
		tag,
		signedData,
		publicKey,
		signatureAlgorithm,
		hashAlgorithm,
	)
}

func (i *TestRuntimeInterface) Hash(data []byte, tag string, hashAlgorithm runtime.HashAlgorithm) ([]byte, error) {
	if i.OnHash == nil {
		return nil, nil
	}
	return i.OnHash(data, tag, hashAlgorithm)
}

func (i *TestRuntimeInterface) SetCadenceValue(owner common.Address, key string, value cadence.Value) (err error) {
	if i.OnSetCadenceValue == nil {
		panic("must specify TestRuntimeInterface.OnSetCadenceValue")
	}
	return i.OnSetCadenceValue(owner, key, value)
}

func (i *TestRuntimeInterface) GetAccountBalance(address runtime.Address) (uint64, error) {
	if i.OnGetAccountBalance == nil {
		panic("must specify TestRuntimeInterface.OnGetAccountBalance")
	}
	return i.OnGetAccountBalance(address)
}

func (i *TestRuntimeInterface) GetAccountAvailableBalance(address runtime.Address) (uint64, error) {
	if i.OnGetAccountAvailableBalance == nil {
		panic("must specify TestRuntimeInterface.OnGetAccountAvailableBalance")
	}
	return i.OnGetAccountAvailableBalance(address)
}

func (i *TestRuntimeInterface) GetStorageUsed(address runtime.Address) (uint64, error) {
	if i.OnGetStorageUsed == nil {
		panic("must specify TestRuntimeInterface.OnGetStorageUsed")
	}
	return i.OnGetStorageUsed(address)
}

func (i *TestRuntimeInterface) GetStorageCapacity(address runtime.Address) (uint64, error) {
	if i.OnGetStorageCapacity == nil {
		panic("must specify TestRuntimeInterface.OnGetStorageCapacity")
	}
	return i.OnGetStorageCapacity(address)
}

func (i *TestRuntimeInterface) ImplementationDebugLog(message string) error {
	if i.OnImplementationDebugLog == nil {
		return nil
	}
	return i.OnImplementationDebugLog(message)
}

func (i *TestRuntimeInterface) ValidatePublicKey(key *cadenceStdlib.PublicKey) error {
	if i.OnValidatePublicKey == nil {
		return errors.New("mock defaults to public key validation failure")
	}

	return i.OnValidatePublicKey(key)
}

func (i *TestRuntimeInterface) BLSVerifyPOP(key *cadenceStdlib.PublicKey, s []byte) (bool, error) {
	if i.OnBLSVerifyPOP == nil {
		return false, nil
	}

	return i.OnBLSVerifyPOP(key, s)
}

func (i *TestRuntimeInterface) BLSAggregateSignatures(sigs [][]byte) ([]byte, error) {
	if i.OnBLSAggregateSignatures == nil {
		return []byte{}, nil
	}

	return i.OnBLSAggregateSignatures(sigs)
}

func (i *TestRuntimeInterface) BLSAggregatePublicKeys(keys []*cadenceStdlib.PublicKey) (*cadenceStdlib.PublicKey, error) {
	if i.OnBLSAggregatePublicKeys == nil {
		return nil, nil
	}

	return i.OnBLSAggregatePublicKeys(keys)
}

func (i *TestRuntimeInterface) GetAccountContractNames(address runtime.Address) ([]string, error) {
	if i.OnGetAccountContractNames == nil {
		return []string{}, nil
	}

	return i.OnGetAccountContractNames(address)
}

func (i *TestRuntimeInterface) GenerateAccountID(address common.Address) (uint64, error) {
	if i.OnGenerateAccountID == nil {
		if i.accountIDs == nil {
			i.accountIDs = map[common.Address]uint64{}
		}
		i.accountIDs[address]++
		return i.accountIDs[address], nil
	}

	return i.OnGenerateAccountID(address)
}

func (i *TestRuntimeInterface) RecordTrace(
	operation string,
	location runtime.Location,
	duration time.Duration,
	attrs []attribute.KeyValue,
) {
	if i.OnRecordTrace == nil {
		return
	}
	i.OnRecordTrace(operation, location, duration, attrs)
}

func (i *TestRuntimeInterface) MeterMemory(usage common.MemoryUsage) error {
	if i.OnMeterMemory == nil {
		return nil
	}

	return i.OnMeterMemory(usage)
}

func (i *TestRuntimeInterface) ComputationUsed() (uint64, error) {
	if i.OnComputationUsed == nil {
		return 0, nil
	}

	return i.OnComputationUsed()
}

func (i *TestRuntimeInterface) MemoryUsed() (uint64, error) {
	if i.OnMemoryUsed == nil {
		return 0, nil
	}

	return i.OnMemoryUsed()
}

func (i *TestRuntimeInterface) InteractionUsed() (uint64, error) {
	if i.OnInteractionUsed == nil {
		return 0, nil
	}

	return i.OnInteractionUsed()
}
