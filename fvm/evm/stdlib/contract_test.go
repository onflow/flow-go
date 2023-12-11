package stdlib_test

import (
	"encoding/binary"
	"testing"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	contracts2 "github.com/onflow/flow-core-contracts/lib/go/contracts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/fvm/evm/stdlib"
	. "github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

type testContractHandler struct {
	flowTokenAddress  common.Address
	allocateAddress   func() types.Address
	addressIndex      uint64
	accountByAddress  func(types.Address, bool) types.Account
	lastExecutedBlock func() *types.Block
	run               func(tx []byte, coinbase types.Address)
}

func (t *testContractHandler) FlowTokenAddress() common.Address {
	return t.flowTokenAddress
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
	balance  func() types.Balance
	transfer func(address types.Address, balance types.Balance)
	deposit  func(vault *types.FLOWTokenVault)
	withdraw func(balance types.Balance) *types.FLOWTokenVault
	deploy   func(code types.Code, limit types.GasLimit, balance types.Balance) types.Address
	call     func(address types.Address, data types.Data, limit types.GasLimit, balance types.Balance) types.Data
}

var _ types.Account = &testFlowAccount{}

func (t *testFlowAccount) Address() types.Address {
	return t.address
}

func (t *testFlowAccount) Balance() types.Balance {
	if t.balance == nil {
		return types.Balance(0)
	}
	return t.balance()
}

func (t *testFlowAccount) Transfer(address types.Address, balance types.Balance) {
	if t.transfer == nil {
		panic("unexpected Transfer")
	}
	t.transfer(address, balance)
}

func (t *testFlowAccount) Deposit(vault *types.FLOWTokenVault) {
	if t.deposit == nil {
		panic("unexpected Deposit")
	}
	t.deposit(vault)
}

func (t *testFlowAccount) Withdraw(balance types.Balance) *types.FLOWTokenVault {
	if t.withdraw == nil {
		panic("unexpected Withdraw")
	}
	return t.withdraw(balance)
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

func deployContracts(
	t *testing.T,
	rt runtime.Runtime,
	contractsAddress flow.Address,
	runtimeInterface *TestRuntimeInterface,
	transactionEnvironment runtime.Environment,
	nextTransactionLocation func() common.TransactionLocation,
) {

	contractsAddressHex := contractsAddress.Hex()

	contracts := []struct {
		name     string
		code     []byte
		deployTx []byte
	}{
		{
			name: "FungibleToken",
			code: contracts2.FungibleToken(),
		},
		{
			name: "NonFungibleToken",
			code: contracts2.NonFungibleToken(),
		},
		{
			name: "MetadataViews",
			code: contracts2.MetadataViews(
				contractsAddressHex,
				contractsAddressHex,
			),
		},
		{
			name: "FungibleTokenMetadataViews",
			code: contracts2.FungibleTokenMetadataViews(
				contractsAddressHex,
				contractsAddressHex,
			),
		},
		{
			name: "ViewResolver",
			code: contracts2.ViewResolver(),
		},
		{
			name: "FlowToken",
			code: contracts2.FlowToken(
				contractsAddressHex,
				contractsAddressHex,
				contractsAddressHex,
			),
			deployTx: []byte(`
              transaction(name: String, code: String) {
                prepare(signer: AuthAccount) {
                  signer.contracts.add(name: name, code: code.utf8, signer)
                }
              }
            `),
		},
		{
			name: stdlib.ContractName,
			code: stdlib.ContractCode(contractsAddress),
		},
	}

	for _, contract := range contracts {

		deployTx := contract.deployTx
		if len(deployTx) == 0 {
			deployTx = blueprints.DeployContractTransactionTemplate
		}

		err := rt.ExecuteTransaction(
			runtime.Script{
				Source: deployTx,
				Arguments: EncodeArgs([]cadence.Value{
					cadence.String(contract.name),
					cadence.String(contract.code),
				}),
			},
			runtime.Context{
				Interface:   runtimeInterface,
				Environment: transactionEnvironment,
				Location:    nextTransactionLocation(),
			},
		)
		require.NoError(t, err)
	}

}

func newEVMTransactionEnvironment(handler types.ContractHandler, service flow.Address) runtime.Environment {
	transactionEnvironment := runtime.NewBaseInterpreterEnvironment(runtime.Config{})

	stdlib.SetupEnvironment(
		transactionEnvironment,
		handler,
		service,
	)

	return transactionEnvironment
}

func newEVMScriptEnvironment(handler types.ContractHandler, service flow.Address) runtime.Environment {
	scriptEnvironment := runtime.NewScriptInterpreterEnvironment(runtime.Config{})

	stdlib.SetupEnvironment(
		scriptEnvironment,
		handler,
		service,
	)

	return scriptEnvironment
}

func TestEVMEncodeABI(t *testing.T) {

	t.Parallel()

	handler := &testContractHandler{}

	contractsAddress := flow.BytesToAddress([]byte{0x1})

	transactionEnvironment := newEVMTransactionEnvironment(handler, contractsAddress)
	scriptEnvironment := newEVMScriptEnvironment(handler, contractsAddress)

	rt := runtime.NewInterpreterRuntime(runtime.Config{})

	script := []byte(`
      import EVM from 0x1

      access(all)
      fun main(): [UInt8] {
        return EVM.encodeABI(["John Doe", UInt64(33), false])
      }
	`)

	accountCodes := map[common.Location][]byte{}
	var events []cadence.Event

	runtimeInterface := &TestRuntimeInterface{
		Storage: NewTestLedger(nil, nil),
		OnGetSigningAccounts: func() ([]runtime.Address, error) {
			return []runtime.Address{runtime.Address(contractsAddress)}, nil
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

	// Deploy contracts

	deployContracts(
		t,
		rt,
		contractsAddress,
		runtimeInterface,
		transactionEnvironment,
		nextTransactionLocation,
	)

	// Run script

	result, err := rt.ExecuteScript(
		runtime.Script{
			Source:    script,
			Arguments: [][]byte{},
		},
		runtime.Context{
			Interface:   runtimeInterface,
			Environment: scriptEnvironment,
			Location:    nextScriptLocation(),
		},
	)
	require.NoError(t, err)

	abiBytes := []byte{
		0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
		0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
		0x0, 0x0, 0x0, 0x60, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
		0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
		0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x21, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
		0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
		0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
		0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
		0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
		0x0, 0x8, 0x4a, 0x6f, 0x68, 0x6e, 0x20, 0x44, 0x6f, 0x65, 0x0, 0x0,
		0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
		0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
	}
	cdcBytes := make([]cadence.Value, 0)
	for _, bt := range abiBytes {
		cdcBytes = append(cdcBytes, cadence.UInt8(bt))
	}
	encodedABI := cadence.NewArray(
		cdcBytes,
	).WithType(cadence.NewVariableSizedArrayType(cadence.TheUInt8Type))

	assert.Equal(t,
		encodedABI,
		result,
	)
}

func TestEVMDecodeABI(t *testing.T) {

	t.Parallel()

	handler := &testContractHandler{}

	contractsAddress := flow.BytesToAddress([]byte{0x1})

	transactionEnvironment := newEVMTransactionEnvironment(handler, contractsAddress)
	scriptEnvironment := newEVMScriptEnvironment(handler, contractsAddress)

	rt := runtime.NewInterpreterRuntime(runtime.Config{})

	script := []byte(`
      import EVM from 0x1

      access(all)
      fun main(data: [UInt8]): Bool {
        let types = [Type<String>(), Type<UInt64>(), Type<Bool>()]
        let values = EVM.decodeABI(types: types, data: data)

        assert(values.length == 3)
        assert((values[0] as! String) == "John Doe")
        assert((values[1] as! UInt64) == UInt64(33))
        assert((values[2] as! Bool) == false)

        return true
      }
	`)

	accountCodes := map[common.Location][]byte{}
	var events []cadence.Event

	runtimeInterface := &TestRuntimeInterface{
		Storage: NewTestLedger(nil, nil),
		OnGetSigningAccounts: func() ([]runtime.Address, error) {
			return []runtime.Address{runtime.Address(contractsAddress)}, nil
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

	// Deploy contracts

	deployContracts(
		t,
		rt,
		contractsAddress,
		runtimeInterface,
		transactionEnvironment,
		nextTransactionLocation,
	)

	// Run script
	abiBytes := []byte{
		0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
		0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
		0x0, 0x0, 0x0, 0x60, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
		0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
		0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x21, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
		0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
		0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
		0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
		0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
		0x0, 0x8, 0x4a, 0x6f, 0x68, 0x6e, 0x20, 0x44, 0x6f, 0x65, 0x0, 0x0,
		0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
		0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
	}
	cdcBytes := make([]cadence.Value, 0)
	for _, bt := range abiBytes {
		cdcBytes = append(cdcBytes, cadence.UInt8(bt))
	}
	encodedABI := cadence.NewArray(
		cdcBytes,
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
			Environment: scriptEnvironment,
			Location:    nextScriptLocation(),
		},
	)
	require.NoError(t, err)

	assert.Equal(t, cadence.NewBool(true), result)
}

func TestEVMEncodeDecodeABIRoundtrip(t *testing.T) {

	t.Parallel()

	handler := &testContractHandler{}

	contractsAddress := flow.BytesToAddress([]byte{0x1})

	transactionEnvironment := newEVMTransactionEnvironment(handler, contractsAddress)
	scriptEnvironment := newEVMScriptEnvironment(handler, contractsAddress)

	rt := runtime.NewInterpreterRuntime(runtime.Config{})

	script := []byte(`
      import EVM from 0x1

      access(all)
      fun main(): Bool {
        // Check EVM.EVMAddress encode/decode
        // bytes for address 0x7A58c0Be72BE218B41C608b7Fe7C5bB630736C71
        let address = EVM.EVMAddress(
          bytes: [
            122, 88, 192, 190, 114, 190, 33, 139, 65, 198,
            8, 183, 254, 124, 91, 182, 48, 115, 108, 113
          ]
        )
        var data = EVM.encodeABI([address])
        var values = EVM.decodeABI(types: [Type<EVM.EVMAddress>()], data: data)
        assert(values.length == 1)
        assert((values[0] as! EVM.EVMAddress).bytes == address.bytes)

        // Check String encode/decode
        data = EVM.encodeABI(["John Doe", ""])
        values = EVM.decodeABI(types: [Type<String>(), Type<String>()], data: data)
        assert((values[0] as! String) == "John Doe")
        assert((values[1] as! String) == "")

        // Check Bool encode/decode
        data = EVM.encodeABI([true, false])
        values = EVM.decodeABI(types: [Type<Bool>(), Type<Bool>()], data: data)
        assert((values[0] as! Bool) == true)
        assert((values[1] as! Bool) == false)

        // Check UInt*/Int* encode/decode
        data = EVM.encodeABI([
          UInt8(33),
          UInt16(33),
          UInt32(33),
          UInt64(33),
          UInt128(33),
          UInt256(33),
          Int8(-33),
          Int16(-33),
          Int32(-33),
          Int64(-33),
          Int128(-33),
          Int256(-33)
        ])
        values = EVM.decodeABI(
          types: [
            Type<UInt8>(),
            Type<UInt16>(),
            Type<UInt32>(),
            Type<UInt64>(),
            Type<UInt128>(),
            Type<UInt256>(),
            Type<Int8>(),
            Type<Int16>(),
            Type<Int32>(),
            Type<Int64>(),
            Type<Int128>(),
            Type<Int256>()
          ],
          data: data
        )
        assert((values[0] as! UInt8) == 33)
        assert((values[1] as! UInt16) == 33)
        assert((values[2] as! UInt32) == 33)
        assert((values[3] as! UInt64) == 33)
        assert((values[4] as! UInt128) == 33)
        assert((values[5] as! UInt256) == 33)
        assert((values[6] as! Int8) == -33)
        assert((values[7] as! Int16) == -33)
        assert((values[8] as! Int32) == -33)
        assert((values[9] as! Int64) == -33)
        assert((values[10] as! Int128) == -33)
        assert((values[11] as! Int256) == -33)

        // Check array of leaf types encode/decode
        data = EVM.encodeABI([
          ["one", "two"],
          [true, false],
          [5, 10] as [UInt8],
          [5, 10] as [UInt16],
          [5, 10] as [UInt32],
          [5, 10] as [UInt64],
          [5, 10] as [UInt128],
          [5, 10] as [UInt256],
          [-5, -10] as [Int8],
          [-5, -10] as [Int16],
          [-5, -10] as [Int32],
          [-5, -10] as [Int64],
          [-5, -10] as [Int128],
          [-5, -10] as [Int256]
        ])
        values = EVM.decodeABI(
          types: [
            Type<[String]>(),
            Type<[Bool]>(),
            Type<[UInt8]>(),
            Type<[UInt16]>(),
            Type<[UInt32]>(),
            Type<[UInt64]>(),
            Type<[UInt128]>(),
            Type<[UInt256]>(),
            Type<[Int8]>(),
            Type<[Int16]>(),
            Type<[Int32]>(),
            Type<[Int64]>(),
            Type<[Int128]>(),
            Type<[Int256]>()
          ],
          data: data
        )
        assert((values[0] as! [String]) == ["one", "two"])
        assert((values[1] as! [Bool]) == [true, false])
        assert((values[2] as! [UInt8]) == [5, 10])
        assert((values[3] as! [UInt16]) == [5, 10])
        assert((values[4] as! [UInt32]) == [5, 10])
        assert((values[5] as! [UInt64]) == [5, 10])
        assert((values[6] as! [UInt128]) == [5, 10])
        assert((values[7] as! [UInt256]) == [5, 10])
        assert((values[8] as! [Int8]) == [-5, -10])
        assert((values[9] as! [Int16]) == [-5, -10])
        assert((values[10] as! [Int32]) == [-5, -10])
        assert((values[11] as! [Int64]) == [-5, -10])
        assert((values[12] as! [Int128]) == [-5, -10])
        assert((values[13] as! [Int256]) == [-5, -10])

        return true
      }
	`)

	accountCodes := map[common.Location][]byte{}
	var events []cadence.Event

	runtimeInterface := &TestRuntimeInterface{
		Storage: NewTestLedger(nil, nil),
		OnGetSigningAccounts: func() ([]runtime.Address, error) {
			return []runtime.Address{runtime.Address(contractsAddress)}, nil
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

	// Deploy contracts

	deployContracts(
		t,
		rt,
		contractsAddress,
		runtimeInterface,
		transactionEnvironment,
		nextTransactionLocation,
	)

	// Run script

	result, err := rt.ExecuteScript(
		runtime.Script{
			Source:    script,
			Arguments: [][]byte{},
		},
		runtime.Context{
			Interface:   runtimeInterface,
			Environment: scriptEnvironment,
			Location:    nextScriptLocation(),
		},
	)
	require.NoError(t, err)

	assert.Equal(t,
		cadence.Bool(true),
		result,
	)
}

func TestEVMEncodeDecodeABIErrors(t *testing.T) {

	t.Parallel()

	t.Run("encodeABI with unsupported Address type", func(t *testing.T) {

		t.Parallel()

		handler := &testContractHandler{}

		contractsAddress := flow.BytesToAddress([]byte{0x1})

		transactionEnvironment := newEVMTransactionEnvironment(handler, contractsAddress)
		scriptEnvironment := newEVMScriptEnvironment(handler, contractsAddress)

		rt := runtime.NewInterpreterRuntime(runtime.Config{})

		accountCodes := map[common.Location][]byte{}
		var events []cadence.Event

		runtimeInterface := &TestRuntimeInterface{
			Storage: NewTestLedger(nil, nil),
			OnGetSigningAccounts: func() ([]runtime.Address, error) {
				return []runtime.Address{runtime.Address(contractsAddress)}, nil
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

		// Deploy contracts

		deployContracts(
			t,
			rt,
			contractsAddress,
			runtimeInterface,
			transactionEnvironment,
			nextTransactionLocation,
		)

		// Run script

		script := []byte(`
          import EVM from 0x1

          access(all)
          fun main(): Bool {
            let address: Address = 0x045a1763c93006ca
            let data = EVM.encodeABI([address])

            return true
          }
		`)

		_, err := rt.ExecuteScript(
			runtime.Script{
				Source:    script,
				Arguments: [][]byte{},
			},
			runtime.Context{
				Interface:   runtimeInterface,
				Environment: scriptEnvironment,
				Location:    nextScriptLocation(),
			},
		)
		require.Error(t, err)
		assert.ErrorContains(
			t,
			err,
			"unsupported ABI encoding for value of type: Address",
		)
	})

	t.Run("encodeABI with unsupported fixed-point number type", func(t *testing.T) {

		t.Parallel()

		handler := &testContractHandler{}

		contractsAddress := flow.BytesToAddress([]byte{0x1})

		transactionEnvironment := newEVMTransactionEnvironment(handler, contractsAddress)
		scriptEnvironment := newEVMScriptEnvironment(handler, contractsAddress)

		rt := runtime.NewInterpreterRuntime(runtime.Config{})

		accountCodes := map[common.Location][]byte{}
		var events []cadence.Event

		runtimeInterface := &TestRuntimeInterface{
			Storage: NewTestLedger(nil, nil),
			OnGetSigningAccounts: func() ([]runtime.Address, error) {
				return []runtime.Address{runtime.Address(contractsAddress)}, nil
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

		// Deploy contracts

		deployContracts(
			t,
			rt,
			contractsAddress,
			runtimeInterface,
			transactionEnvironment,
			nextTransactionLocation,
		)

		// Run script

		script := []byte(`
          import EVM from 0x1

          access(all)
          fun main(): Bool {
            let data = EVM.encodeABI([0.2])

            return true
          }
		`)

		_, err := rt.ExecuteScript(
			runtime.Script{
				Source:    script,
				Arguments: [][]byte{},
			},
			runtime.Context{
				Interface:   runtimeInterface,
				Environment: scriptEnvironment,
				Location:    nextScriptLocation(),
			},
		)
		require.Error(t, err)
		assert.ErrorContains(
			t,
			err,
			"unsupported ABI encoding for value of type: UFix64",
		)
	})

	t.Run("encodeABI with unsupported dictionary type", func(t *testing.T) {

		t.Parallel()

		handler := &testContractHandler{}

		contractsAddress := flow.BytesToAddress([]byte{0x1})

		transactionEnvironment := newEVMTransactionEnvironment(handler, contractsAddress)
		scriptEnvironment := newEVMScriptEnvironment(handler, contractsAddress)

		rt := runtime.NewInterpreterRuntime(runtime.Config{})

		accountCodes := map[common.Location][]byte{}
		var events []cadence.Event

		runtimeInterface := &TestRuntimeInterface{
			Storage: NewTestLedger(nil, nil),
			OnGetSigningAccounts: func() ([]runtime.Address, error) {
				return []runtime.Address{runtime.Address(contractsAddress)}, nil
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

		// Deploy contracts

		deployContracts(
			t,
			rt,
			contractsAddress,
			runtimeInterface,
			transactionEnvironment,
			nextTransactionLocation,
		)

		// Run script

		script := []byte(`
          import EVM from 0x1

          access(all)
          fun main(): Bool {
            let dict: {Int: Bool} = {0: false, 1: true}
            let data = EVM.encodeABI([dict])

            return true
          }
		`)

		_, err := rt.ExecuteScript(
			runtime.Script{
				Source:    script,
				Arguments: [][]byte{},
			},
			runtime.Context{
				Interface:   runtimeInterface,
				Environment: scriptEnvironment,
				Location:    nextScriptLocation(),
			},
		)
		require.Error(t, err)
		assert.ErrorContains(
			t,
			err,
			"unsupported ABI encoding for value of type: {Int: Bool}",
		)
	})

	t.Run("encodeABI with unsupported array element type", func(t *testing.T) {

		t.Parallel()

		handler := &testContractHandler{}

		contractsAddress := flow.BytesToAddress([]byte{0x1})

		transactionEnvironment := newEVMTransactionEnvironment(handler, contractsAddress)
		scriptEnvironment := newEVMScriptEnvironment(handler, contractsAddress)

		rt := runtime.NewInterpreterRuntime(runtime.Config{})

		accountCodes := map[common.Location][]byte{}
		var events []cadence.Event

		runtimeInterface := &TestRuntimeInterface{
			Storage: NewTestLedger(nil, nil),
			OnGetSigningAccounts: func() ([]runtime.Address, error) {
				return []runtime.Address{runtime.Address(contractsAddress)}, nil
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

		// Deploy contracts

		deployContracts(
			t,
			rt,
			contractsAddress,
			runtimeInterface,
			transactionEnvironment,
			nextTransactionLocation,
		)

		// Run script

		script := []byte(`
          import EVM from 0x1

          access(all)
          fun main(): Bool {
            let chars: [Character] = ["a", "b", "c"]
            let data = EVM.encodeABI([chars])

            return true
          }
		`)

		_, err := rt.ExecuteScript(
			runtime.Script{
				Source:    script,
				Arguments: [][]byte{},
			},
			runtime.Context{
				Interface:   runtimeInterface,
				Environment: scriptEnvironment,
				Location:    nextScriptLocation(),
			},
		)
		require.Error(t, err)
		assert.ErrorContains(
			t,
			err,
			"unsupported ABI encoding for value of type: [Character]",
		)
	})

	t.Run("encodeABI with unsupported custom composite type", func(t *testing.T) {

		t.Parallel()

		handler := &testContractHandler{}

		contractsAddress := flow.BytesToAddress([]byte{0x1})

		transactionEnvironment := newEVMTransactionEnvironment(handler, contractsAddress)
		scriptEnvironment := newEVMScriptEnvironment(handler, contractsAddress)

		rt := runtime.NewInterpreterRuntime(runtime.Config{})

		accountCodes := map[common.Location][]byte{}
		var events []cadence.Event

		runtimeInterface := &TestRuntimeInterface{
			Storage: NewTestLedger(nil, nil),
			OnGetSigningAccounts: func() ([]runtime.Address, error) {
				return []runtime.Address{runtime.Address(contractsAddress)}, nil
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

		// Deploy contracts

		deployContracts(
			t,
			rt,
			contractsAddress,
			runtimeInterface,
			transactionEnvironment,
			nextTransactionLocation,
		)

		// Run script

		script := []byte(`
          import EVM from 0x1

          access(all) struct Token {
            access(all) let id: Int
            access(all) var balance: Int

            init(id: Int, balance: Int) {
              self.id = id
              self.balance = balance
            }
          }

          access(all)
          fun main(): Bool {
            let token = Token(id: 9, balance: 150)
            let data = EVM.encodeABI([token])

            return true
          }
		`)

		_, err := rt.ExecuteScript(
			runtime.Script{
				Source:    script,
				Arguments: [][]byte{},
			},
			runtime.Context{
				Interface:   runtimeInterface,
				Environment: scriptEnvironment,
				Location:    nextScriptLocation(),
			},
		)
		require.Error(t, err)
		assert.ErrorContains(
			t,
			err,
			"unsupported ABI encoding for value of type: s.0100000000000000000000000000000000000000000000000000000000000000.Token",
		)
	})

	t.Run("decodeABI with mismatched type", func(t *testing.T) {

		t.Parallel()

		handler := &testContractHandler{}

		contractsAddress := flow.BytesToAddress([]byte{0x1})

		transactionEnvironment := newEVMTransactionEnvironment(handler, contractsAddress)
		scriptEnvironment := newEVMScriptEnvironment(handler, contractsAddress)

		rt := runtime.NewInterpreterRuntime(runtime.Config{})

		accountCodes := map[common.Location][]byte{}
		var events []cadence.Event

		runtimeInterface := &TestRuntimeInterface{
			Storage: NewTestLedger(nil, nil),
			OnGetSigningAccounts: func() ([]runtime.Address, error) {
				return []runtime.Address{runtime.Address(contractsAddress)}, nil
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

		// Deploy contracts

		deployContracts(
			t,
			rt,
			contractsAddress,
			runtimeInterface,
			transactionEnvironment,
			nextTransactionLocation,
		)

		// Run script

		script := []byte(`
          import EVM from 0x1

          access(all)
          fun main(): Bool {
            let data = EVM.encodeABI(["Peter"])
            let values = EVM.decodeABI(types: [Type<Bool>()], data: data)

            return true
          }
		`)

		_, err := rt.ExecuteScript(
			runtime.Script{
				Source:    script,
				Arguments: [][]byte{},
			},
			runtime.Context{
				Interface:   runtimeInterface,
				Environment: scriptEnvironment,
				Location:    nextScriptLocation(),
			},
		)
		require.Error(t, err)
		assert.ErrorContains(
			t,
			err,
			"decoding of ABI to values failed with: abi: improperly encoded boolean value",
		)
	})

	t.Run("decodeABI with surplus of types", func(t *testing.T) {

		t.Parallel()

		handler := &testContractHandler{}

		contractsAddress := flow.BytesToAddress([]byte{0x1})

		transactionEnvironment := newEVMTransactionEnvironment(handler, contractsAddress)
		scriptEnvironment := newEVMScriptEnvironment(handler, contractsAddress)

		rt := runtime.NewInterpreterRuntime(runtime.Config{})

		accountCodes := map[common.Location][]byte{}
		var events []cadence.Event

		runtimeInterface := &TestRuntimeInterface{
			Storage: NewTestLedger(nil, nil),
			OnGetSigningAccounts: func() ([]runtime.Address, error) {
				return []runtime.Address{runtime.Address(contractsAddress)}, nil
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

		// Deploy contracts

		deployContracts(
			t,
			rt,
			contractsAddress,
			runtimeInterface,
			transactionEnvironment,
			nextTransactionLocation,
		)

		// Run script

		script := []byte(`
          import EVM from 0x1

          access(all)
          fun main(): Bool {
            let data = EVM.encodeABI(["Peter"])
            let values = EVM.decodeABI(types: [Type<String>(), Type<Bool>()], data: data)

            return true
          }
		`)

		_, err := rt.ExecuteScript(
			runtime.Script{
				Source:    script,
				Arguments: [][]byte{},
			},
			runtime.Context{
				Interface:   runtimeInterface,
				Environment: scriptEnvironment,
				Location:    nextScriptLocation(),
			},
		)
		require.Error(t, err)
		assert.ErrorContains(
			t,
			err,
			"decoding of ABI to values failed with: abi: improperly encoded boolean value",
		)
	})

	t.Run("decodeABI with surplus of values", func(t *testing.T) {
		t.Skip()

		t.Parallel()

		handler := &testContractHandler{}

		contractsAddress := flow.BytesToAddress([]byte{0x1})

		transactionEnvironment := newEVMTransactionEnvironment(handler, contractsAddress)
		scriptEnvironment := newEVMScriptEnvironment(handler, contractsAddress)

		rt := runtime.NewInterpreterRuntime(runtime.Config{})

		accountCodes := map[common.Location][]byte{}
		var events []cadence.Event

		runtimeInterface := &TestRuntimeInterface{
			Storage: NewTestLedger(nil, nil),
			OnGetSigningAccounts: func() ([]runtime.Address, error) {
				return []runtime.Address{runtime.Address(contractsAddress)}, nil
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

		// Deploy contracts

		deployContracts(
			t,
			rt,
			contractsAddress,
			runtimeInterface,
			transactionEnvironment,
			nextTransactionLocation,
		)

		// Run script

		script := []byte(`
          import EVM from 0x1

          access(all)
          fun main(): Bool {
            let data = EVM.encodeABI(["Peter", UInt64(9999)])
            let values = EVM.decodeABI(types: [Type<String>()], data: data)

            return true
          }
		`)

		_, err := rt.ExecuteScript(
			runtime.Script{
				Source:    script,
				Arguments: [][]byte{},
			},
			runtime.Context{
				Interface:   runtimeInterface,
				Environment: scriptEnvironment,
				Location:    nextScriptLocation(),
			},
		)
		require.Error(t, err)
		assert.ErrorContains(
			t,
			err,
			"decoding of ABI to values failed with: received 1 types for 2 values",
		)
	})

	t.Run("decodeABI with random data", func(t *testing.T) {
		t.Skip()

		t.Parallel()

		handler := &testContractHandler{}

		contractsAddress := flow.BytesToAddress([]byte{0x1})

		transactionEnvironment := newEVMTransactionEnvironment(handler, contractsAddress)
		scriptEnvironment := newEVMScriptEnvironment(handler, contractsAddress)

		rt := runtime.NewInterpreterRuntime(runtime.Config{})

		accountCodes := map[common.Location][]byte{}
		var events []cadence.Event

		runtimeInterface := &TestRuntimeInterface{
			Storage: NewTestLedger(nil, nil),
			OnGetSigningAccounts: func() ([]runtime.Address, error) {
				return []runtime.Address{runtime.Address(contractsAddress)}, nil
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

		// Deploy contracts

		deployContracts(
			t,
			rt,
			contractsAddress,
			runtimeInterface,
			transactionEnvironment,
			nextTransactionLocation,
		)

		// Run script

		script := []byte(`
          import EVM from 0x1

          access(all)
          fun main(): Bool {
            let data: [UInt8] = "436164656e636521".decodeHex()
            let values = EVM.decodeABI(types: [Type<String>()], data: data)

            return true
          }
		`)

		_, err := rt.ExecuteScript(
			runtime.Script{
				Source:    script,
				Arguments: [][]byte{},
			},
			runtime.Context{
				Interface:   runtimeInterface,
				Environment: scriptEnvironment,
				Location:    nextScriptLocation(),
			},
		)
		require.Error(t, err)
		assert.ErrorContains(
			t,
			err,
			"decoding of ABI to values failed with: received 1 types for 0 values",
		)
	})

	t.Run("decodeABI with unsupported Address type", func(t *testing.T) {

		t.Parallel()

		handler := &testContractHandler{}

		contractsAddress := flow.BytesToAddress([]byte{0x1})

		transactionEnvironment := newEVMTransactionEnvironment(handler, contractsAddress)
		scriptEnvironment := newEVMScriptEnvironment(handler, contractsAddress)

		rt := runtime.NewInterpreterRuntime(runtime.Config{})

		accountCodes := map[common.Location][]byte{}
		var events []cadence.Event

		runtimeInterface := &TestRuntimeInterface{
			Storage: NewTestLedger(nil, nil),
			OnGetSigningAccounts: func() ([]runtime.Address, error) {
				return []runtime.Address{runtime.Address(contractsAddress)}, nil
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

		// Deploy contracts

		deployContracts(
			t,
			rt,
			contractsAddress,
			runtimeInterface,
			transactionEnvironment,
			nextTransactionLocation,
		)

		// Run script

		script := []byte(`
          import EVM from 0x1

          access(all)
          fun main(): Bool {
            let data = EVM.encodeABI(["Peter"])
            let values = EVM.decodeABI(types: [Type<Address>()], data: data)

            return true
          }
		`)

		_, err := rt.ExecuteScript(
			runtime.Script{
				Source:    script,
				Arguments: [][]byte{},
			},
			runtime.Context{
				Interface:   runtimeInterface,
				Environment: scriptEnvironment,
				Location:    nextScriptLocation(),
			},
		)
		require.Error(t, err)
		assert.ErrorContains(
			t,
			err,
			"unsupported ABI decoding for value of type: Address",
		)
	})

	t.Run("decodeABI with unsupported fixed-point number type", func(t *testing.T) {

		t.Parallel()

		handler := &testContractHandler{}

		contractsAddress := flow.BytesToAddress([]byte{0x1})

		transactionEnvironment := newEVMTransactionEnvironment(handler, contractsAddress)
		scriptEnvironment := newEVMScriptEnvironment(handler, contractsAddress)

		rt := runtime.NewInterpreterRuntime(runtime.Config{})

		accountCodes := map[common.Location][]byte{}
		var events []cadence.Event

		runtimeInterface := &TestRuntimeInterface{
			Storage: NewTestLedger(nil, nil),
			OnGetSigningAccounts: func() ([]runtime.Address, error) {
				return []runtime.Address{runtime.Address(contractsAddress)}, nil
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

		// Deploy contracts

		deployContracts(
			t,
			rt,
			contractsAddress,
			runtimeInterface,
			transactionEnvironment,
			nextTransactionLocation,
		)

		// Run script

		script := []byte(`
          import EVM from 0x1

          access(all)
          fun main(): Bool {
            let data = EVM.encodeABI(["Peter"])
            let values = EVM.decodeABI(types: [Type<UFix64>()], data: data)

            return true
          }
		`)

		_, err := rt.ExecuteScript(
			runtime.Script{
				Source:    script,
				Arguments: [][]byte{},
			},
			runtime.Context{
				Interface:   runtimeInterface,
				Environment: scriptEnvironment,
				Location:    nextScriptLocation(),
			},
		)
		require.Error(t, err)
		assert.ErrorContains(
			t,
			err,
			"unsupported ABI decoding for value of type: UFix64",
		)
	})

	t.Run("encodeABI with unsupported dictionary type", func(t *testing.T) {

		t.Parallel()

		handler := &testContractHandler{}

		contractsAddress := flow.BytesToAddress([]byte{0x1})

		transactionEnvironment := newEVMTransactionEnvironment(handler, contractsAddress)
		scriptEnvironment := newEVMScriptEnvironment(handler, contractsAddress)

		rt := runtime.NewInterpreterRuntime(runtime.Config{})

		accountCodes := map[common.Location][]byte{}
		var events []cadence.Event

		runtimeInterface := &TestRuntimeInterface{
			Storage: NewTestLedger(nil, nil),
			OnGetSigningAccounts: func() ([]runtime.Address, error) {
				return []runtime.Address{runtime.Address(contractsAddress)}, nil
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

		// Deploy contracts

		deployContracts(
			t,
			rt,
			contractsAddress,
			runtimeInterface,
			transactionEnvironment,
			nextTransactionLocation,
		)

		// Run script

		script := []byte(`
          import EVM from 0x1

          access(all)
          fun main(): Bool {
            let data = EVM.encodeABI(["Peter"])
            let values = EVM.decodeABI(types: [Type<{Int: Bool}>()], data: data)

            return true
          }
		`)

		_, err := rt.ExecuteScript(
			runtime.Script{
				Source:    script,
				Arguments: [][]byte{},
			},
			runtime.Context{
				Interface:   runtimeInterface,
				Environment: scriptEnvironment,
				Location:    nextScriptLocation(),
			},
		)
		require.Error(t, err)
		assert.ErrorContains(
			t,
			err,
			"unsupported ABI decoding for value of type: {Int: Bool}",
		)
	})

	t.Run("decodeABI with unsupported array element type", func(t *testing.T) {

		t.Parallel()

		handler := &testContractHandler{}

		contractsAddress := flow.BytesToAddress([]byte{0x1})

		transactionEnvironment := newEVMTransactionEnvironment(handler, contractsAddress)
		scriptEnvironment := newEVMScriptEnvironment(handler, contractsAddress)

		rt := runtime.NewInterpreterRuntime(runtime.Config{})

		accountCodes := map[common.Location][]byte{}
		var events []cadence.Event

		runtimeInterface := &TestRuntimeInterface{
			Storage: NewTestLedger(nil, nil),
			OnGetSigningAccounts: func() ([]runtime.Address, error) {
				return []runtime.Address{runtime.Address(contractsAddress)}, nil
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

		// Deploy contracts

		deployContracts(
			t,
			rt,
			contractsAddress,
			runtimeInterface,
			transactionEnvironment,
			nextTransactionLocation,
		)

		// Run script

		script := []byte(`
          import EVM from 0x1

          access(all)
          fun main(): Bool {
            let data = EVM.encodeABI(["Peter"])
            let values = EVM.decodeABI(types: [Type<[Character]>()], data: data)

            return true
          }
		`)

		_, err := rt.ExecuteScript(
			runtime.Script{
				Source:    script,
				Arguments: [][]byte{},
			},
			runtime.Context{
				Interface:   runtimeInterface,
				Environment: scriptEnvironment,
				Location:    nextScriptLocation(),
			},
		)
		require.Error(t, err)
		assert.ErrorContains(
			t,
			err,
			"unsupported ABI decoding for value of type: [Character]",
		)
	})

	t.Run("decodeABI with unsupported custom composite type", func(t *testing.T) {

		t.Parallel()

		handler := &testContractHandler{}

		contractsAddress := flow.BytesToAddress([]byte{0x1})

		transactionEnvironment := newEVMTransactionEnvironment(handler, contractsAddress)
		scriptEnvironment := newEVMScriptEnvironment(handler, contractsAddress)

		rt := runtime.NewInterpreterRuntime(runtime.Config{})

		accountCodes := map[common.Location][]byte{}
		var events []cadence.Event

		runtimeInterface := &TestRuntimeInterface{
			Storage: NewTestLedger(nil, nil),
			OnGetSigningAccounts: func() ([]runtime.Address, error) {
				return []runtime.Address{runtime.Address(contractsAddress)}, nil
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

		// Deploy contracts

		deployContracts(
			t,
			rt,
			contractsAddress,
			runtimeInterface,
			transactionEnvironment,
			nextTransactionLocation,
		)

		// Run script

		script := []byte(`
          import EVM from 0x1

          access(all) struct Token {
            access(all) let id: Int
            access(all) var balance: Int

            init(id: Int, balance: Int) {
              self.id = id
              self.balance = balance
            }
          }

          access(all)
          fun main(): Bool {
            let data = EVM.encodeABI(["Peter"])
            let values = EVM.decodeABI(types: [Type<Token>()], data: data)

            return true
          }
		`)

		_, err := rt.ExecuteScript(
			runtime.Script{
				Source:    script,
				Arguments: [][]byte{},
			},
			runtime.Context{
				Interface:   runtimeInterface,
				Environment: scriptEnvironment,
				Location:    nextScriptLocation(),
			},
		)
		require.Error(t, err)
		assert.ErrorContains(
			t,
			err,
			"unsupported ABI decoding for value of type: s.0100000000000000000000000000000000000000000000000000000000000000.Token",
		)
	})
}

func TestEVMAddressConstructionAndReturn(t *testing.T) {

	t.Parallel()

	handler := &testContractHandler{}

	contractsAddress := flow.BytesToAddress([]byte{0x1})

	transactionEnvironment := newEVMTransactionEnvironment(handler, contractsAddress)
	scriptEnvironment := newEVMScriptEnvironment(handler, contractsAddress)

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
			return []runtime.Address{runtime.Address(contractsAddress)}, nil
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

	// Deploy contracts

	deployContracts(
		t,
		rt,
		contractsAddress,
		runtimeInterface,
		transactionEnvironment,
		nextTransactionLocation,
	)

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
			Environment: scriptEnvironment,
			Location:    nextScriptLocation(),
		},
	)
	require.NoError(t, err)

	evmAddressCadenceType := stdlib.NewEVMAddressCadenceType(common.Address(contractsAddress))

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

	contractsAddress := flow.BytesToAddress([]byte{0x1})

	transactionEnvironment := newEVMTransactionEnvironment(handler, contractsAddress)
	scriptEnvironment := newEVMScriptEnvironment(handler, contractsAddress)

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
			return []runtime.Address{runtime.Address(contractsAddress)}, nil
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

	// Deploy contracts

	deployContracts(
		t,
		rt,
		contractsAddress,
		runtimeInterface,
		transactionEnvironment,
		nextTransactionLocation,
	)

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
			Environment: scriptEnvironment,
			Location:    nextScriptLocation(),
		},
	)
	require.NoError(t, err)

	evmBalanceCadenceType := stdlib.NewBalanceCadenceType(common.Address(contractsAddress))

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

	contractsAddress := flow.BytesToAddress([]byte{0x1})

	transactionEnvironment := newEVMTransactionEnvironment(handler, contractsAddress)
	scriptEnvironment := newEVMScriptEnvironment(handler, contractsAddress)

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
			return []runtime.Address{runtime.Address(contractsAddress)}, nil
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

	// Deploy contracts

	deployContracts(
		t,
		rt,
		contractsAddress,
		runtimeInterface,
		transactionEnvironment,
		nextTransactionLocation,
	)

	// Run script

	_, err := rt.ExecuteScript(
		runtime.Script{
			Source:    script,
			Arguments: EncodeArgs([]cadence.Value{evmTx, coinbase}),
		},
		runtime.Context{
			Interface:   runtimeInterface,
			Environment: scriptEnvironment,
			Location:    nextScriptLocation(),
		},
	)
	require.NoError(t, err)

	assert.True(t, runCalled)
}

func TestEVMCreateBridgedAccount(t *testing.T) {

	t.Parallel()

	handler := &testContractHandler{}

	contractsAddress := flow.BytesToAddress([]byte{0x1})

	transactionEnvironment := newEVMTransactionEnvironment(handler, contractsAddress)
	scriptEnvironment := newEVMScriptEnvironment(handler, contractsAddress)

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
			return []runtime.Address{runtime.Address(contractsAddress)}, nil
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

	// Deploy contracts

	deployContracts(
		t,
		rt,
		contractsAddress,
		runtimeInterface,
		transactionEnvironment,
		nextTransactionLocation,
	)

	// Run script

	actual, err := rt.ExecuteScript(
		runtime.Script{
			Source: script,
		},
		runtime.Context{
			Interface:   runtimeInterface,
			Environment: scriptEnvironment,
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

	contractsAddress := flow.BytesToAddress([]byte{0x1})

	transactionEnvironment := newEVMTransactionEnvironment(handler, contractsAddress)
	scriptEnvironment := newEVMScriptEnvironment(handler, contractsAddress)

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
			return []runtime.Address{runtime.Address(contractsAddress)}, nil
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

	// Deploy contracts

	deployContracts(
		t,
		rt,
		contractsAddress,
		runtimeInterface,
		transactionEnvironment,
		nextTransactionLocation,
	)

	// Run script

	actual, err := rt.ExecuteScript(
		runtime.Script{
			Source: script,
		},
		runtime.Context{
			Interface:   runtimeInterface,
			Environment: scriptEnvironment,
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

func TestEVMAddressDeposit(t *testing.T) {

	t.Parallel()

	expectedBalance, err := cadence.NewUFix64FromParts(1, 23000000)
	require.NoError(t, err)

	var deposited bool

	handler := &testContractHandler{

		accountByAddress: func(fromAddress types.Address, isAuthorized bool) types.Account {
			assert.Equal(t, types.Address{2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, fromAddress)
			assert.False(t, isAuthorized)

			return &testFlowAccount{
				address: fromAddress,
				deposit: func(vault *types.FLOWTokenVault) {
					deposited = true
					assert.Equal(
						t,
						types.Balance(expectedBalance),
						vault.Balance(),
					)
				},
			}
		},
	}

	contractsAddress := flow.BytesToAddress([]byte{0x1})

	transactionEnvironment := newEVMTransactionEnvironment(handler, contractsAddress)
	scriptEnvironment := newEVMScriptEnvironment(handler, contractsAddress)

	rt := runtime.NewInterpreterRuntime(runtime.Config{})

	script := []byte(`
      import EVM from 0x1
      import FlowToken from 0x1

      access(all)
      fun main() {
          let admin = getAuthAccount(0x1)
              .borrow<&FlowToken.Administrator>(from: /storage/flowTokenAdmin)!
          let minter <- admin.createNewMinter(allowedAmount: 1.23)
          let vault <- minter.mintTokens(amount: 1.23)
          destroy minter

          let address = EVM.EVMAddress(
              bytes: [2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]
          )
          address.deposit(from: <-vault)
      }
   `)

	accountCodes := map[common.Location][]byte{}
	var events []cadence.Event

	runtimeInterface := &TestRuntimeInterface{
		Storage: NewTestLedger(nil, nil),
		OnGetSigningAccounts: func() ([]runtime.Address, error) {
			return []runtime.Address{runtime.Address(contractsAddress)}, nil
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

	// Deploy contracts

	deployContracts(
		t,
		rt,
		contractsAddress,
		runtimeInterface,
		transactionEnvironment,
		nextTransactionLocation,
	)

	// Run script

	_, err = rt.ExecuteScript(
		runtime.Script{
			Source: script,
		},
		runtime.Context{
			Interface:   runtimeInterface,
			Environment: scriptEnvironment,
			Location:    nextScriptLocation(),
		},
	)
	require.NoError(t, err)

	require.True(t, deposited)
}

func TestBridgedAccountWithdraw(t *testing.T) {

	t.Parallel()

	expectedDepositBalance, err := cadence.NewUFix64FromParts(2, 34000000)
	require.NoError(t, err)

	expectedWithdrawBalance, err := cadence.NewUFix64FromParts(1, 23000000)
	require.NoError(t, err)

	var deposited bool
	var withdrew bool

	contractsAddress := flow.BytesToAddress([]byte{0x1})

	handler := &testContractHandler{
		flowTokenAddress: common.Address(contractsAddress),
		accountByAddress: func(fromAddress types.Address, isAuthorized bool) types.Account {
			assert.Equal(t, types.Address{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, fromAddress)
			assert.Equal(t, deposited, isAuthorized)

			return &testFlowAccount{
				address: fromAddress,
				deposit: func(vault *types.FLOWTokenVault) {
					deposited = true
					assert.Equal(t,
						types.Balance(expectedDepositBalance),
						vault.Balance(),
					)
				},
				withdraw: func(balance types.Balance) *types.FLOWTokenVault {
					assert.Equal(t,
						types.Balance(expectedWithdrawBalance),
						balance,
					)
					withdrew = true
					return types.NewFlowTokenVault(balance)
				},
			}
		},
	}

	transactionEnvironment := newEVMTransactionEnvironment(handler, contractsAddress)
	scriptEnvironment := newEVMScriptEnvironment(handler, contractsAddress)

	rt := runtime.NewInterpreterRuntime(runtime.Config{})

	script := []byte(`
      import EVM from 0x1
      import FlowToken from 0x1

      access(all)
      fun main(): UFix64 {
          let admin = getAuthAccount(0x1)
              .borrow<&FlowToken.Administrator>(from: /storage/flowTokenAdmin)!
          let minter <- admin.createNewMinter(allowedAmount: 2.34)
          let vault <- minter.mintTokens(amount: 2.34)
          destroy minter

          let bridgedAccount <- EVM.createBridgedAccount()
          bridgedAccount.address().deposit(from: <-vault)

          let vault2 <- bridgedAccount.withdraw(balance: EVM.Balance(flow: 1.23))
          let balance = vault2.balance
          destroy bridgedAccount
          destroy vault2

          return balance
      }
   `)

	accountCodes := map[common.Location][]byte{}
	var events []cadence.Event

	runtimeInterface := &TestRuntimeInterface{
		Storage: NewTestLedger(nil, nil),
		OnGetSigningAccounts: func() ([]runtime.Address, error) {
			return []runtime.Address{runtime.Address(contractsAddress)}, nil
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

	// Deploy contracts

	deployContracts(
		t,
		rt,
		contractsAddress,
		runtimeInterface,
		transactionEnvironment,
		nextTransactionLocation,
	)

	// Run script

	result, err := rt.ExecuteScript(
		runtime.Script{
			Source: script,
		},
		runtime.Context{
			Interface:   runtimeInterface,
			Environment: scriptEnvironment,
			Location:    nextScriptLocation(),
		},
	)
	require.NoError(t, err)

	assert.True(t, deposited)
	assert.True(t, withdrew)
	assert.Equal(t, expectedWithdrawBalance, result)
}

func TestBridgedAccountDeploy(t *testing.T) {

	t.Parallel()

	var deployed bool

	contractsAddress := flow.BytesToAddress([]byte{0x1})

	expectedBalance, err := cadence.NewUFix64FromParts(1, 23000000)
	require.NoError(t, err)

	var handler *testContractHandler
	handler = &testContractHandler{
		flowTokenAddress: common.Address(contractsAddress),
		accountByAddress: func(fromAddress types.Address, isAuthorized bool) types.Account {
			assert.Equal(t, types.Address{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, fromAddress)
			assert.True(t, isAuthorized)

			return &testFlowAccount{
				address: fromAddress,
				deploy: func(code types.Code, limit types.GasLimit, balance types.Balance) types.Address {
					deployed = true
					assert.Equal(t, types.Code{4, 5, 6}, code)
					assert.Equal(t, types.GasLimit(9999), limit)
					assert.Equal(t, types.Balance(expectedBalance), balance)

					return handler.AllocateAddress()
				},
			}
		},
	}

	transactionEnvironment := newEVMTransactionEnvironment(handler, contractsAddress)
	scriptEnvironment := newEVMScriptEnvironment(handler, contractsAddress)

	rt := runtime.NewInterpreterRuntime(runtime.Config{})

	script := []byte(`
      import EVM from 0x1
      import FlowToken from 0x1

      access(all)
      fun main(): [UInt8; 20] {
          let bridgedAccount <- EVM.createBridgedAccount()
          let address = bridgedAccount.deploy(
              code: [4, 5, 6],
              gasLimit: 9999,
              value: EVM.Balance(flow: 1.23)
          )
          destroy bridgedAccount
          return address.bytes
      }
   `)

	accountCodes := map[common.Location][]byte{}
	var events []cadence.Event

	runtimeInterface := &TestRuntimeInterface{
		Storage: NewTestLedger(nil, nil),
		OnGetSigningAccounts: func() ([]runtime.Address, error) {
			return []runtime.Address{runtime.Address(contractsAddress)}, nil
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

	// Deploy contracts

	deployContracts(
		t,
		rt,
		contractsAddress,
		runtimeInterface,
		transactionEnvironment,
		nextTransactionLocation,
	)

	// Run script

	actual, err := rt.ExecuteScript(
		runtime.Script{
			Source: script,
		},
		runtime.Context{
			Interface:   runtimeInterface,
			Environment: scriptEnvironment,
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

	require.True(t, deployed)
}

func TestEVMAccountBalance(t *testing.T) {

	t.Parallel()

	contractsAddress := flow.BytesToAddress([]byte{0x1})

	expectedBalanceValue, err := cadence.NewUFix64FromParts(1, 1337000)
	expectedBalance := cadence.
		NewStruct([]cadence.Value{expectedBalanceValue}).
		WithType(stdlib.NewBalanceCadenceType(common.Address(contractsAddress)))

	require.NoError(t, err)

	handler := &testContractHandler{
		flowTokenAddress: common.Address(contractsAddress),
		accountByAddress: func(fromAddress types.Address, isAuthorized bool) types.Account {
			assert.Equal(t, types.Address{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, fromAddress)
			assert.False(t, isAuthorized)

			return &testFlowAccount{
				address: fromAddress,
				balance: func() types.Balance {
					return types.Balance(expectedBalanceValue)
				},
			}
		},
	}

	transactionEnvironment := newEVMTransactionEnvironment(handler, contractsAddress)
	scriptEnvironment := newEVMScriptEnvironment(handler, contractsAddress)

	rt := runtime.NewInterpreterRuntime(runtime.Config{})

	script := []byte(`
      import EVM from 0x1

      access(all)
      fun main(): EVM.Balance {
          let bridgedAccount <- EVM.createBridgedAccount()
          let balance = bridgedAccount.balance()
          destroy bridgedAccount
          return balance
      }
    `)

	accountCodes := map[common.Location][]byte{}
	var events []cadence.Event

	runtimeInterface := &TestRuntimeInterface{
		Storage: NewTestLedger(nil, nil),
		OnGetSigningAccounts: func() ([]runtime.Address, error) {
			return []runtime.Address{runtime.Address(contractsAddress)}, nil
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

	// Deploy contracts

	deployContracts(
		t,
		rt,
		contractsAddress,
		runtimeInterface,
		transactionEnvironment,
		nextTransactionLocation,
	)

	// Run script

	actual, err := rt.ExecuteScript(
		runtime.Script{
			Source: script,
		},
		runtime.Context{
			Interface:   runtimeInterface,
			Environment: scriptEnvironment,
			Location:    nextScriptLocation(),
		},
	)
	require.NoError(t, err)

	require.NoError(t, err)
	require.Equal(t, expectedBalance, actual)
}
