package evm_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"

	"github.com/onflow/flow-go/fvm/evm"
	"github.com/onflow/flow-go/fvm/evm/stdlib/emulator"
	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/model/flow"
)

func encodeArgs(argValues []cadence.Value) [][]byte {
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

var evmAddressBytesCadenceType = cadence.NewConstantSizedArrayType(20, cadence.TheUInt8Type)

var evmAddressCadenceType = cadence.NewStructType(
	nil,
	emulator.Flex_FlexAddressType.QualifiedIdentifier(),
	[]cadence.Field{
		{
			Identifier: emulator.Flex_FlexAddressTypeBytesFieldName,
			Type:       evmAddressBytesCadenceType,
		},
	},
	nil,
)

func TestFlexAddressConstructionAndReturn(t *testing.T) {

	t.Parallel()
	testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
		testutils.RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {

			env := runtime.NewBaseInterpreterEnvironment(runtime.Config{})
			evm.SetupEnvironment(flow.Emulator, backend, env)

			inter := runtime.NewInterpreterRuntime(runtime.Config{})

			script := []byte(`
			pub fun main(_ bytes: [UInt8; 20]): Flex.FlexAddress {
				return Flex.FlexAddress(bytes: bytes)
				}
			`)

			runtimeInterface := &testRuntimeInterface{
				storage: backend,
				decodeArgument: func(b []byte, t cadence.Type) (cadence.Value, error) {
					return json.Decode(nil, b)
				},
			}

			addressBytesArray := cadence.NewArray([]cadence.Value{
				cadence.UInt8(1), cadence.UInt8(1),
				cadence.UInt8(2), cadence.UInt8(2),
				cadence.UInt8(3), cadence.UInt8(3),
				cadence.UInt8(4), cadence.UInt8(4),
				cadence.UInt8(5), cadence.UInt8(5),
				cadence.UInt8(6), cadence.UInt8(6),
				cadence.UInt8(7), cadence.UInt8(7),
				cadence.UInt8(8), cadence.UInt8(8),
				cadence.UInt8(9), cadence.UInt8(9),
				cadence.UInt8(10), cadence.UInt8(10),
			}).WithType(evmAddressBytesCadenceType)

			result, err := inter.ExecuteScript(
				runtime.Script{
					Source: script,
					Arguments: encodeArgs([]cadence.Value{
						addressBytesArray,
					}),
				},
				runtime.Context{
					Interface:   runtimeInterface,
					Environment: env,
					Location:    common.ScriptLocation{},
				},
			)
			require.NoError(t, err)

			assert.Equal(t,
				cadence.Struct{
					StructType: evmAddressCadenceType,
					Fields: []cadence.Value{
						addressBytesArray,
					},
				},
				result,
			)
		})
	})
}

func TestEVMRun(t *testing.T) {

	t.Parallel()
	testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
		testutils.RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {
			tc := testutils.GetStorageTestContract(t)
			testutils.RunWithDeployedContract(t, tc, backend, rootAddr, func(testContract *testutils.TestContract) {
				testutils.RunWithEOATestAccount(t, backend, rootAddr, func(testAccount *testutils.EOATestAccount) {
					num := int64(12)

					interEnv := runtime.NewBaseInterpreterEnvironment(runtime.Config{})

					evm.SetupEnvironment(flow.Emulator, backend, interEnv)

					inter := runtime.NewInterpreterRuntime(runtime.Config{})

					script := []byte(`
						pub fun main(tx: [UInt8], coinbaseBytes: [UInt8; 20]): Bool {
							let coinbase = Flex.FlexAddress(bytes: coinbaseBytes)
							return Flex.run(tx: tx, coinbase: coinbase)
						}
					`)

					gasLimit := uint64(100_000)

					txBytes := testAccount.PrepareSignAndEncodeTx(t,
						testContract.DeployedAt.ToCommon(),
						testContract.MakeCallData(t, "store", big.NewInt(num)),
						big.NewInt(0),
						gasLimit,
						big.NewInt(1),
					)
					tx := cadence.NewArray(
						testutils.ConvertToCadence(txBytes),
					).WithType(evmAddressBytesCadenceType)

					coinbase := cadence.NewArray(
						testutils.ConvertToCadence(testAccount.Address().Bytes()),
					).WithType(evmAddressBytesCadenceType)

					runtimeInterface := &testRuntimeInterface{
						storage: backend,
						decodeArgument: func(b []byte, t cadence.Type) (cadence.Value, error) {
							return json.Decode(nil, b)
						},
					}

					result, err := inter.ExecuteScript(
						runtime.Script{
							Source:    script,
							Arguments: encodeArgs([]cadence.Value{tx, coinbase}),
						},
						runtime.Context{
							Interface:   runtimeInterface,
							Environment: interEnv,
							Location:    common.ScriptLocation{},
						},
					)
					require.NoError(t, err)
					assert.Equal(t, cadence.Bool(true), result)
				})
			})
		})
	})
}

// TODO: replace with Cadence runtime testing utils once available https://github.com/onflow/cadence/pull/2800

type testRuntimeInterface struct {
	resolveLocation  func(identifiers []runtime.Identifier, location runtime.Location) ([]runtime.ResolvedLocation, error)
	getCode          func(_ runtime.Location) ([]byte, error)
	getAndSetProgram func(
		location runtime.Location,
		load func() (*interpreter.Program, error),
	) (*interpreter.Program, error)
	setInterpreterSharedState func(state *interpreter.SharedState)
	getInterpreterSharedState func() *interpreter.SharedState
	storage                   atree.Ledger
	createAccount             func(payer runtime.Address) (address runtime.Address, err error)
	addEncodedAccountKey      func(address runtime.Address, publicKey []byte) error
	removeEncodedAccountKey   func(address runtime.Address, index int) (publicKey []byte, err error)
	addAccountKey             func(
		address runtime.Address,
		publicKey *cadenceStdlib.PublicKey,
		hashAlgo runtime.HashAlgorithm,
		weight int,
	) (*cadenceStdlib.AccountKey, error)
	getAccountKey             func(address runtime.Address, index int) (*cadenceStdlib.AccountKey, error)
	removeAccountKey          func(address runtime.Address, index int) (*cadenceStdlib.AccountKey, error)
	accountKeysCount          func(address runtime.Address) (uint64, error)
	updateAccountContractCode func(location common.AddressLocation, code []byte) error
	getAccountContractCode    func(location common.AddressLocation) (code []byte, err error)
	removeAccountContractCode func(location common.AddressLocation) (err error)
	getSigningAccounts        func() ([]runtime.Address, error)
	log                       func(string)
	emitEvent                 func(cadence.Event) error
	resourceOwnerChanged      func(
		interpreter *interpreter.Interpreter,
		resource *interpreter.CompositeValue,
		oldAddress common.Address,
		newAddress common.Address,
	)
	generateUUID       func() (uint64, error)
	meterComputation   func(compKind common.ComputationKind, intensity uint) error
	decodeArgument     func(b []byte, t cadence.Type) (cadence.Value, error)
	programParsed      func(location runtime.Location, duration time.Duration)
	programChecked     func(location runtime.Location, duration time.Duration)
	programInterpreted func(location runtime.Location, duration time.Duration)
	readRandom         func([]byte) error
	verifySignature    func(
		signature []byte,
		tag string,
		signedData []byte,
		publicKey []byte,
		signatureAlgorithm runtime.SignatureAlgorithm,
		hashAlgorithm runtime.HashAlgorithm,
	) (bool, error)
	hash                       func(data []byte, tag string, hashAlgorithm runtime.HashAlgorithm) ([]byte, error)
	setCadenceValue            func(owner runtime.Address, key string, value cadence.Value) (err error)
	getAccountBalance          func(_ runtime.Address) (uint64, error)
	getAccountAvailableBalance func(_ runtime.Address) (uint64, error)
	getStorageUsed             func(_ runtime.Address) (uint64, error)
	getStorageCapacity         func(_ runtime.Address) (uint64, error)
	programs                   map[runtime.Location]*interpreter.Program
	implementationDebugLog     func(message string) error
	validatePublicKey          func(publicKey *cadenceStdlib.PublicKey) error
	bLSVerifyPOP               func(pk *cadenceStdlib.PublicKey, s []byte) (bool, error)
	blsAggregateSignatures     func(sigs [][]byte) ([]byte, error)
	blsAggregatePublicKeys     func(keys []*cadenceStdlib.PublicKey) (*cadenceStdlib.PublicKey, error)
	getAccountContractNames    func(address runtime.Address) ([]string, error)
	recordTrace                func(operation string, location runtime.Location, duration time.Duration, attrs []attribute.KeyValue)
	meterMemory                func(usage common.MemoryUsage) error
	computationUsed            func() (uint64, error)
	memoryUsed                 func() (uint64, error)
	interactionUsed            func() (uint64, error)
	updatedContractCode        bool
	generateAccountID          func(address common.Address) (uint64, error)
	unsafeRandom               func() (uint64, error)
}

// testRuntimeInterface should implement cadence runtime Interface
var _ runtime.Interface = &testRuntimeInterface{}

func (i *testRuntimeInterface) GenerateAccountID(address common.Address) (uint64, error) {
	if i.generateAccountID == nil {
		return 0, nil
	}

	return i.generateAccountID(address)
}

func (i *testRuntimeInterface) ResolveLocation(identifiers []runtime.Identifier, location runtime.Location) ([]runtime.ResolvedLocation, error) {
	if i.resolveLocation == nil {
		return []runtime.ResolvedLocation{
			{
				Location:    location,
				Identifiers: identifiers,
			},
		}, nil
	}
	return i.resolveLocation(identifiers, location)
}

func (i *testRuntimeInterface) GetCode(location runtime.Location) ([]byte, error) {
	if i.getCode == nil {
		return nil, nil
	}
	return i.getCode(location)
}

func (i *testRuntimeInterface) GetOrLoadProgram(
	location runtime.Location,
	load func() (*interpreter.Program, error),
) (
	program *interpreter.Program,
	err error,
) {
	if i.getAndSetProgram == nil {
		if i.programs == nil {
			i.programs = map[runtime.Location]*interpreter.Program{}
		}

		var ok bool
		program, ok = i.programs[location]
		if ok {
			return
		}

		program, err = load()

		// NOTE: important: still set empty program,
		// even if error occurred

		i.programs[location] = program

		return
	}

	return i.getAndSetProgram(location, load)
}

func (i *testRuntimeInterface) SetInterpreterSharedState(state *interpreter.SharedState) {
	if i.setInterpreterSharedState == nil {
		return
	}

	i.setInterpreterSharedState(state)
}

func (i *testRuntimeInterface) GetInterpreterSharedState() *interpreter.SharedState {
	if i.getInterpreterSharedState == nil {
		return nil
	}

	return i.getInterpreterSharedState()
}

func (i *testRuntimeInterface) ValueExists(owner, key []byte) (exists bool, err error) {
	if i.storage == nil {
		panic("must specify testRuntimeInterface.storage.valueExists")
	}
	return i.storage.ValueExists(owner, key)
}

func (i *testRuntimeInterface) GetValue(owner, key []byte) (value []byte, err error) {
	if i.storage == nil {
		panic("must specify testRuntimeInterface.storage.getValue")
	}
	return i.storage.GetValue(owner, key)
}

func (i *testRuntimeInterface) SetValue(owner, key, value []byte) (err error) {
	if i.storage == nil {
		panic("must specify testRuntimeInterface.storage.setValue")
	}
	return i.storage.SetValue(owner, key, value)
}

func (i *testRuntimeInterface) AllocateStorageIndex(owner []byte) (atree.StorageIndex, error) {
	if i.storage == nil {
		panic("must specify testRuntimeInterface.storage.allocateStorageIndex")
	}
	return i.storage.AllocateStorageIndex(owner)
}

func (i *testRuntimeInterface) CreateAccount(payer runtime.Address) (address runtime.Address, err error) {
	if i.createAccount == nil {
		panic("must specify testRuntimeInterface.createAccount")
	}
	return i.createAccount(payer)
}

func (i *testRuntimeInterface) AddEncodedAccountKey(address runtime.Address, publicKey []byte) error {
	if i.addEncodedAccountKey == nil {
		panic("must specify testRuntimeInterface.addEncodedAccountKey")
	}
	return i.addEncodedAccountKey(address, publicKey)
}

func (i *testRuntimeInterface) RevokeEncodedAccountKey(address runtime.Address, index int) ([]byte, error) {
	if i.removeEncodedAccountKey == nil {
		panic("must specify testRuntimeInterface.removeEncodedAccountKey")
	}
	return i.removeEncodedAccountKey(address, index)
}

func (i *testRuntimeInterface) AddAccountKey(
	address runtime.Address,
	publicKey *cadenceStdlib.PublicKey,
	hashAlgo runtime.HashAlgorithm,
	weight int,
) (*cadenceStdlib.AccountKey, error) {
	if i.addAccountKey == nil {
		panic("must specify testRuntimeInterface.addAccountKey")
	}
	return i.addAccountKey(address, publicKey, hashAlgo, weight)
}

func (i *testRuntimeInterface) GetAccountKey(address runtime.Address, index int) (*cadenceStdlib.AccountKey, error) {
	if i.getAccountKey == nil {
		panic("must specify testRuntimeInterface.getAccountKey")
	}
	return i.getAccountKey(address, index)
}

func (i *testRuntimeInterface) AccountKeysCount(address runtime.Address) (uint64, error) {
	if i.accountKeysCount == nil {
		panic("must specify testRuntimeInterface.accountKeysCount")
	}
	return i.accountKeysCount(address)
}

func (i *testRuntimeInterface) RevokeAccountKey(address runtime.Address, index int) (*cadenceStdlib.AccountKey, error) {
	if i.removeAccountKey == nil {
		panic("must specify testRuntimeInterface.removeAccountKey")
	}
	return i.removeAccountKey(address, index)
}

func (i *testRuntimeInterface) UpdateAccountContractCode(location common.AddressLocation, code []byte) (err error) {
	if i.updateAccountContractCode == nil {
		panic("must specify testRuntimeInterface.updateAccountContractCode")
	}

	err = i.updateAccountContractCode(location, code)
	if err != nil {
		return err
	}

	i.updatedContractCode = true

	return nil
}

func (i *testRuntimeInterface) GetAccountContractCode(location common.AddressLocation) (code []byte, err error) {
	if i.getAccountContractCode == nil {
		panic("must specify testRuntimeInterface.getAccountContractCode")
	}
	return i.getAccountContractCode(location)
}

func (i *testRuntimeInterface) RemoveAccountContractCode(location common.AddressLocation) (err error) {
	if i.removeAccountContractCode == nil {
		panic("must specify testRuntimeInterface.removeAccountContractCode")
	}
	return i.removeAccountContractCode(location)
}

func (i *testRuntimeInterface) GetSigningAccounts() ([]runtime.Address, error) {
	if i.getSigningAccounts == nil {
		return nil, nil
	}
	return i.getSigningAccounts()
}

func (i *testRuntimeInterface) ProgramLog(message string) error {
	i.log(message)
	return nil
}

func (i *testRuntimeInterface) EmitEvent(event cadence.Event) error {
	return i.emitEvent(event)
}

func (i *testRuntimeInterface) ResourceOwnerChanged(
	interpreter *interpreter.Interpreter,
	resource *interpreter.CompositeValue,
	oldOwner common.Address,
	newOwner common.Address,
) {
	if i.resourceOwnerChanged != nil {
		i.resourceOwnerChanged(
			interpreter,
			resource,
			oldOwner,
			newOwner,
		)
	}
}

func (i *testRuntimeInterface) GenerateUUID() (uint64, error) {
	if i.generateUUID == nil {
		return 0, nil
	}
	return i.generateUUID()
}

func (i *testRuntimeInterface) MeterComputation(compKind common.ComputationKind, intensity uint) error {
	if i.meterComputation == nil {
		return nil
	}
	return i.meterComputation(compKind, intensity)
}

func (i *testRuntimeInterface) DecodeArgument(b []byte, t cadence.Type) (cadence.Value, error) {
	return i.decodeArgument(b, t)
}

func (i *testRuntimeInterface) ProgramParsed(location runtime.Location, duration time.Duration) {
	if i.programParsed == nil {
		return
	}
	i.programParsed(location, duration)
}

func (i *testRuntimeInterface) ProgramChecked(location runtime.Location, duration time.Duration) {
	if i.programChecked == nil {
		return
	}
	i.programChecked(location, duration)
}

func (i *testRuntimeInterface) ProgramInterpreted(location runtime.Location, duration time.Duration) {
	if i.programInterpreted == nil {
		return
	}
	i.programInterpreted(location, duration)
}

func (i *testRuntimeInterface) GetCurrentBlockHeight() (uint64, error) {
	return 1, nil
}

func (i *testRuntimeInterface) GetBlockAtHeight(height uint64) (block cadenceStdlib.Block, exists bool, err error) {

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

func (i *testRuntimeInterface) ReadRandom(buffer []byte) error {
	if i.readRandom == nil {
		return nil
	}
	return i.readRandom(buffer)
}

func (i *testRuntimeInterface) VerifySignature(
	signature []byte,
	tag string,
	signedData []byte,
	publicKey []byte,
	signatureAlgorithm runtime.SignatureAlgorithm,
	hashAlgorithm runtime.HashAlgorithm,
) (bool, error) {
	if i.verifySignature == nil {
		return false, nil
	}
	return i.verifySignature(
		signature,
		tag,
		signedData,
		publicKey,
		signatureAlgorithm,
		hashAlgorithm,
	)
}

func (i *testRuntimeInterface) Hash(data []byte, tag string, hashAlgorithm runtime.HashAlgorithm) ([]byte, error) {
	if i.hash == nil {
		return nil, nil
	}
	return i.hash(data, tag, hashAlgorithm)
}

func (i *testRuntimeInterface) SetCadenceValue(owner common.Address, key string, value cadence.Value) (err error) {
	if i.setCadenceValue == nil {
		panic("must specify testRuntimeInterface.setCadenceValue")
	}
	return i.setCadenceValue(owner, key, value)
}

func (i *testRuntimeInterface) GetAccountBalance(address runtime.Address) (uint64, error) {
	if i.getAccountBalance == nil {
		panic("must specify testRuntimeInterface.getAccountBalance")
	}
	return i.getAccountBalance(address)
}

func (i *testRuntimeInterface) GetAccountAvailableBalance(address runtime.Address) (uint64, error) {
	if i.getAccountAvailableBalance == nil {
		panic("must specify testRuntimeInterface.getAccountAvailableBalance")
	}
	return i.getAccountAvailableBalance(address)
}

func (i *testRuntimeInterface) GetStorageUsed(address runtime.Address) (uint64, error) {
	if i.getStorageUsed == nil {
		panic("must specify testRuntimeInterface.getStorageUsed")
	}
	return i.getStorageUsed(address)
}

func (i *testRuntimeInterface) GetStorageCapacity(address runtime.Address) (uint64, error) {
	if i.getStorageCapacity == nil {
		panic("must specify testRuntimeInterface.getStorageCapacity")
	}
	return i.getStorageCapacity(address)
}

func (i *testRuntimeInterface) ImplementationDebugLog(message string) error {
	if i.implementationDebugLog == nil {
		return nil
	}
	return i.implementationDebugLog(message)
}

func (i *testRuntimeInterface) ValidatePublicKey(key *cadenceStdlib.PublicKey) error {
	if i.validatePublicKey == nil {
		return errors.New("mock defaults to public key validation failure")
	}

	return i.validatePublicKey(key)
}

func (i *testRuntimeInterface) BLSVerifyPOP(key *cadenceStdlib.PublicKey, s []byte) (bool, error) {
	if i.bLSVerifyPOP == nil {
		return false, nil
	}

	return i.bLSVerifyPOP(key, s)
}

func (i *testRuntimeInterface) BLSAggregateSignatures(sigs [][]byte) ([]byte, error) {
	if i.blsAggregateSignatures == nil {
		return []byte{}, nil
	}

	return i.blsAggregateSignatures(sigs)
}

func (i *testRuntimeInterface) BLSAggregatePublicKeys(keys []*cadenceStdlib.PublicKey) (*cadenceStdlib.PublicKey, error) {
	if i.blsAggregatePublicKeys == nil {
		return nil, nil
	}

	return i.blsAggregatePublicKeys(keys)
}

func (i *testRuntimeInterface) GetAccountContractNames(address runtime.Address) ([]string, error) {
	if i.getAccountContractNames == nil {
		return []string{}, nil
	}

	return i.getAccountContractNames(address)
}

func (i *testRuntimeInterface) RecordTrace(operation string, location runtime.Location, duration time.Duration, attrs []attribute.KeyValue) {
	if i.recordTrace == nil {
		return
	}
	i.recordTrace(operation, location, duration, attrs)
}

func (i *testRuntimeInterface) MeterMemory(usage common.MemoryUsage) error {
	if i.meterMemory == nil {
		return nil
	}

	return i.meterMemory(usage)
}

func (i *testRuntimeInterface) ComputationUsed() (uint64, error) {
	if i.computationUsed == nil {
		return 0, nil
	}

	return i.computationUsed()
}

func (i *testRuntimeInterface) MemoryUsed() (uint64, error) {
	if i.memoryUsed == nil {
		return 0, nil
	}

	return i.memoryUsed()
}

func (i *testRuntimeInterface) InteractionUsed() (uint64, error) {
	if i.interactionUsed == nil {
		return 0, nil
	}

	return i.interactionUsed()
}

func (i *testRuntimeInterface) UnsafeRandom() (uint64, error) {
	if i.unsafeRandom == nil {
		return 0, nil
	}

	return i.unsafeRandom()
}

func (i *testRuntimeInterface) onTransactionExecutionStart() {
	i.invalidateUpdatedPrograms()
}

func (i *testRuntimeInterface) onScriptExecutionStart() {
	i.invalidateUpdatedPrograms()
}

func (i *testRuntimeInterface) invalidateUpdatedPrograms() {
	if i.updatedContractCode {
		for location := range i.programs {
			delete(i.programs, location)
		}
		i.updatedContractCode = false
	}
}
