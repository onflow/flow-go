package stdlib_test

import (
	"encoding/binary"
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/sema"
	cadenceStdlib "github.com/onflow/cadence/runtime/stdlib"
	"github.com/onflow/cadence/runtime/tests/utils"
	coreContracts "github.com/onflow/flow-core-contracts/lib/go/contracts"
	coreContractstemplates "github.com/onflow/flow-core-contracts/lib/go/templates"
	"github.com/onflow/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/evm/stdlib"
	. "github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

type testContractHandler struct {
	flowTokenAddress     common.Address
	evmContractAddress   common.Address
	deployCOA            func(uint64) types.Address
	accountByAddress     func(types.Address, bool) types.Account
	lastExecutedBlock    func() *types.Block
	run                  func(tx []byte, coinbase types.Address) *types.ResultSummary
	batchRun             func(txs [][]byte, coinbase types.Address) []*types.ResultSummary
	generateResourceUUID func() uint64
	dryRun               func(tx []byte, from types.Address) *types.ResultSummary
}

var _ types.ContractHandler = &testContractHandler{}

func (t *testContractHandler) FlowTokenAddress() common.Address {
	return t.flowTokenAddress
}

func (t *testContractHandler) EVMContractAddress() common.Address {
	return t.evmContractAddress
}

func (t *testContractHandler) DeployCOA(uuid uint64) types.Address {
	if t.deployCOA == nil {
		var address types.Address
		binary.LittleEndian.PutUint64(address[:], uuid)
		return address
	}
	return t.deployCOA(uuid)
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

func (t *testContractHandler) Run(tx []byte, coinbase types.Address) *types.ResultSummary {
	if t.run == nil {
		panic("unexpected Run")
	}
	return t.run(tx, coinbase)
}

func (t *testContractHandler) DryRun(tx []byte, from types.Address) *types.ResultSummary {
	if t.dryRun == nil {
		panic("unexpected DryRun")
	}
	return t.dryRun(tx, from)
}

func (t *testContractHandler) BatchRun(txs [][]byte, coinbase types.Address) []*types.ResultSummary {
	if t.batchRun == nil {
		panic("unexpected BatchRun")
	}
	return t.batchRun(txs, coinbase)
}

func (t *testContractHandler) GenerateResourceUUID() uint64 {
	if t.generateResourceUUID == nil {
		panic("unexpected GenerateResourceUUID")
	}
	return t.generateResourceUUID()
}

type testFlowAccount struct {
	address  types.Address
	balance  func() types.Balance
	code     func() types.Code
	codeHash func() []byte
	nonce    func() uint64
	transfer func(address types.Address, balance types.Balance)
	deposit  func(vault *types.FLOWTokenVault)
	withdraw func(balance types.Balance) *types.FLOWTokenVault
	deploy   func(code types.Code, limit types.GasLimit, balance types.Balance) *types.ResultSummary
	call     func(address types.Address, data types.Data, limit types.GasLimit, balance types.Balance) *types.ResultSummary
}

var _ types.Account = &testFlowAccount{}

func (t *testFlowAccount) Address() types.Address {
	return t.address
}

func (t *testFlowAccount) Balance() types.Balance {
	if t.balance == nil {
		return types.NewBalanceFromUFix64(0)
	}
	return t.balance()
}

func (t *testFlowAccount) Code() types.Code {
	if t.code == nil {
		return types.Code{}
	}
	return t.code()
}

func (t *testFlowAccount) CodeHash() []byte {
	if t.codeHash == nil {
		return nil
	}
	return t.codeHash()
}

func (t *testFlowAccount) Nonce() uint64 {
	if t.nonce == nil {
		return 0
	}
	return t.nonce()
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

func (t *testFlowAccount) Deploy(code types.Code, limit types.GasLimit, balance types.Balance) *types.ResultSummary {
	if t.deploy == nil {
		panic("unexpected Deploy")
	}
	return t.deploy(code, limit, balance)
}

func (t *testFlowAccount) Call(address types.Address, data types.Data, limit types.GasLimit, balance types.Balance) *types.ResultSummary {
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

	env := coreContractstemplates.Environment{
		ServiceAccountAddress:             contractsAddressHex,
		ViewResolverAddress:               contractsAddressHex,
		BurnerAddress:                     contractsAddressHex,
		FungibleTokenAddress:              contractsAddressHex,
		NonFungibleTokenAddress:           contractsAddressHex,
		MetadataViewsAddress:              contractsAddressHex,
		FungibleTokenMetadataViewsAddress: contractsAddressHex,
	}

	contracts := []struct {
		name     string
		code     []byte
		deployTx []byte
	}{
		{
			name: "ViewResolver",
			code: coreContracts.ViewResolver(),
		},
		{
			name: "Burner",
			code: coreContracts.Burner(),
		},
		{
			name: "FungibleToken",
			code: coreContracts.FungibleToken(
				env,
			),
		},
		{
			name: "NonFungibleToken",
			code: coreContracts.NonFungibleToken(
				env,
			),
		},
		{
			name: "MetadataViews",
			code: coreContracts.MetadataViews(
				env,
			),
		},
		{
			name: "FungibleTokenMetadataViews",
			code: coreContracts.FungibleTokenMetadataViews(
				env,
			),
		},
		{
			name: "FlowToken",
			code: coreContracts.FlowToken(
				env,
			),
			deployTx: []byte(`
              transaction(name: String, code: String) {
                prepare(signer: auth(AddContract, Storage, Capabilities) &Account) {
                  signer.contracts.add(name: name, code: code.utf8, signer)
                }
              }
            `),
		},
		{
			name: stdlib.ContractName,
			code: stdlib.ContractCode(contractsAddress, contractsAddress, contractsAddress),
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

func newEVMTransactionEnvironment(handler types.ContractHandler, contractAddress flow.Address) runtime.Environment {
	transactionEnvironment := runtime.NewBaseInterpreterEnvironment(runtime.Config{})

	stdlib.SetupEnvironment(
		transactionEnvironment,
		handler,
		contractAddress,
	)

	return transactionEnvironment
}

func newEVMScriptEnvironment(handler types.ContractHandler, contractAddress flow.Address) runtime.Environment {
	scriptEnvironment := runtime.NewScriptInterpreterEnvironment(runtime.Config{})

	stdlib.SetupEnvironment(
		scriptEnvironment,
		handler,
		contractAddress,
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

	computation := uint(0)
	runtimeInterface := &TestRuntimeInterface{
		Storage: NewTestLedger(nil, nil),
		OnGetSigningAccounts: func() ([]runtime.Address, error) {
			return []runtime.Address{runtime.Address(contractsAddress)}, nil
		},
		OnResolveLocation: LocationResolver,
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
		OnMeterComputation: func(compKind common.ComputationKind, intensity uint) error {
			if compKind == environment.ComputationKindEVMEncodeABI {
				computation += intensity
			}
			return nil
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
	).WithType(cadence.NewVariableSizedArrayType(cadence.UInt8Type))

	assert.Equal(t,
		encodedABI,
		result,
	)
	assert.Equal(t, computation, uint(len(cdcBytes)))
}

func TestEVMEncodeABIComputation(t *testing.T) {

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
        let address = EVM.EVMAddress(
          bytes: [
            122, 88, 192, 190, 114, 190, 33, 139, 65, 198,
            8, 183, 254, 124, 91, 182, 48, 115, 108, 113
          ]
        )
        let arr: [UInt8] = [1, 2, 3, 4, 5]

        return EVM.encodeABI([
          "John Doe",
          UInt64(33),
          false,
          address,
          [arr],
          ["one", "two", "three"]
        ])
      }
	`)

	accountCodes := map[common.Location][]byte{}
	var events []cadence.Event

	computation := uint(0)
	runtimeInterface := &TestRuntimeInterface{
		Storage: NewTestLedger(nil, nil),
		OnGetSigningAccounts: func() ([]runtime.Address, error) {
			return []runtime.Address{runtime.Address(contractsAddress)}, nil
		},
		OnResolveLocation: LocationResolver,
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
		OnMeterComputation: func(compKind common.ComputationKind, intensity uint) error {
			if compKind == environment.ComputationKindEVMEncodeABI {
				computation += intensity
			}
			return nil
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

	cdcBytes, ok := result.(cadence.Array)
	require.True(t, ok)
	// computation & len(cdcBytes.Values) is equal to 832
	assert.Equal(t, computation, uint(len(cdcBytes.Values)))
}

func TestEVMEncodeABIComputationEmptyDynamicVariables(t *testing.T) {

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
        return EVM.encodeABI([
          "",
          [[""], [] as [String]],
          [] as [UInt8],
          ["", "", ""]
        ])
      }
	`)

	accountCodes := map[common.Location][]byte{}
	var events []cadence.Event

	computation := uint(0)
	runtimeInterface := &TestRuntimeInterface{
		Storage: NewTestLedger(nil, nil),
		OnGetSigningAccounts: func() ([]runtime.Address, error) {
			return []runtime.Address{runtime.Address(contractsAddress)}, nil
		},
		OnResolveLocation: LocationResolver,
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
		OnMeterComputation: func(compKind common.ComputationKind, intensity uint) error {
			if compKind == environment.ComputationKindEVMEncodeABI {
				computation += intensity
			}
			return nil
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

	cdcBytes, ok := result.(cadence.Array)
	require.True(t, ok)
	// computation & len(cdcBytes.Values) is equal to 832
	assert.Equal(t, computation, uint(len(cdcBytes.Values)))
}

func TestEVMEncodeABIComputationDynamicVariablesAboveChunkSize(t *testing.T) {

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
        let str = "abcdefghijklmnopqrstuvwxyz"
        let arr: [UInt64] = [
          1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19,
          20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36,
          37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47, 48, 49, 50, 51, 52, 53
        ]

        return EVM.encodeABI([
          str,
          str.concat(str).concat(str),
          [[str]],
          arr,
          [arr],
          arr.concat(arr).concat(arr)
        ])
      }
	`)

	accountCodes := map[common.Location][]byte{}
	var events []cadence.Event

	computation := uint(0)
	runtimeInterface := &TestRuntimeInterface{
		Storage: NewTestLedger(nil, nil),
		OnGetSigningAccounts: func() ([]runtime.Address, error) {
			return []runtime.Address{runtime.Address(contractsAddress)}, nil
		},
		OnResolveLocation: LocationResolver,
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
		OnMeterComputation: func(compKind common.ComputationKind, intensity uint) error {
			if compKind == environment.ComputationKindEVMEncodeABI {
				computation += intensity
			}
			return nil
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

	cdcBytes, ok := result.(cadence.Array)
	require.True(t, ok)
	// computation & len(cdcBytes.Values) is equal to 832
	assert.Equal(t, computation, uint(len(cdcBytes.Values)))
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

	computation := uint(0)
	runtimeInterface := &TestRuntimeInterface{
		Storage: NewTestLedger(nil, nil),
		OnGetSigningAccounts: func() ([]runtime.Address, error) {
			return []runtime.Address{runtime.Address(contractsAddress)}, nil
		},
		OnResolveLocation: LocationResolver,
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
		OnMeterComputation: func(compKind common.ComputationKind, intensity uint) error {
			if compKind == environment.ComputationKindEVMDecodeABI {
				computation += intensity
			}
			return nil
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
	).WithType(cadence.NewVariableSizedArrayType(cadence.UInt8Type))

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
	assert.Equal(t, computation, uint(len(cdcBytes)))
}

func TestEVMDecodeABIComputation(t *testing.T) {

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
        let address = EVM.EVMAddress(
          bytes: [
            122, 88, 192, 190, 114, 190, 33, 139, 65, 198,
            8, 183, 254, 124, 91, 182, 48, 115, 108, 113
          ]
        )
        let arr: [UInt8] = [1, 2, 3, 4, 5]

        let data = EVM.encodeABI([
          "John Doe",
          UInt64(33),
          true,
          address,
          [arr],
          ["one", "two", "three"]
        ])

        let types = [
          Type<String>(), Type<UInt64>(), Type<Bool>(), Type<EVM.EVMAddress>(),
          Type<[[UInt8]]>(), Type<[String]>()
        ]
        let values = EVM.decodeABI(types: types, data: data)

        return data
      }
	`)

	accountCodes := map[common.Location][]byte{}
	var events []cadence.Event

	computation := uint(0)
	runtimeInterface := &TestRuntimeInterface{
		Storage: NewTestLedger(nil, nil),
		OnGetSigningAccounts: func() ([]runtime.Address, error) {
			return []runtime.Address{runtime.Address(contractsAddress)}, nil
		},
		OnResolveLocation: LocationResolver,
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
		OnMeterComputation: func(compKind common.ComputationKind, intensity uint) error {
			if compKind == environment.ComputationKindEVMDecodeABI {
				computation += intensity
			}
			return nil
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

	cdcBytes, ok := result.(cadence.Array)
	require.True(t, ok)
	// computation & len(cdcBytes.Values) is equal to 832
	assert.Equal(t, computation, uint(len(cdcBytes.Values)))
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

        // Check variable-size array of leaf types encode/decode
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
          [-5, -10] as [Int256],
          [address] as [EVM.EVMAddress]
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
            Type<[Int256]>(),
            Type<[EVM.EVMAddress]>()
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
        assert((values[14] as! [EVM.EVMAddress])[0].bytes == [address][0].bytes)

        // Check constant-size array of leaf types encode/decode
        data = EVM.encodeABI([
          ["one", "two"] as [String; 2],
          [true, false] as [Bool; 2],
          [5, 10] as [UInt8; 2],
          [5, 10] as [UInt16; 2],
          [5, 10] as [UInt32; 2],
          [5, 10] as [UInt64; 2],
          [5, 10] as [UInt128; 2],
          [5, 10] as [UInt256; 2],
          [-5, -10] as [Int8; 2],
          [-5, -10] as [Int16; 2],
          [-5, -10] as [Int32; 2],
          [-5, -10] as [Int64; 2],
          [-5, -10] as [Int128; 2],
          [-5, -10] as [Int256; 2],
          [address] as [EVM.EVMAddress; 1]
        ])
        values = EVM.decodeABI(
          types: [
            Type<[String; 2]>(),
            Type<[Bool; 2]>(),
            Type<[UInt8; 2]>(),
            Type<[UInt16; 2]>(),
            Type<[UInt32; 2]>(),
            Type<[UInt64; 2]>(),
            Type<[UInt128; 2]>(),
            Type<[UInt256; 2]>(),
            Type<[Int8; 2]>(),
            Type<[Int16; 2]>(),
            Type<[Int32; 2]>(),
            Type<[Int64; 2]>(),
            Type<[Int128; 2]>(),
            Type<[Int256; 2]>(),
            Type<[EVM.EVMAddress; 1]>()
          ],
          data: data
        )
        assert((values[0] as! [String; 2]) == ["one", "two"])
        assert((values[1] as! [Bool; 2]) == [true, false])
        assert((values[2] as! [UInt8; 2]) == [5, 10])
        assert((values[3] as! [UInt16; 2]) == [5, 10])
        assert((values[4] as! [UInt32; 2]) == [5, 10])
        assert((values[5] as! [UInt64; 2]) == [5, 10])
        assert((values[6] as! [UInt128; 2]) == [5, 10])
        assert((values[7] as! [UInt256; 2]) == [5, 10])
        assert((values[8] as! [Int8; 2]) == [-5, -10])
        assert((values[9] as! [Int16; 2]) == [-5, -10])
        assert((values[10] as! [Int32; 2]) == [-5, -10])
        assert((values[11] as! [Int64; 2]) == [-5, -10])
        assert((values[12] as! [Int128; 2]) == [-5, -10])
        assert((values[13] as! [Int256; 2]) == [-5, -10])
        assert((values[14] as! [EVM.EVMAddress; 1])[0].bytes == [address][0].bytes)

        // Check partial decoding of encoded data
        data = EVM.encodeABI(["Peter", UInt64(9999)])
        values = EVM.decodeABI(types: [Type<String>()], data: data)
        assert(values.length == 1)
        assert((values[0] as! String) == "Peter")

        // Check nested arrays of leaf values
        data = EVM.encodeABI([[["Foo", "Bar"], ["Baz", "Qux"]]])
        values = EVM.decodeABI(types: [Type<[[String]]>()], data: data)
        assert(values.length == 1)
        assert((values[0] as! [[String]]) == [["Foo", "Bar"], ["Baz", "Qux"]])

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
		OnResolveLocation: LocationResolver,
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
			OnResolveLocation: LocationResolver,
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
		utils.RequireError(t, err)
		assert.ErrorContains(
			t,
			err,
			"failed to ABI encode value of type Address",
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
			OnResolveLocation: LocationResolver,
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
		utils.RequireError(t, err)
		assert.ErrorContains(
			t,
			err,
			"failed to ABI encode value of type UFix64",
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
			OnResolveLocation: LocationResolver,
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
		utils.RequireError(t, err)
		assert.ErrorContains(
			t,
			err,
			"failed to ABI encode value of type {Int: Bool}",
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
			OnResolveLocation: LocationResolver,
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
		utils.RequireError(t, err)
		assert.ErrorContains(
			t,
			err,
			"failed to ABI encode value of type Character",
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
			OnResolveLocation: LocationResolver,
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
            access(all) var balance: UInt

            init(id: Int, balance: UInt) {
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
		utils.RequireError(t, err)
		assert.ErrorContains(
			t,
			err,
			"failed to ABI encode value of type s.0100000000000000000000000000000000000000000000000000000000000000.Token",
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
			OnResolveLocation: LocationResolver,
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
		utils.RequireError(t, err)
		assert.ErrorContains(
			t,
			err,
			"failed to ABI decode data",
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
			OnResolveLocation: LocationResolver,
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
		utils.RequireError(t, err)
		assert.ErrorContains(
			t,
			err,
			"failed to ABI decode data",
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
			OnResolveLocation: LocationResolver,
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
		utils.RequireError(t, err)
		assert.ErrorContains(
			t,
			err,
			"failed to ABI decode data with type UFix64",
		)
	})

	t.Run("decodeABI with unsupported dictionary type", func(t *testing.T) {

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
			OnResolveLocation: LocationResolver,
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
		utils.RequireError(t, err)
		assert.ErrorContains(
			t,
			err,
			"failed to ABI decode data with type {Int: Bool}",
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
			OnResolveLocation: LocationResolver,
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
		utils.RequireError(t, err)
		assert.ErrorContains(
			t,
			err,
			"failed to ABI decode data with type [Character]",
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
			OnResolveLocation: LocationResolver,
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
            access(all) var balance: UInt

            init(id: Int, balance: UInt) {
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
		utils.RequireError(t, err)
		assert.ErrorContains(
			t,
			err,
			"failed to ABI decode data with type s.0100000000000000000000000000000000000000000000000000000000000000.Token",
		)
	})
}

func TestEVMEncodeABIWithSignature(t *testing.T) {

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
        // bytes for address 0x7A58c0Be72BE218B41C608b7Fe7C5bB630736C71
        let address = EVM.EVMAddress(
          bytes: [
            122, 88, 192, 190, 114, 190, 33, 139, 65, 198,
            8, 183, 254, 124, 91, 182, 48, 115, 108, 113
          ]
        )

        return EVM.encodeABIWithSignature(
          "withdraw(address,uint256)",
          [address, UInt256(250)]
        )
      }
	`)

	accountCodes := map[common.Location][]byte{}
	var events []cadence.Event

	computation := uint(0)
	runtimeInterface := &TestRuntimeInterface{
		Storage: NewTestLedger(nil, nil),
		OnGetSigningAccounts: func() ([]runtime.Address, error) {
			return []runtime.Address{runtime.Address(contractsAddress)}, nil
		},
		OnResolveLocation: LocationResolver,
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
		OnMeterComputation: func(compKind common.ComputationKind, intensity uint) error {
			if compKind == environment.ComputationKindEVMEncodeABI {
				computation += intensity
			}
			return nil
		},
		OnHash: func(
			data []byte,
			tag string,
			hashAlgorithm runtime.HashAlgorithm,
		) ([]byte, error) {
			return crypto.Keccak256(data), nil
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
		0xf3, 0xfe, 0xf3, 0xa3, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
		0x0, 0x0, 0x0, 0x7a, 0x58, 0xc0, 0xbe, 0x72, 0xbe, 0x21, 0x8b, 0x41,
		0xc6, 0x8, 0xb7, 0xfe, 0x7c, 0x5b, 0xb6, 0x30, 0x73, 0x6c, 0x71, 0x0,
		0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
		0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
		0x0, 0x0, 0xfa,
	}
	cdcBytes := make([]cadence.Value, 0)
	for _, bt := range abiBytes {
		cdcBytes = append(cdcBytes, cadence.UInt8(bt))
	}
	encodedABI := cadence.NewArray(
		cdcBytes,
	).WithType(cadence.NewVariableSizedArrayType(cadence.UInt8Type))

	assert.Equal(t,
		encodedABI,
		result,
	)
	// The method ID is a byte array of length 4
	assert.Equal(t, computation+4, uint(len(cdcBytes)))
}

func TestEVMDecodeABIWithSignature(t *testing.T) {

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
        let values = EVM.decodeABIWithSignature(
          "withdraw(address,uint256)",
          types: [Type<EVM.EVMAddress>(), Type<UInt256>()],
          data: data
        )

        // bytes for address 0x7A58c0Be72BE218B41C608b7Fe7C5bB630736C71
        let address = EVM.EVMAddress(
          bytes: [
            122, 88, 192, 190, 114, 190, 33, 139, 65, 198,
            8, 183, 254, 124, 91, 182, 48, 115, 108, 113
          ]
        )

        assert(values.length == 2)
        assert((values[0] as! EVM.EVMAddress).bytes == address.bytes)
        assert((values[1] as! UInt256) == UInt256(250))

        return true
      }
	`)

	accountCodes := map[common.Location][]byte{}
	var events []cadence.Event

	computation := uint(0)
	runtimeInterface := &TestRuntimeInterface{
		Storage: NewTestLedger(nil, nil),
		OnGetSigningAccounts: func() ([]runtime.Address, error) {
			return []runtime.Address{runtime.Address(contractsAddress)}, nil
		},
		OnResolveLocation: LocationResolver,
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
		OnMeterComputation: func(compKind common.ComputationKind, intensity uint) error {
			if compKind == environment.ComputationKindEVMDecodeABI {
				computation += intensity
			}
			return nil
		},
		OnHash: func(
			data []byte,
			tag string,
			hashAlgorithm runtime.HashAlgorithm,
		) ([]byte, error) {
			return crypto.Keccak256(data), nil
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
		0xf3, 0xfe, 0xf3, 0xa3, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
		0x0, 0x0, 0x0, 0x7a, 0x58, 0xc0, 0xbe, 0x72, 0xbe, 0x21, 0x8b, 0x41,
		0xc6, 0x8, 0xb7, 0xfe, 0x7c, 0x5b, 0xb6, 0x30, 0x73, 0x6c, 0x71, 0x0,
		0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
		0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
		0x0, 0x0, 0xfa,
	}
	cdcBytes := make([]cadence.Value, 0)
	for _, bt := range abiBytes {
		cdcBytes = append(cdcBytes, cadence.UInt8(bt))
	}
	encodedABI := cadence.NewArray(
		cdcBytes,
	).WithType(cadence.NewVariableSizedArrayType(cadence.UInt8Type))

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
	// The method ID is a byte array of length 4
	assert.Equal(t, computation+4, uint(len(cdcBytes)))
}

func TestEVMDecodeABIWithSignatureMismatch(t *testing.T) {

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
        // The data was encoded for the function "withdraw(address,uint256)",
        // but we pass a different function signature
        let values = EVM.decodeABIWithSignature(
          "deposit(uint256, address)",
          types: [Type<UInt256>(), Type<EVM.EVMAddress>()],
          data: data
        )

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
		OnResolveLocation: LocationResolver,
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
		OnHash: func(
			data []byte,
			tag string,
			hashAlgorithm runtime.HashAlgorithm,
		) ([]byte, error) {
			return crypto.Keccak256(data), nil
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
		0xf3, 0xfe, 0xf3, 0xa3, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
		0x0, 0x0, 0x0, 0x7a, 0x58, 0xc0, 0xbe, 0x72, 0xbe, 0x21, 0x8b, 0x41,
		0xc6, 0x8, 0xb7, 0xfe, 0x7c, 0x5b, 0xb6, 0x30, 0x73, 0x6c, 0x71, 0x0,
		0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
		0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
		0x0, 0x0, 0xfa,
	}
	cdcBytes := make([]cadence.Value, 0)
	for _, bt := range abiBytes {
		cdcBytes = append(cdcBytes, cadence.UInt8(bt))
	}
	encodedABI := cadence.NewArray(
		cdcBytes,
	).WithType(cadence.NewVariableSizedArrayType(cadence.UInt8Type))

	_, err := rt.ExecuteScript(
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
	require.Error(t, err)
	assert.ErrorContains(t, err, "panic: signature mismatch")
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
		OnResolveLocation: LocationResolver,
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
		cadence.NewStruct([]cadence.Value{
			addressBytesArray,
		}).WithType(evmAddressCadenceType),
		result,
	)
}

func TestEVMAddressSerializationAndDeserialization(t *testing.T) {

	t.Parallel()

	handler := &testContractHandler{}

	contractsAddress := flow.BytesToAddress([]byte{0x1})

	transactionEnvironment := newEVMTransactionEnvironment(handler, contractsAddress)
	scriptEnvironment := newEVMScriptEnvironment(handler, contractsAddress)

	rt := runtime.NewInterpreterRuntime(runtime.Config{})

	addressFromBytesScript := []byte(`
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
		OnResolveLocation: LocationResolver,
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

	sourceBytes := []byte{
		1, 1, 2, 2, 3, 3, 4, 4, 5, 5,
		6, 6, 7, 7, 8, 8, 9, 9, 10, 10,
	}

	// construct the address as a cadence value from sourceBytes
	addressBytesArray := cadence.NewArray([]cadence.Value{
		cadence.UInt8(sourceBytes[0]), cadence.UInt8(sourceBytes[1]),
		cadence.UInt8(sourceBytes[2]), cadence.UInt8(sourceBytes[3]),
		cadence.UInt8(sourceBytes[4]), cadence.UInt8(sourceBytes[5]),
		cadence.UInt8(sourceBytes[6]), cadence.UInt8(sourceBytes[7]),
		cadence.UInt8(sourceBytes[8]), cadence.UInt8(sourceBytes[9]),
		cadence.UInt8(sourceBytes[10]), cadence.UInt8(sourceBytes[11]),
		cadence.UInt8(sourceBytes[12]), cadence.UInt8(sourceBytes[13]),
		cadence.UInt8(sourceBytes[14]), cadence.UInt8(sourceBytes[15]),
		cadence.UInt8(sourceBytes[16]), cadence.UInt8(sourceBytes[17]),
		cadence.UInt8(sourceBytes[18]), cadence.UInt8(sourceBytes[19]),
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

	constructAddrResult, err := rt.ExecuteScript(
		runtime.Script{
			Source: addressFromBytesScript,
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
	evmAddress := cadence.NewStruct([]cadence.Value{
		addressBytesArray,
	}).WithType(evmAddressCadenceType)

	assert.Equal(t,
		evmAddress,
		constructAddrResult,
	)

	// Attempt to serialize and deserialize the address

	addressSerializationScript := []byte(`
	  import EVM from 0x1

	  access(all)
	  fun main(address: EVM.EVMAddress): String {
		return address.toString()
	  }
	`)

	serializeAddrResult, err := rt.ExecuteScript(
		runtime.Script{
			Source: addressSerializationScript,
			Arguments: EncodeArgs([]cadence.Value{
				evmAddress,
			}),
		},
		runtime.Context{
			Interface:   runtimeInterface,
			Environment: scriptEnvironment,
			Location:    nextScriptLocation(),
		},
	)

	require.NoError(t, err)

	// Encode the sourceBytes array as a hex string as the expected value to compare the result against

	expectedHex, _ := cadence.NewString(hex.EncodeToString(sourceBytes))

	assert.Equal(t,
		expectedHex,
		serializeAddrResult,
	)

	// Attempt to deserialize the address

	addressDeserializationScript := []byte(`
	  import EVM from 0x1

	  access(all)
	  fun main(hexString: String): EVM.EVMAddress {
		return EVM.addressFromString(hexString)
	  }
	`)

	deserializeAddrResult, err := rt.ExecuteScript(
		runtime.Script{
			Source: addressDeserializationScript,
			Arguments: EncodeArgs([]cadence.Value{
				serializeAddrResult,
			}),
		},
		runtime.Context{
			Interface:   runtimeInterface,
			Environment: scriptEnvironment,
			Location:    nextScriptLocation(),
		},
	)

	require.NoError(t, err)

	assert.Equal(t,
		evmAddress,
		deserializeAddrResult,
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
      fun main(_ attoflow: UInt): EVM.Balance {
          return EVM.Balance(attoflow: attoflow)
      }
    `)

	accountCodes := map[common.Location][]byte{}
	var events []cadence.Event

	runtimeInterface := &TestRuntimeInterface{
		Storage: NewTestLedger(nil, nil),
		OnGetSigningAccounts: func() ([]runtime.Address, error) {
			return []runtime.Address{runtime.Address(contractsAddress)}, nil
		},
		OnResolveLocation: LocationResolver,
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

	flowValue := cadence.NewUInt(1230000000000000000)

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
		cadence.NewStruct([]cadence.Value{
			flowValue,
		}).WithType(evmBalanceCadenceType),
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

	contractsAddress := flow.BytesToAddress([]byte{0x1})
	handler := &testContractHandler{
		evmContractAddress: common.Address(contractsAddress),
		run: func(tx []byte, coinbase types.Address) *types.ResultSummary {
			runCalled = true

			assert.Equal(t, []byte{1, 2, 3}, tx)
			assert.Equal(t,
				types.Address{
					1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 6, 7, 7, 8, 8, 9, 9, 10, 10,
				},
				coinbase,
			)
			return &types.ResultSummary{
				Status: types.StatusSuccessful,
			}
		},
	}

	transactionEnvironment := newEVMTransactionEnvironment(handler, contractsAddress)
	scriptEnvironment := newEVMScriptEnvironment(handler, contractsAddress)

	rt := runtime.NewInterpreterRuntime(runtime.Config{})

	script := []byte(`
      import EVM from 0x1

      access(all)
      fun main(tx: [UInt8], coinbaseBytes: [UInt8; 20]): UInt8 {
          let coinbase = EVM.EVMAddress(bytes: coinbaseBytes)
          let res = EVM.run(tx: tx, coinbase: coinbase)
		  let st = res.status
		  return st.rawValue
      }
    `)

	accountCodes := map[common.Location][]byte{}
	var events []cadence.Event

	runtimeInterface := &TestRuntimeInterface{
		Storage: NewTestLedger(nil, nil),
		OnGetSigningAccounts: func() ([]runtime.Address, error) {
			return []runtime.Address{runtime.Address(contractsAddress)}, nil
		},
		OnResolveLocation: LocationResolver,
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

	val, err := rt.ExecuteScript(
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

	assert.Equal(t, types.StatusSuccessful, types.Status(val.(cadence.UInt8)))
	assert.True(t, runCalled)

	// test must run
	script = []byte(`
		import EVM from 0x1

		access(all)
		fun main(tx: [UInt8], coinbaseBytes: [UInt8; 20]): UInt8 {
			let coinbase = EVM.EVMAddress(bytes: coinbaseBytes)
			let res = EVM.mustRun(tx: tx, coinbase: coinbase)
			let st = res.status
			return st.rawValue
		}
  	`)
	val, err = rt.ExecuteScript(
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

	assert.Equal(t, types.StatusSuccessful, types.Status(val.(cadence.UInt8)))
	assert.True(t, runCalled)
}

func TestEVMDryRun(t *testing.T) {

	t.Parallel()

	dryRunCalled := false
	evmTx := cadence.NewArray([]cadence.Value{
		cadence.UInt8(1),
		cadence.UInt8(2),
		cadence.UInt8(3),
	}).WithType(stdlib.EVMTransactionBytesCadenceType)

	contractsAddress := flow.BytesToAddress([]byte{0x1})
	handler := &testContractHandler{
		evmContractAddress: common.Address(contractsAddress),
		dryRun: func(tx []byte, from types.Address) *types.ResultSummary {
			dryRunCalled = true
			assert.Equal(t, types.Address{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}, from)
			assert.Equal(t, tx, []byte{1, 2, 3})

			return &types.ResultSummary{
				Status: types.StatusSuccessful,
			}
		},
	}

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
		OnResolveLocation: LocationResolver,
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
      fun main(tx: [UInt8]): EVM.Result {
          return EVM.dryRun(
			tx: tx,
			from: EVM.EVMAddress(bytes: [0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19]), // random address 
          )
      }
    `)

	val, err := rt.ExecuteScript(
		runtime.Script{
			Source:    script,
			Arguments: EncodeArgs([]cadence.Value{evmTx}),
		},
		runtime.Context{
			Interface:   runtimeInterface,
			Environment: scriptEnvironment,
			Location:    nextScriptLocation(),
		},
	)
	require.NoError(t, err)
	res, err := stdlib.ResultSummaryFromEVMResultValue(val)
	require.NoError(t, err)
	assert.Equal(t, types.StatusSuccessful, res.Status)
	assert.True(t, dryRunCalled)
}

func TestEVMBatchRun(t *testing.T) {

	t.Parallel()

	evmTxs := cadence.NewArray([]cadence.Value{
		cadence.NewArray([]cadence.Value{cadence.UInt8(1), cadence.UInt8(2), cadence.UInt8(3)}),
		cadence.NewArray([]cadence.Value{cadence.UInt8(4), cadence.UInt8(5), cadence.UInt8(6)}),
		cadence.NewArray([]cadence.Value{cadence.UInt8(7), cadence.UInt8(8), cadence.UInt8(9)}),
	}).WithType(cadence.NewVariableSizedArrayType(cadence.NewVariableSizedArrayType(cadence.UInt8Type)))

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

	contractsAddress := flow.BytesToAddress([]byte{0x1})
	handler := &testContractHandler{
		evmContractAddress: common.Address(contractsAddress),
		batchRun: func(txs [][]byte, coinbase types.Address) []*types.ResultSummary {
			runCalled = true

			assert.EqualValues(t, [][]byte{
				{1, 2, 3}, {4, 5, 6}, {7, 8, 9},
			}, txs)
			assert.Equal(t,
				types.Address{
					1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 6, 7, 7, 8, 8, 9, 9, 10, 10,
				},
				coinbase,
			)

			results := make([]*types.ResultSummary, 3)
			for i := range results {
				results[i] = &types.ResultSummary{
					Status: types.StatusSuccessful,
				}
			}

			return results
		},
	}

	transactionEnvironment := newEVMTransactionEnvironment(handler, contractsAddress)
	scriptEnvironment := newEVMScriptEnvironment(handler, contractsAddress)

	rt := runtime.NewInterpreterRuntime(runtime.Config{})

	script := []byte(`
      import EVM from 0x1

      access(all)
      fun main(txs: [[UInt8]], coinbaseBytes: [UInt8; 20]): [EVM.Result] {
          let coinbase = EVM.EVMAddress(bytes: coinbaseBytes)
          return EVM.batchRun(txs: txs, coinbase: coinbase)
      }
    `)

	accountCodes := map[common.Location][]byte{}
	var events []cadence.Event

	runtimeInterface := &TestRuntimeInterface{
		Storage: NewTestLedger(nil, nil),
		OnGetSigningAccounts: func() ([]runtime.Address, error) {
			return []runtime.Address{runtime.Address(contractsAddress)}, nil
		},
		OnResolveLocation: LocationResolver,
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

	val, err := rt.ExecuteScript(
		runtime.Script{
			Source:    script,
			Arguments: EncodeArgs([]cadence.Value{evmTxs, coinbase}),
		},
		runtime.Context{
			Interface:   runtimeInterface,
			Environment: scriptEnvironment,
			Location:    nextScriptLocation(),
		},
	)
	require.NoError(t, err)

	resultsCadence, ok := val.(cadence.Array)
	require.True(t, ok)

	for _, v := range resultsCadence.Values {
		res, err := stdlib.ResultSummaryFromEVMResultValue(v)
		require.NoError(t, err)
		assert.Equal(t, types.StatusSuccessful, res.Status)
	}
	assert.True(t, runCalled)
}

func TestEVMCreateCadenceOwnedAccount(t *testing.T) {

	t.Parallel()

	uuidCounter := uint64(0)
	handler := &testContractHandler{
		deployCOA: func(uuid uint64) types.Address {
			require.Equal(t, uuidCounter, uuid)
			return types.Address{uint8(uuidCounter)}
		},
	}

	contractsAddress := flow.BytesToAddress([]byte{0x1})

	transactionEnvironment := newEVMTransactionEnvironment(handler, contractsAddress)
	scriptEnvironment := newEVMScriptEnvironment(handler, contractsAddress)
	rt := runtime.NewInterpreterRuntime(runtime.Config{})

	script := []byte(`
      import EVM from 0x1

      access(all)
      fun main(): [UInt8; 20] {
          let cadenceOwnedAccount1 <- EVM.createCadenceOwnedAccount()
          destroy cadenceOwnedAccount1

          let cadenceOwnedAccount2 <- EVM.createCadenceOwnedAccount()
          let bytes = cadenceOwnedAccount2.address().bytes
          destroy cadenceOwnedAccount2

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
		OnResolveLocation: LocationResolver,
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
		OnGenerateUUID: func() (uint64, error) {
			uuidCounter++
			return uuidCounter, nil
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

	// reset events
	events = make([]cadence.Event, 0)

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
		cadence.UInt8(4), cadence.UInt8(0),
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
		cadence.UInt8Type,
	))

	require.Equal(t, expected, actual)

	// check deposit event
	expectedEventTypes := []string{
		"EVM.CadenceOwnedAccountCreated",
		"EVM.CadenceOwnedAccountCreated",
	}
	CheckCadenceEventTypes(t, events, expectedEventTypes)

	// check cadence owned account created events
	expectedCoaAddress := types.Address{3}
	require.Equal(t,
		expectedCoaAddress.ToCadenceValue(),
		cadence.SearchFieldByName(
			events[0],
			types.CadenceOwnedAccountCreatedTypeAddressBytesFieldName,
		),
	)

	expectedCoaAddress = types.Address{4}
	require.Equal(t,
		expectedCoaAddress.ToCadenceValue(),
		cadence.SearchFieldByName(
			events[1],
			types.CadenceOwnedAccountCreatedTypeAddressBytesFieldName,
		),
	)
}

func TestCadenceOwnedAccountCall(t *testing.T) {

	t.Parallel()

	expectedBalance, err := cadence.NewUFix64FromParts(1, 23000000)
	require.NoError(t, err)

	contractsAddress := flow.BytesToAddress([]byte{0x1})

	handler := &testContractHandler{
		evmContractAddress: common.Address(contractsAddress),
		accountByAddress: func(fromAddress types.Address, isAuthorized bool) types.Account {
			assert.Equal(t, types.Address{3, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, fromAddress)
			assert.True(t, isAuthorized)

			return &testFlowAccount{
				address: fromAddress,
				call: func(
					toAddress types.Address,
					data types.Data,
					limit types.GasLimit,
					balance types.Balance,
				) *types.ResultSummary {
					assert.Equal(t, types.Address{2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, toAddress)
					assert.Equal(t, types.Data{4, 5, 6}, data)
					assert.Equal(t, types.GasLimit(9999), limit)
					assert.Equal(t, types.NewBalanceFromUFix64(expectedBalance), balance)

					return &types.ResultSummary{
						Status:        types.StatusSuccessful,
						ReturnedValue: types.Data{3, 1, 4},
					}
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
      fun main(): [UInt8] {
          let cadenceOwnedAccount <- EVM.createCadenceOwnedAccount()
		  let bal = EVM.Balance(attoflow: 0)
		  bal.setFLOW(flow: 1.23)
          let response = cadenceOwnedAccount.call(
              to: EVM.EVMAddress(
                  bytes: [2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]
              ),
              data: [4, 5, 6],
              gasLimit: 9999,
              value: bal
          )
          destroy cadenceOwnedAccount
          return response.data
      }
   `)

	accountCodes := map[common.Location][]byte{}
	var events []cadence.Event

	runtimeInterface := &TestRuntimeInterface{
		Storage: NewTestLedger(nil, nil),
		OnGetSigningAccounts: func() ([]runtime.Address, error) {
			return []runtime.Address{runtime.Address(contractsAddress)}, nil
		},
		OnResolveLocation: LocationResolver,
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
	}).WithType(cadence.NewVariableSizedArrayType(cadence.UInt8Type))

	require.Equal(t, expected, actual)
}

func TestEVMAddressDeposit(t *testing.T) {

	t.Parallel()

	expectedBalanceInUFix64, err := cadence.NewUFix64FromParts(1, 23000000)
	require.NoError(t, err)
	expectedBalance := types.NewBalanceFromUFix64(expectedBalanceInUFix64)

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
						expectedBalance,
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
          let admin = getAuthAccount<auth(Storage) &Account>(0x1)
              .storage.borrow<&FlowToken.Administrator>(from: /storage/flowTokenAdmin)!
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
		OnResolveLocation: LocationResolver,
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

func TestCOADeposit(t *testing.T) {

	t.Parallel()

	expectedBalance, err := cadence.NewUFix64FromParts(1, 23000000)
	require.NoError(t, err)

	var deposited bool

	var expectedCoaAddress = types.Address{5, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	handler := &testContractHandler{

		accountByAddress: func(fromAddress types.Address, isAuthorized bool) types.Account {
			assert.Equal(t, expectedCoaAddress, fromAddress)
			assert.False(t, isAuthorized)

			return &testFlowAccount{
				address: fromAddress,
				deposit: func(vault *types.FLOWTokenVault) {
					deposited = true
					assert.Equal(
						t,
						types.NewBalanceFromUFix64(expectedBalance),
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
          let admin = getAuthAccount<auth(BorrowValue) &Account>(0x1)
              .storage.borrow<&FlowToken.Administrator>(from: /storage/flowTokenAdmin)!
          let minter <- admin.createNewMinter(allowedAmount: 1.23)
          let vault <- minter.mintTokens(amount: 1.23)
          destroy minter

          let cadenceOwnedAccount <- EVM.createCadenceOwnedAccount()
          cadenceOwnedAccount.deposit(from: <-vault)
		  destroy cadenceOwnedAccount
      }
   `)

	accountCodes := map[common.Location][]byte{}
	var events []cadence.Event

	runtimeInterface := &TestRuntimeInterface{
		Storage: NewTestLedger(nil, nil),
		OnGetSigningAccounts: func() ([]runtime.Address, error) {
			return []runtime.Address{runtime.Address(contractsAddress)}, nil
		},
		OnResolveLocation: LocationResolver,
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

	// reset events before script execution
	events = make([]cadence.Event, 0)

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

	// check deposit event
	expectedEventTypes := []string{
		"FlowToken.MinterCreated",
		"FlowToken.TokensMinted",
		"EVM.CadenceOwnedAccountCreated",
		"EVM.FLOWTokensDeposited",
	}
	CheckCadenceEventTypes(t, events, expectedEventTypes)

	// token deposit event
	tokenDepositEvent := events[3]
	tokenDepositEventFields := cadence.FieldsMappedByName(tokenDepositEvent)

	// check address
	require.Equal(t,
		expectedCoaAddress.ToCadenceValue(),
		tokenDepositEventFields["addressBytes"],
	)
	// check amount
	require.Equal(t,
		expectedBalance,
		tokenDepositEventFields["amount"],
	)
}

func TestCadenceOwnedAccountWithdraw(t *testing.T) {

	t.Parallel()

	expectedDepositBalance, err := cadence.NewUFix64FromParts(2, 34000000)
	require.NoError(t, err)

	expectedWithdrawBalance, err := cadence.NewUFix64FromParts(1, 23000000)
	require.NoError(t, err)

	var deposited bool
	var withdrew bool

	contractsAddress := flow.BytesToAddress([]byte{0x1})

	var nextUUID uint64 = 1

	var expectedCoaAddress = types.Address{5, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	handler := &testContractHandler{
		flowTokenAddress: common.Address(contractsAddress),
		accountByAddress: func(fromAddress types.Address, isAuthorized bool) types.Account {
			assert.Equal(t, expectedCoaAddress, fromAddress)
			assert.Equal(t, deposited, isAuthorized)

			return &testFlowAccount{
				address: fromAddress,
				deposit: func(vault *types.FLOWTokenVault) {
					deposited = true
					assert.Equal(t,
						types.NewBalanceFromUFix64(expectedDepositBalance),
						vault.Balance(),
					)
				},
				withdraw: func(balance types.Balance) *types.FLOWTokenVault {
					assert.Equal(t,
						types.NewBalanceFromUFix64(expectedWithdrawBalance),
						balance,
					)
					withdrew = true
					return types.NewFlowTokenVault(balance)
				},
			}
		},
		generateResourceUUID: func() uint64 {
			uuid := nextUUID
			nextUUID++
			return uuid
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
          let admin = getAuthAccount<auth(BorrowValue) &Account>(0x1)
              .storage.borrow<&FlowToken.Administrator>(from: /storage/flowTokenAdmin)!
          let minter <- admin.createNewMinter(allowedAmount: 2.34)
          let vault <- minter.mintTokens(amount: 2.34)
          destroy minter

          let cadenceOwnedAccount <- EVM.createCadenceOwnedAccount()
          cadenceOwnedAccount.deposit(from: <-vault)

          let vault2 <- cadenceOwnedAccount.withdraw(balance: EVM.Balance(attoflow: 1230000000000000000))
          let balance = vault2.balance
          log(vault2.uuid)

          destroy cadenceOwnedAccount
          destroy vault2

          return balance
      }
   `)

	accountCodes := map[common.Location][]byte{}
	var events []cadence.Event
	var logs []string

	runtimeInterface := &TestRuntimeInterface{
		Storage: NewTestLedger(nil, nil),
		OnGetSigningAccounts: func() ([]runtime.Address, error) {
			return []runtime.Address{runtime.Address(contractsAddress)}, nil
		},
		OnResolveLocation: LocationResolver,
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
		OnProgramLog: func(s string) {
			logs = append(logs, s)
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

	// reset events
	events = make([]cadence.Event, 0)
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

	assert.Equal(t, []string{"1"}, logs)

	// check deposit event
	expectedEventTypes := []string{
		"FlowToken.MinterCreated",
		"FlowToken.TokensMinted",
		"EVM.CadenceOwnedAccountCreated",
		"EVM.FLOWTokensDeposited",
		"EVM.FLOWTokensWithdrawn",
	}
	CheckCadenceEventTypes(t, events, expectedEventTypes)

	// token deposit event
	tokenWithdrawEvent := events[4]
	tokenWithdrawEventFields := cadence.FieldsMappedByName(tokenWithdrawEvent)

	// check address
	require.Equal(t,
		expectedCoaAddress.ToCadenceValue(),
		tokenWithdrawEventFields["addressBytes"],
	)
	// check amount
	require.Equal(t,
		expectedWithdrawBalance,
		tokenWithdrawEventFields["amount"],
	)
}

func TestCadenceOwnedAccountDeploy(t *testing.T) {

	t.Parallel()

	var deployed bool

	contractsAddress := flow.BytesToAddress([]byte{0x1})

	expectedBalance, err := cadence.NewUFix64FromParts(1, 23000000)
	require.NoError(t, err)

	handler := &testContractHandler{
		evmContractAddress: common.Address(contractsAddress),
		accountByAddress: func(fromAddress types.Address, isAuthorized bool) types.Account {
			assert.Equal(t, types.Address{3, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, fromAddress)
			assert.True(t, isAuthorized)

			return &testFlowAccount{
				address: fromAddress,
				deploy: func(code types.Code, limit types.GasLimit, balance types.Balance) *types.ResultSummary {
					deployed = true
					assert.Equal(t, types.Code{4, 5, 6}, code)
					assert.Equal(t, types.GasLimit(9999), limit)
					assert.Equal(t, types.NewBalanceFromUFix64(expectedBalance), balance)

					return &types.ResultSummary{
						Status:                  types.StatusSuccessful,
						DeployedContractAddress: &types.Address{4},
						ReturnedValue:           types.Data{5},
					}
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
      fun main(): [UInt8] {
          let cadenceOwnedAccount <- EVM.createCadenceOwnedAccount()
          let res = cadenceOwnedAccount.deploy(
              code: [4, 5, 6],
              gasLimit: 9999,
              value: EVM.Balance(attoflow: 1230000000000000000)
          )
          destroy cadenceOwnedAccount

          assert(res.deployedContract?.bytes == [4, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0])
          return res.data
      }
   `)

	accountCodes := map[common.Location][]byte{}
	var events []cadence.Event

	runtimeInterface := &TestRuntimeInterface{
		Storage: NewTestLedger(nil, nil),
		OnGetSigningAccounts: func() ([]runtime.Address, error) {
			return []runtime.Address{runtime.Address(contractsAddress)}, nil
		},
		OnResolveLocation: LocationResolver,
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

	expected := cadence.
		NewArray([]cadence.Value{cadence.UInt8(5)}).
		WithType(cadence.NewVariableSizedArrayType(cadence.UInt8Type))

	require.Equal(t, expected, actual)

	require.True(t, deployed)
}

func RunEVMScript(
	t *testing.T,
	handler *testContractHandler,
	script []byte,
	expectedValue cadence.Value,
) {
	contractsAddress := flow.Address(handler.evmContractAddress)
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
		OnResolveLocation: LocationResolver,
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

	require.Equal(t, expectedValue, actual)
}

func TestEVMAccountBalance(t *testing.T) {
	t.Parallel()

	contractsAddress := flow.BytesToAddress([]byte{0x1})
	expectedBalanceValue := cadence.NewUInt(1013370000000000000)
	expectedBalance := cadence.
		NewStruct([]cadence.Value{expectedBalanceValue}).
		WithType(stdlib.NewBalanceCadenceType(common.Address(contractsAddress)))

	handler := &testContractHandler{
		flowTokenAddress:   common.Address(contractsAddress),
		evmContractAddress: common.Address(contractsAddress),
		accountByAddress: func(fromAddress types.Address, isAuthorized bool) types.Account {
			assert.Equal(t, types.Address{3, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, fromAddress)
			assert.False(t, isAuthorized)

			return &testFlowAccount{
				address: fromAddress,
				balance: func() types.Balance {
					return types.NewBalance(expectedBalanceValue.Value)
				},
			}
		},
	}

	script := []byte(`
	import EVM from 0x1

	access(all)
	fun main(): EVM.Balance {
			let cadenceOwnedAccount <- EVM.createCadenceOwnedAccount()
			let balance = cadenceOwnedAccount.balance()
			destroy cadenceOwnedAccount
			return balance
		}
	`)
	RunEVMScript(t, handler, script, expectedBalance)
}

func TestEVMAccountNonce(t *testing.T) {
	t.Parallel()

	contractsAddress := flow.BytesToAddress([]byte{0x1})
	expectedNonceValue := cadence.NewUInt64(2000)
	handler := &testContractHandler{
		flowTokenAddress:   common.Address(contractsAddress),
		evmContractAddress: common.Address(contractsAddress),
		accountByAddress: func(fromAddress types.Address, isAuthorized bool) types.Account {
			assert.Equal(t, types.Address{3, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, fromAddress)
			assert.False(t, isAuthorized)

			return &testFlowAccount{
				address: fromAddress,
				nonce: func() uint64 {
					return uint64(expectedNonceValue)
				},
			}
		},
	}

	script := []byte(`
      import EVM from 0x1

      access(all)
      fun main(): UInt64 {
          let cadenceOwnedAccount <- EVM.createCadenceOwnedAccount()
          let nonce = cadenceOwnedAccount.address().nonce()
          destroy cadenceOwnedAccount
          return nonce
      }
    `)

	RunEVMScript(t, handler, script, expectedNonceValue)
}

func TestEVMAccountCode(t *testing.T) {
	t.Parallel()

	contractsAddress := flow.BytesToAddress([]byte{0x1})
	expectedCodeRaw := []byte{1, 2, 3}
	expectedCodeValue := cadence.NewArray(
		[]cadence.Value{cadence.UInt8(1), cadence.UInt8(2), cadence.UInt8(3)},
	).WithType(cadence.NewVariableSizedArrayType(cadence.UInt8Type))

	handler := &testContractHandler{
		flowTokenAddress:   common.Address(contractsAddress),
		evmContractAddress: common.Address(contractsAddress),
		accountByAddress: func(fromAddress types.Address, isAuthorized bool) types.Account {
			assert.Equal(t, types.Address{3, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, fromAddress)
			assert.False(t, isAuthorized)

			return &testFlowAccount{
				address: fromAddress,
				code: func() types.Code {
					return expectedCodeRaw
				},
			}
		},
	}

	script := []byte(`
      import EVM from 0x1

      access(all)
      fun main(): [UInt8] {
          let cadenceOwnedAccount <- EVM.createCadenceOwnedAccount()
          let code = cadenceOwnedAccount.address().code()
          destroy cadenceOwnedAccount
          return code
      }
    `)

	RunEVMScript(t, handler, script, expectedCodeValue)
}

func TestEVMAccountCodeHash(t *testing.T) {
	t.Parallel()

	contractsAddress := flow.BytesToAddress([]byte{0x1})
	expectedCodeHashRaw := []byte{1, 2, 3}
	expectedCodeHashValue := cadence.NewArray(
		[]cadence.Value{cadence.UInt8(1), cadence.UInt8(2), cadence.UInt8(3)},
	).WithType(cadence.NewVariableSizedArrayType(cadence.UInt8Type))

	handler := &testContractHandler{
		flowTokenAddress:   common.Address(contractsAddress),
		evmContractAddress: common.Address(contractsAddress),
		accountByAddress: func(fromAddress types.Address, isAuthorized bool) types.Account {
			assert.Equal(t, types.Address{3, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, fromAddress)
			assert.False(t, isAuthorized)

			return &testFlowAccount{
				address: fromAddress,
				codeHash: func() []byte {
					return expectedCodeHashRaw
				},
			}
		},
	}

	script := []byte(`
      import EVM from 0x1

      access(all)
      fun main(): [UInt8] {
          let cadenceOwnedAccount <- EVM.createCadenceOwnedAccount()
          let codeHash = cadenceOwnedAccount.address().codeHash()
          destroy cadenceOwnedAccount
          return codeHash
      }
    `)

	RunEVMScript(t, handler, script, expectedCodeHashValue)
}

func TestEVMValidateCOAOwnershipProof(t *testing.T) {
	t.Parallel()

	contractsAddress := flow.BytesToAddress([]byte{0x1})

	proof := &types.COAOwnershipProofInContext{
		COAOwnershipProof: types.COAOwnershipProof{
			Address:        types.FlowAddress(contractsAddress),
			CapabilityPath: "coa",
			Signatures:     []types.Signature{[]byte("signature")},
			KeyIndices:     []uint64{0},
		},
		SignedData: []byte("signedData"),
		EVMAddress: RandomAddress(t),
	}

	handler := &testContractHandler{
		deployCOA: func(_ uint64) types.Address {
			return proof.EVMAddress
		},
	}
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
		OnResolveLocation: LocationResolver,
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
		OnGetAccountKey: func(addr runtime.Address, index int) (*cadenceStdlib.AccountKey, error) {
			require.Equal(t, proof.Address[:], addr[:])
			return &cadenceStdlib.AccountKey{
				PublicKey: &cadenceStdlib.PublicKey{},
				KeyIndex:  index,
				Weight:    100,
				HashAlgo:  sema.HashAlgorithmKECCAK_256,
				IsRevoked: false,
			}, nil
		},
		OnVerifySignature: func(
			signature []byte,
			tag string,
			sd,
			publicKey []byte,
			signatureAlgorithm runtime.SignatureAlgorithm,
			hashAlgorithm runtime.HashAlgorithm) (bool, error) {
			// require.Equal(t, []byte(signedData.ToGoValue()), st)
			return true, nil
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

	setupTx := []byte(`
		import EVM from 0x1

		transaction {
			prepare(account: auth(Capabilities, SaveValue) &Account) {
				let cadenceOwnedAccount <- EVM.createCadenceOwnedAccount()

				account.storage.save(
				    <-cadenceOwnedAccount,
					to: /storage/coa
				)

				let cap = account.capabilities.storage
				    .issue<&EVM.CadenceOwnedAccount>(/storage/coa)
				account.capabilities.publish(cap, at: /public/coa)
			}
		}`)

	err := rt.ExecuteTransaction(
		runtime.Script{
			Source: setupTx,
		},
		runtime.Context{
			Interface:   runtimeInterface,
			Environment: transactionEnvironment,
			Location:    nextTransactionLocation(),
		},
	)
	require.NoError(t, err)

	script := []byte(`
      import EVM from 0x1

      access(all)
      fun main(
		  address: Address,
		  path: PublicPath,
		  signedData: [UInt8],
		  keyIndices: [UInt64],
		  signatures: [[UInt8]],
		  evmAddress: [UInt8; 20]

		) {
          EVM.validateCOAOwnershipProof(
			address: address,
			path: path,
			signedData: signedData,
			keyIndices: keyIndices,
			signatures: signatures, 
			evmAddress: evmAddress
		  )
      }
    `)

	// Run script
	_, err = rt.ExecuteScript(
		runtime.Script{
			Source:    script,
			Arguments: EncodeArgs(proof.ToCadenceValues()),
		},
		runtime.Context{
			Interface:   runtimeInterface,
			Environment: scriptEnvironment,
			Location:    nextScriptLocation(),
		},
	)
	require.NoError(t, err)
}

func TestInternalEVMAccess(t *testing.T) {

	t.Parallel()

	handler := &testContractHandler{}

	contractsAddress := flow.BytesToAddress([]byte{0x1})
	transactionEnvironment := newEVMTransactionEnvironment(handler, contractsAddress)
	scriptEnvironment := newEVMScriptEnvironment(handler, contractsAddress)
	rt := runtime.NewInterpreterRuntime(runtime.Config{})

	script := []byte(`
      import EVM from 0x1

      access(all)
      fun main() {
          let a = InternalEVM.createBridgedAccount()
      }
    `)

	accountCodes := map[common.Location][]byte{}

	runtimeInterface := &TestRuntimeInterface{
		Storage: NewTestLedger(nil, nil),
		OnGetSigningAccounts: func() ([]runtime.Address, error) {
			return []runtime.Address{runtime.Address(contractsAddress)}, nil
		},
		OnResolveLocation: LocationResolver,
		OnUpdateAccountContractCode: func(location common.AddressLocation, code []byte) error {
			accountCodes[location] = code
			return nil
		},
		OnGetAccountContractCode: func(location common.AddressLocation) (code []byte, err error) {
			code = accountCodes[location]
			return code, nil
		},
		OnEmitEvent: func(event cadence.Event) error {
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
			Source: script,
		},
		runtime.Context{
			Interface:   runtimeInterface,
			Environment: scriptEnvironment,
			Location:    nextScriptLocation(),
		},
	)
	require.Error(t, err)
}

func TestEVMGetLatestBlock(t *testing.T) {
	t.Parallel()

	contractsAddress := flow.BytesToAddress([]byte{0x1})

	latestBlock := &types.Block{
		Height:      uint64(2),
		TotalSupply: big.NewInt(1500000000000000000),
		Timestamp:   uint64(1337),
	}
	handler := &testContractHandler{
		evmContractAddress: common.Address(contractsAddress),
		lastExecutedBlock: func() *types.Block {
			return latestBlock
		},
	}

	script := []byte(`
      import EVM from 0x1

      access(all)
      fun main(): EVM.EVMBlock {
        return EVM.getLatestBlock()
      }
	`)

	evmBlockCadenceType := stdlib.NewEVMBlockCadenceType(
		common.Address(contractsAddress),
	)

	blockHeight := cadence.NewUInt64(latestBlock.Height)
	hash, err := latestBlock.Hash()
	require.NoError(t, err)
	blockHash, err := cadence.NewString(hash.Hex())
	require.NoError(t, err)
	blockTotalSupply := cadence.NewIntFromBig(latestBlock.TotalSupply)
	timestamp := cadence.NewUInt64(latestBlock.Timestamp)

	expectedEVMBlock := cadence.NewStruct([]cadence.Value{
		blockHeight,
		blockHash,
		blockTotalSupply,
		timestamp,
	}).WithType(evmBlockCadenceType)

	RunEVMScript(t, handler, script, expectedEVMBlock)
}
