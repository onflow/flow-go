package stdlib_test

import (
	"encoding/binary"
	"testing"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/fvm/evm/stdlib"
	. "github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

type testContractHandler struct {
	allocateAddress   func() types.Address
	addressIndex      uint64
	accountByAddress  func(types.Address, bool) types.Account
	lastExecutedBlock func() *types.Block
	run               func(tx []byte, coinbase types.Address)
}

var _ types.ContractHandler = &testContractHandler{}

func (t *testContractHandler) AllocateAddress() types.Address {
	if t.allocateAddress == nil {
		t.addressIndex++
		var address types.Address
		binary.LittleEndian.PutUint64(address[:], t.addressIndex)
		return address
	}
	return t.allocateAddress()
}

func (t *testContractHandler) AccountByAddress(addr types.Address, isAuthorized bool) types.Account {
	if t.accountByAddress == nil {
		panic("unexpected AccountByAddress")
	}
	return t.accountByAddress(addr, isAuthorized)
}

func (t *testContractHandler) LastExecutedBlock() *types.Block {
	if t.lastExecutedBlock == nil {
		panic("unexpected LastExecutedBlock")
	}
	return t.lastExecutedBlock()
}

func (t *testContractHandler) Run(tx []byte, coinbase types.Address) {
	if t.run == nil {
		panic("unexpected Run")
	}
	t.run(tx, coinbase)
}

type testFlowAccount struct {
	address  types.Address
	vault    *types.FLOWTokenVault
	deploy   func(code types.Code, limit types.GasLimit, balance types.Balance) types.Address
	call     func(address types.Address, data types.Data, limit types.GasLimit, balance types.Balance) types.Data
	transfer func(address types.Address, balance types.Balance)
}

var _ types.Account = &testFlowAccount{}

func (t *testFlowAccount) Address() types.Address {
	return t.address
}

func (t *testFlowAccount) Balance() types.Balance {
	if t.vault == nil {
		return types.Balance(0)
	}
	return t.vault.Balance()
}

func (t *testFlowAccount) Transfer(address types.Address, balance types.Balance) {
	if t.transfer == nil {
		panic("unexpected Call")
	}
	t.transfer(address, balance)
}

func (t *testFlowAccount) Deposit(vault *types.FLOWTokenVault) {
	if t.vault == nil {
		t.vault = types.NewFlowTokenVault(0)
	}
	t.vault.Deposit(vault)
}

func (t *testFlowAccount) Withdraw(balance types.Balance) *types.FLOWTokenVault {
	if t.vault == nil {
		return types.NewFlowTokenVault(0)
	}
	return t.vault.Withdraw(balance)
}

func (t *testFlowAccount) Deploy(code types.Code, limit types.GasLimit, balance types.Balance) types.Address {
	if t.deploy == nil {
		panic("unexpected Deploy")
	}
	return t.deploy(code, limit, balance)
}

func (t *testFlowAccount) Call(address types.Address, data types.Data, limit types.GasLimit, balance types.Balance) types.Data {
	if t.call == nil {
		panic("unexpected Call")
	}
	return t.call(address, data, limit, balance)
}

func TestEVMEncodeABI(t *testing.T) {

	t.Parallel()

	handler := &testContractHandler{}

	env := runtime.NewBaseInterpreterEnvironment(runtime.Config{})

	contractAddress := flow.BytesToAddress([]byte{0x1})

	stdlib.SetupEnvironment(env, handler, contractAddress)

	rt := runtime.NewInterpreterRuntime(runtime.Config{})

	script := []byte(`
      import EVM from 0x1

      access(all)
      fun main(): [UInt8] {
        let abiSpec = "[{\"type\": \"function\", \"name\": \"details\", \"inputs\": [{ \"name\": \"name\", \"type\": \"string\" }, { \"name\": \"surname\", \"type\": \"string\" }, { \"name\": \"age\", \"type\": \"uint64\" }], \"outputs\": []}]"
        return EVM.encodeABI(abiSpec, arguments: ["John", "Doe", UInt64(33)])
      }
	`)

	accountCodes := map[common.Location][]byte{}
	var events []cadence.Event

	runtimeInterface := &TestRuntimeInterface{
		Storage: NewTestLedger(nil, nil),
		OnGetSigningAccounts: func() ([]runtime.Address, error) {
			return []runtime.Address{runtime.Address(contractAddress)}, nil
		},
		OnResolveLocation: SingleIdentifierLocationResolver(t),
		OnUpdateAccountContractCode: func(location common.AddressLocation, code []byte) error {
			accountCodes[location] = code
			return nil
		},
		OnGetAccountContractCode: func(location common.AddressLocation) (code []byte, err error) {
			code = accountCodes[location]
			return code, nil
		},
		OnEmitEvent: func(event cadence.Event) error {
			events = append(events, event)
			return nil
		},
		OnDecodeArgument: func(b []byte, t cadence.Type) (cadence.Value, error) {
			return json.Decode(nil, b)
		},
	}

	nextTransactionLocation := NewTransactionLocationGenerator()
	nextScriptLocation := NewScriptLocationGenerator()

	// Deploy EVM contract

	err := rt.ExecuteTransaction(
		runtime.Script{
			Source: blueprints.DeployContractTransactionTemplate,
			Arguments: EncodeArgs([]cadence.Value{
				cadence.String(stdlib.ContractName),
				cadence.String(stdlib.ContractCode),
			}),
		},
		runtime.Context{
			Interface:   runtimeInterface,
			Environment: env,
			Location:    nextTransactionLocation(),
		},
	)
	require.NoError(t, err)

	// Run script

	result, err := rt.ExecuteScript(
		runtime.Script{
			Source:    script,
			Arguments: [][]byte{},
		},
		runtime.Context{
			Interface:   runtimeInterface,
			Environment: env,
			Location:    nextScriptLocation(),
		},
	)
	require.NoError(t, err)

	encodedABI := cadence.NewArray(
		[]cadence.Value{
			cadence.UInt8(0xef), cadence.UInt8(0x84), cadence.UInt8(0xad), cadence.UInt8(0x63),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x60),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0xa0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x21),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x4),
			cadence.UInt8(0x4a), cadence.UInt8(0x6f), cadence.UInt8(0x68), cadence.UInt8(0x6e),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x3),
			cadence.UInt8(0x44), cadence.UInt8(0x6f), cadence.UInt8(0x65), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0)},
	).WithType(cadence.NewVariableSizedArrayType(cadence.TheUInt8Type))
	assert.Equal(t,
		encodedABI,
		result,
	)
}

func TestEVMDecodeABI(t *testing.T) {

	t.Parallel()

	handler := &testContractHandler{}

	env := runtime.NewBaseInterpreterEnvironment(runtime.Config{})

	contractAddress := flow.BytesToAddress([]byte{0x1})

	stdlib.SetupEnvironment(env, handler, contractAddress)

	rt := runtime.NewInterpreterRuntime(runtime.Config{})

	script := []byte(`
      import EVM from 0x1

      access(all)
      fun main(data: [UInt8]): Bool {
        let abiSpec = "[{\"type\": \"function\", \"name\": \"details\", \"inputs\": [{ \"name\": \"name\", \"type\": \"string\" }, { \"name\": \"surname\", \"type\": \"string\" }, { \"name\": \"age\", \"type\": \"uint64\" }], \"outputs\": []}]"
        let unpacked = EVM.decodeABI(abiSpec, data: data)
        assert(unpacked.length == 3)
        assert((unpacked[0] as! String) == "John")
        assert((unpacked[1] as! String) == "Doe")
        assert((unpacked[2] as! UInt64) == UInt64(33))
        return true
      }
	`)

	accountCodes := map[common.Location][]byte{}
	var events []cadence.Event

	runtimeInterface := &TestRuntimeInterface{
		Storage: NewTestLedger(nil, nil),
		OnGetSigningAccounts: func() ([]runtime.Address, error) {
			return []runtime.Address{runtime.Address(contractAddress)}, nil
		},
		OnResolveLocation: SingleIdentifierLocationResolver(t),
		OnUpdateAccountContractCode: func(location common.AddressLocation, code []byte) error {
			accountCodes[location] = code
			return nil
		},
		OnGetAccountContractCode: func(location common.AddressLocation) (code []byte, err error) {
			code = accountCodes[location]
			return code, nil
		},
		OnEmitEvent: func(event cadence.Event) error {
			events = append(events, event)
			return nil
		},
		OnDecodeArgument: func(b []byte, t cadence.Type) (cadence.Value, error) {
			return json.Decode(nil, b)
		},
	}

	nextTransactionLocation := NewTransactionLocationGenerator()
	nextScriptLocation := NewScriptLocationGenerator()

	// Deploy EVM contract

	err := rt.ExecuteTransaction(
		runtime.Script{
			Source: blueprints.DeployContractTransactionTemplate,
			Arguments: EncodeArgs([]cadence.Value{
				cadence.String(stdlib.ContractName),
				cadence.String(stdlib.ContractCode),
			}),
		},
		runtime.Context{
			Interface:   runtimeInterface,
			Environment: env,
			Location:    nextTransactionLocation(),
		},
	)
	require.NoError(t, err)

	// Run script
	encodedABI := cadence.NewArray(
		[]cadence.Value{
			cadence.UInt8(0xef), cadence.UInt8(0x84), cadence.UInt8(0xad), cadence.UInt8(0x63),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x60),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0xa0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x21),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x4),
			cadence.UInt8(0x4a), cadence.UInt8(0x6f), cadence.UInt8(0x68), cadence.UInt8(0x6e),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x3),
			cadence.UInt8(0x44), cadence.UInt8(0x6f), cadence.UInt8(0x65), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0),
			cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0), cadence.UInt8(0x0)},
	).WithType(cadence.NewVariableSizedArrayType(cadence.TheUInt8Type))

	result, err := rt.ExecuteScript(
		runtime.Script{
			Source: script,
			Arguments: EncodeArgs([]cadence.Value{
				encodedABI,
			}),
		},
		runtime.Context{
			Interface:   runtimeInterface,
			Environment: env,
			Location:    nextScriptLocation(),
		},
	)
	require.NoError(t, err)

	assert.Equal(t, cadence.NewBool(true), result)
}

func TestEVMAddressConstructionAndReturn(t *testing.T) {

	t.Parallel()

	handler := &testContractHandler{}

	env := runtime.NewBaseInterpreterEnvironment(runtime.Config{})

	contractAddress := flow.BytesToAddress([]byte{0x1})

	stdlib.SetupEnvironment(env, handler, contractAddress)

	rt := runtime.NewInterpreterRuntime(runtime.Config{})

	script := []byte(`
      import EVM from 0x1

      access(all)
      fun main(_ bytes: [UInt8; 20]): EVM.EVMAddress {
          return EVM.EVMAddress(bytes: bytes)
      }
    `)

	accountCodes := map[common.Location][]byte{}
	var events []cadence.Event

	runtimeInterface := &TestRuntimeInterface{
		Storage: NewTestLedger(nil, nil),
		OnGetSigningAccounts: func() ([]runtime.Address, error) {
			return []runtime.Address{runtime.Address(contractAddress)}, nil
		},
		OnResolveLocation: SingleIdentifierLocationResolver(t),
		OnUpdateAccountContractCode: func(location common.AddressLocation, code []byte) error {
			accountCodes[location] = code
			return nil
		},
		OnGetAccountContractCode: func(location common.AddressLocation) (code []byte, err error) {
			code = accountCodes[location]
			return code, nil
		},
		OnEmitEvent: func(event cadence.Event) error {
			events = append(events, event)
			return nil
		},
		OnDecodeArgument: func(b []byte, t cadence.Type) (cadence.Value, error) {
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
	}).WithType(stdlib.EVMAddressBytesCadenceType)

	nextTransactionLocation := NewTransactionLocationGenerator()
	nextScriptLocation := NewScriptLocationGenerator()

	// Deploy EVM contract

	err := rt.ExecuteTransaction(
		runtime.Script{
			Source: blueprints.DeployContractTransactionTemplate,
			Arguments: EncodeArgs([]cadence.Value{
				cadence.String(stdlib.ContractName),
				cadence.String(stdlib.ContractCode),
			}),
		},
		runtime.Context{
			Interface:   runtimeInterface,
			Environment: env,
			Location:    nextTransactionLocation(),
		},
	)
	require.NoError(t, err)

	// Run script

	result, err := rt.ExecuteScript(
		runtime.Script{
			Source: script,
			Arguments: EncodeArgs([]cadence.Value{
				addressBytesArray,
			}),
		},
		runtime.Context{
			Interface:   runtimeInterface,
			Environment: env,
			Location:    nextScriptLocation(),
		},
	)
	require.NoError(t, err)

	evmAddressCadenceType := stdlib.NewEVMAddressCadenceType(common.Address(contractAddress))

	assert.Equal(t,
		cadence.Struct{
			StructType: evmAddressCadenceType,
			Fields: []cadence.Value{
				addressBytesArray,
			},
		},
		result,
	)
}

func TestBalanceConstructionAndReturn(t *testing.T) {

	t.Parallel()

	handler := &testContractHandler{}

	env := runtime.NewBaseInterpreterEnvironment(runtime.Config{})

	contractAddress := flow.BytesToAddress([]byte{0x1})

	stdlib.SetupEnvironment(env, handler, contractAddress)

	rt := runtime.NewInterpreterRuntime(runtime.Config{})

	script := []byte(`
      import EVM from 0x1

      access(all)
      fun main(_ flow: UFix64): EVM.Balance {
          return EVM.Balance(flow: flow)
      }
    `)

	accountCodes := map[common.Location][]byte{}
	var events []cadence.Event

	runtimeInterface := &TestRuntimeInterface{
		Storage: NewTestLedger(nil, nil),
		OnGetSigningAccounts: func() ([]runtime.Address, error) {
			return []runtime.Address{runtime.Address(contractAddress)}, nil
		},
		OnResolveLocation: SingleIdentifierLocationResolver(t),
		OnUpdateAccountContractCode: func(location common.AddressLocation, code []byte) error {
			accountCodes[location] = code
			return nil
		},
		OnGetAccountContractCode: func(location common.AddressLocation) (code []byte, err error) {
			code = accountCodes[location]
			return code, nil
		},
		OnEmitEvent: func(event cadence.Event) error {
			events = append(events, event)
			return nil
		},
		OnDecodeArgument: func(b []byte, t cadence.Type) (cadence.Value, error) {
			return json.Decode(nil, b)
		},
	}

	nextTransactionLocation := NewTransactionLocationGenerator()
	nextScriptLocation := NewScriptLocationGenerator()

	// Deploy EVM contract

	err := rt.ExecuteTransaction(
		runtime.Script{
			Source: blueprints.DeployContractTransactionTemplate,
			Arguments: EncodeArgs([]cadence.Value{
				cadence.String(stdlib.ContractName),
				cadence.String(stdlib.ContractCode),
			}),
		},
		runtime.Context{
			Interface:   runtimeInterface,
			Environment: env,
			Location:    nextTransactionLocation(),
		},
	)
	require.NoError(t, err)

	// Run script

	flowValue, err := cadence.NewUFix64FromParts(1, 23000000)
	require.NoError(t, err)

	result, err := rt.ExecuteScript(
		runtime.Script{
			Source: script,
			Arguments: EncodeArgs([]cadence.Value{
				flowValue,
			}),
		},
		runtime.Context{
			Interface:   runtimeInterface,
			Environment: env,
			Location:    nextScriptLocation(),
		},
	)
	require.NoError(t, err)

	evmBalanceCadenceType := stdlib.NewBalanceCadenceType(common.Address(contractAddress))

	assert.Equal(t,
		cadence.Struct{
			StructType: evmBalanceCadenceType,
			Fields: []cadence.Value{
				flowValue,
			},
		},
		result,
	)
}

func TestEVMRun(t *testing.T) {

	t.Parallel()

	evmTx := cadence.NewArray([]cadence.Value{
		cadence.UInt8(1),
		cadence.UInt8(2),
		cadence.UInt8(3),
	}).WithType(stdlib.EVMTransactionBytesCadenceType)

	coinbase := cadence.NewArray([]cadence.Value{
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
	}).WithType(stdlib.EVMAddressBytesCadenceType)

	runCalled := false

	handler := &testContractHandler{
		run: func(tx []byte, coinbase types.Address) {
			runCalled = true

			assert.Equal(t, []byte{1, 2, 3}, tx)
			assert.Equal(t,
				types.Address{
					1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 6, 7, 7, 8, 8, 9, 9, 10, 10,
				},
				coinbase,
			)

		},
	}

	env := runtime.NewBaseInterpreterEnvironment(runtime.Config{})

	contractAddress := flow.BytesToAddress([]byte{0x1})

	stdlib.SetupEnvironment(env, handler, contractAddress)

	rt := runtime.NewInterpreterRuntime(runtime.Config{})

	script := []byte(`
      import EVM from 0x1

      access(all)
      fun main(tx: [UInt8], coinbaseBytes: [UInt8; 20]) {
          let coinbase = EVM.EVMAddress(bytes: coinbaseBytes)
          EVM.run(tx: tx, coinbase: coinbase)
      }
    `)

	accountCodes := map[common.Location][]byte{}
	var events []cadence.Event

	runtimeInterface := &TestRuntimeInterface{
		Storage: NewTestLedger(nil, nil),
		OnGetSigningAccounts: func() ([]runtime.Address, error) {
			return []runtime.Address{runtime.Address(contractAddress)}, nil
		},
		OnResolveLocation: SingleIdentifierLocationResolver(t),
		OnUpdateAccountContractCode: func(location common.AddressLocation, code []byte) error {
			accountCodes[location] = code
			return nil
		},
		OnGetAccountContractCode: func(location common.AddressLocation) (code []byte, err error) {
			code = accountCodes[location]
			return code, nil
		},
		OnEmitEvent: func(event cadence.Event) error {
			events = append(events, event)
			return nil
		},
		OnDecodeArgument: func(b []byte, t cadence.Type) (cadence.Value, error) {
			return json.Decode(nil, b)
		},
	}

	nextTransactionLocation := NewTransactionLocationGenerator()
	nextScriptLocation := NewScriptLocationGenerator()

	// Deploy EVM contract

	err := rt.ExecuteTransaction(
		runtime.Script{
			Source: blueprints.DeployContractTransactionTemplate,
			Arguments: EncodeArgs([]cadence.Value{
				cadence.String(stdlib.ContractName),
				cadence.String(stdlib.ContractCode),
			}),
		},
		runtime.Context{
			Interface:   runtimeInterface,
			Environment: env,
			Location:    nextTransactionLocation(),
		},
	)
	require.NoError(t, err)

	// Run script

	_, err = rt.ExecuteScript(
		runtime.Script{
			Source:    script,
			Arguments: EncodeArgs([]cadence.Value{evmTx, coinbase}),
		},
		runtime.Context{
			Interface:   runtimeInterface,
			Environment: env,
			Location:    nextScriptLocation(),
		},
	)
	require.NoError(t, err)

	assert.True(t, runCalled)
}

func TestEVMCreateBridgedAccount(t *testing.T) {

	t.Parallel()

	handler := &testContractHandler{}

	env := runtime.NewBaseInterpreterEnvironment(runtime.Config{})

	contractAddress := flow.BytesToAddress([]byte{0x1})

	stdlib.SetupEnvironment(env, handler, contractAddress)

	rt := runtime.NewInterpreterRuntime(runtime.Config{})

	script := []byte(`
      import EVM from 0x1

      access(all)
      fun main(): [UInt8; 20] {
          let bridgedAccount1 <- EVM.createBridgedAccount()
          destroy bridgedAccount1

          let bridgedAccount2 <- EVM.createBridgedAccount()
          let bytes = bridgedAccount2.address().bytes
          destroy bridgedAccount2

          return bytes
      }
    `)

	accountCodes := map[common.Location][]byte{}
	var events []cadence.Event

	runtimeInterface := &TestRuntimeInterface{
		Storage: NewTestLedger(nil, nil),
		OnGetSigningAccounts: func() ([]runtime.Address, error) {
			return []runtime.Address{runtime.Address(contractAddress)}, nil
		},
		OnResolveLocation: SingleIdentifierLocationResolver(t),
		OnUpdateAccountContractCode: func(location common.AddressLocation, code []byte) error {
			accountCodes[location] = code
			return nil
		},
		OnGetAccountContractCode: func(location common.AddressLocation) (code []byte, err error) {
			code = accountCodes[location]
			return code, nil
		},
		OnEmitEvent: func(event cadence.Event) error {
			events = append(events, event)
			return nil
		},
		OnDecodeArgument: func(b []byte, t cadence.Type) (cadence.Value, error) {
			return json.Decode(nil, b)
		},
	}

	nextTransactionLocation := NewTransactionLocationGenerator()
	nextScriptLocation := NewScriptLocationGenerator()

	// Deploy EVM contract

	err := rt.ExecuteTransaction(
		runtime.Script{
			Source: blueprints.DeployContractTransactionTemplate,
			Arguments: EncodeArgs([]cadence.Value{
				cadence.String(stdlib.ContractName),
				cadence.String(stdlib.ContractCode),
			}),
		},
		runtime.Context{
			Interface:   runtimeInterface,
			Environment: env,
			Location:    nextTransactionLocation(),
		},
	)
	require.NoError(t, err)

	// Run script

	actual, err := rt.ExecuteScript(
		runtime.Script{
			Source: script,
		},
		runtime.Context{
			Interface:   runtimeInterface,
			Environment: env,
			Location:    nextScriptLocation(),
		},
	)
	require.NoError(t, err)

	expected := cadence.NewArray([]cadence.Value{
		cadence.UInt8(2), cadence.UInt8(0),
		cadence.UInt8(0), cadence.UInt8(0),
		cadence.UInt8(0), cadence.UInt8(0),
		cadence.UInt8(0), cadence.UInt8(0),
		cadence.UInt8(0), cadence.UInt8(0),
		cadence.UInt8(0), cadence.UInt8(0),
		cadence.UInt8(0), cadence.UInt8(0),
		cadence.UInt8(0), cadence.UInt8(0),
		cadence.UInt8(0), cadence.UInt8(0),
		cadence.UInt8(0), cadence.UInt8(0),
	}).WithType(cadence.NewConstantSizedArrayType(
		types.AddressLength,
		cadence.UInt8Type{},
	))

	require.Equal(t, expected, actual)
}

func TestBridgedAccountCall(t *testing.T) {

	t.Parallel()

	expectedBalance, err := cadence.NewUFix64FromParts(1, 23000000)
	require.NoError(t, err)

	handler := &testContractHandler{
		accountByAddress: func(fromAddress types.Address, isAuthorized bool) types.Account {
			assert.Equal(t, types.Address{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, fromAddress)
			assert.True(t, isAuthorized)

			return &testFlowAccount{
				address: fromAddress,
				call: func(
					toAddress types.Address,
					data types.Data,
					limit types.GasLimit,
					balance types.Balance,
				) types.Data {
					assert.Equal(t, types.Address{2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, toAddress)
					assert.Equal(t, types.Data{4, 5, 6}, data)
					assert.Equal(t, types.GasLimit(9999), limit)
					assert.Equal(t, types.Balance(expectedBalance), balance)

					return types.Data{3, 1, 4}
				},
			}
		},
	}

	env := runtime.NewBaseInterpreterEnvironment(runtime.Config{})

	contractAddress := flow.BytesToAddress([]byte{0x1})

	stdlib.SetupEnvironment(env, handler, contractAddress)

	rt := runtime.NewInterpreterRuntime(runtime.Config{})

	script := []byte(`
      import EVM from 0x1

      access(all)
      fun main(): [UInt8] {
          let bridgedAccount <- EVM.createBridgedAccount()
          let response = bridgedAccount.call(
              to: EVM.EVMAddress(
                  bytes: [2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]
              ),
              data: [4, 5, 6],
              gasLimit: 9999,
              value: EVM.Balance(flow: 1.23)
          )
          destroy bridgedAccount
          return response
      }
   `)

	accountCodes := map[common.Location][]byte{}
	var events []cadence.Event

	runtimeInterface := &TestRuntimeInterface{
		Storage: NewTestLedger(nil, nil),
		OnGetSigningAccounts: func() ([]runtime.Address, error) {
			return []runtime.Address{runtime.Address(contractAddress)}, nil
		},
		OnResolveLocation: SingleIdentifierLocationResolver(t),
		OnUpdateAccountContractCode: func(location common.AddressLocation, code []byte) error {
			accountCodes[location] = code
			return nil
		},
		OnGetAccountContractCode: func(location common.AddressLocation) (code []byte, err error) {
			code = accountCodes[location]
			return code, nil
		},
		OnEmitEvent: func(event cadence.Event) error {
			events = append(events, event)
			return nil
		},
		OnDecodeArgument: func(b []byte, t cadence.Type) (cadence.Value, error) {
			return json.Decode(nil, b)
		},
	}

	nextTransactionLocation := NewTransactionLocationGenerator()
	nextScriptLocation := NewScriptLocationGenerator()

	// Deploy EVM contract

	err = rt.ExecuteTransaction(
		runtime.Script{
			Source: blueprints.DeployContractTransactionTemplate,
			Arguments: EncodeArgs([]cadence.Value{
				cadence.String(stdlib.ContractName),
				cadence.String(stdlib.ContractCode),
			}),
		},
		runtime.Context{
			Interface:   runtimeInterface,
			Environment: env,
			Location:    nextTransactionLocation(),
		},
	)
	require.NoError(t, err)

	// Run script

	actual, err := rt.ExecuteScript(
		runtime.Script{
			Source: script,
		},
		runtime.Context{
			Interface:   runtimeInterface,
			Environment: env,
			Location:    nextScriptLocation(),
		},
	)
	require.NoError(t, err)

	expected := cadence.NewArray([]cadence.Value{
		cadence.UInt8(3),
		cadence.UInt8(1),
		cadence.UInt8(4),
	}).WithType(cadence.NewVariableSizedArrayType(cadence.UInt8Type{}))

	require.Equal(t, expected, actual)
}
