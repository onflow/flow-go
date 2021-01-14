package fvm

import (
	"bytes"
	"fmt"
	"sort"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

const TopShotContractAddress = "0b2a3299cc857e29"

// Runtime is Cadence interface hence it would be hard to mock it
// implementation for our needs is simple enough though
type mockRuntime struct {
	executeTxResult error
}

func (m mockRuntime) SetCoverageReport(_ *runtime.CoverageReport) {
	panic("should not be used")
}

func (m mockRuntime) ExecuteScript(_ runtime.Script, _ runtime.Context) (cadence.Value, error) {
	panic("should not be used")
}

func (m mockRuntime) ExecuteTransaction(_ runtime.Script, _ runtime.Context) error {
	return m.executeTxResult
}

func TestSafetyCheck(t *testing.T) {

	// We are now logging for all failed Tx
	// t.Run("logs for failing tx but not related to TopShot", func(t *testing.T) {

	// 	runtimeError := runtime.Error{
	// 		Err: sema.CheckerError{
	// 			Errors: []error{&sema.ImportedProgramError{
	// 				CheckerError: &sema.CheckerError{},
	// 				ImportLocation: ast.AddressLocation{
	// 					Name:    "NotTopshot",
	// 					Address: common.BytesToAddress([]byte{0, 1, 2}),
	// 				},
	// 			}},
	// 		},
	// 	}

	// 	err, buffer := prepare(runtimeError)
	// 	require.Error(t, err)
	// 	require.Equal(t, 0, buffer.Len())
	// })

	t.Run("parsing error in imported contract", func(t *testing.T) {

		rt := runtime.NewInterpreterRuntime()

		buffer := &bytes.Buffer{}
		log := zerolog.New(buffer)
		txInvocator := NewTransactionInvocator(log)

		vm := New(rt)

		code := `
			import 0x0b2a3299cc857e29
			transaction {}
		`

		proc := Transaction(&flow.TransactionBody{Script: []byte(code)}, 0)

		contractAddress := flow.HexToAddress("0b2a3299cc857e29")

		ledger := state.NewMapLedger()

		contractCode := `X`

		// TODO: refactor this manual deployment by setting ledger keys
		//   into a proper deployment of the contract

		encodedName, err := encodeContractNames([]string{"TestContract"})
		require.NoError(t, err)

		ledger.Set(
			string(contractAddress.Bytes()),
			string(contractAddress.Bytes()),
			"contract_names",
			encodedName,
		)

		ledger.Set(
			string(contractAddress.Bytes()),
			string(contractAddress.Bytes()),
			"code.TestContract",
			[]byte(contractCode),
		)

		context := NewContext(log)

		err = txInvocator.Process(vm, context, proc, ledger)
		require.Error(t, err)

		require.Contains(t, buffer.String(), "programs")
		require.Contains(t, buffer.String(), "codes")
	})

	t.Run("checking error in imported contract", func(t *testing.T) {

		rt := runtime.NewInterpreterRuntime()

		buffer := &bytes.Buffer{}
		log := zerolog.New(buffer)
		txInvocator := NewTransactionInvocator(log)

		vm := New(rt)

		code := `
			import 0x0b2a3299cc857e29
			transaction {}
		`

		proc := Transaction(&flow.TransactionBody{Script: []byte(code)}, 0)

		contractAddress := flow.HexToAddress("0b2a3299cc857e29")

		ledger := state.NewMapLedger()

		contractCode := `pub contract TestContract: X {}`

		// TODO: refactor this manual deployment by setting ledger keys
		//   into a proper deployment of the contract

		encodedName, err := encodeContractNames([]string{"TestContract"})
		require.NoError(t, err)

		ledger.Set(
			string(contractAddress.Bytes()),
			string(contractAddress.Bytes()),
			"contract_names",
			encodedName,
		)
		ledger.Set(
			string(contractAddress.Bytes()),
			string(contractAddress.Bytes()),
			"code.TestContract",
			[]byte(contractCode),
		)

		context := NewContext(log)

		err = txInvocator.Process(vm, context, proc, ledger)
		require.Error(t, err)

		require.Contains(t, buffer.String(), "programs")
		require.Contains(t, buffer.String(), "codes")
	})

	t.Run("parsing error in transaction", func(t *testing.T) {

		rt := runtime.NewInterpreterRuntime()

		buffer := &bytes.Buffer{}
		log := zerolog.New(buffer)
		txInvocator := NewTransactionInvocator(log)

		vm := New(rt)

		code := `X`

		proc := Transaction(&flow.TransactionBody{Script: []byte(code)}, 0)

		ledger := state.NewMapLedger()
		context := NewContext(log)

		err := txInvocator.Process(vm, context, proc, ledger)
		require.Error(t, err)

		require.NotContains(t, buffer.String(), "programs")
		require.NotContains(t, buffer.String(), "codes")
	})

	t.Run("checking error in transaction", func(t *testing.T) {

		rt := runtime.NewInterpreterRuntime()

		buffer := &bytes.Buffer{}
		log := zerolog.New(buffer)
		txInvocator := NewTransactionInvocator(log)

		vm := New(rt)

		code := `transaction(arg: X) { }`

		proc := Transaction(&flow.TransactionBody{Script: []byte(code)}, 0)

		ledger := state.NewMapLedger()
		context := NewContext(log)

		err := txInvocator.Process(vm, context, proc, ledger)
		require.Error(t, err)

		require.NotContains(t, buffer.String(), "programs")
		require.NotContains(t, buffer.String(), "codes")
	})
}

func encodeContractNames(contractNames []string) ([]byte, error) {
	sort.Strings(contractNames)
	var buf bytes.Buffer
	cborEncoder := cbor.NewEncoder(&buf)
	err := cborEncoder.Encode(contractNames)
	if err != nil {
		return nil, fmt.Errorf("cannot encode contract names")
	}
	return buf.Bytes(), nil
}
