package fvm_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"sort"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/sema"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/extralog"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/fvm/utils"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestSafetyCheck(t *testing.T) {

	t.Run("parsing error in imported contract", func(t *testing.T) {

		// temporary solution
		dumpPath := extralog.ExtraLogDumpPath

		unittest.RunWithTempDir(t, func(tmpDir string) {

			extralog.ExtraLogDumpPath = tmpDir

			rt := fvm.NewInterpreterRuntime()

			buffer := &bytes.Buffer{}
			log := zerolog.New(buffer)
			txInvoker := fvm.NewTransactionInvoker(log)

			vm := fvm.NewVirtualMachine(rt)

			code := `
				import 0x0b2a3299cc857e29

				transaction {}
			`

			proc := fvm.Transaction(&flow.TransactionBody{Script: []byte(code)}, 0)

			contractAddress := flow.HexToAddress("0b2a3299cc857e29")

			view := utils.NewSimpleView()

			contractCode := `X`

			// TODO: refactor this manual deployment by setting ledger keys
			//   into a proper deployment of the contract

			encodedName, err := encodeContractNames([]string{"TestContract"})
			require.NoError(t, err)

			err = view.Set(
				string(contractAddress.Bytes()),
				string(contractAddress.Bytes()),
				"contract_names",
				encodedName,
			)
			require.NoError(t, err)
			err = view.Set(
				string(contractAddress.Bytes()),
				string(contractAddress.Bytes()),
				"code.TestContract",
				[]byte(contractCode),
			)
			require.NoError(t, err)

			context := fvm.NewContext(log)

			sth := state.NewStateHolder(state.NewState(
				view,
				state.WithMaxKeySizeAllowed(context.MaxStateKeySize),
				state.WithMaxValueSizeAllowed(context.MaxStateValueSize),
				state.WithMaxInteractionSizeAllowed(context.MaxStateInteractionSize),
			))

			err = txInvoker.Process(vm, &context, proc, sth, programs.NewEmptyPrograms())
			require.Error(t, err)

			require.Contains(t, buffer.String(), "programs")
			require.Contains(t, buffer.String(), "codes")
			require.Equal(t, int(context.MaxNumOfTxRetries), proc.Retried)

			dumpFiles := listFilesInDir(t, tmpDir)

			// one for codes, one for programs, per retry
			require.Len(t, dumpFiles, 2*proc.Retried)
		})

		extralog.ExtraLogDumpPath = dumpPath
	})

	t.Run("checking error in imported contract", func(t *testing.T) {

		rt := fvm.NewInterpreterRuntime()

		buffer := &bytes.Buffer{}
		log := zerolog.New(buffer)
		txInvoker := fvm.NewTransactionInvoker(log)

		vm := fvm.NewVirtualMachine(rt)

		code := `
			import 0x0b2a3299cc857e29

			transaction {}
		`

		proc := fvm.Transaction(&flow.TransactionBody{Script: []byte(code)}, 0)

		contractAddress := flow.HexToAddress("0b2a3299cc857e29")

		view := utils.NewSimpleView()

		contractCode := `pub contract TestContract: X {}`

		// TODO: refactor this manual deployment by setting ledger keys
		//   into a proper deployment of the contract

		encodedName, err := encodeContractNames([]string{"TestContract"})
		require.NoError(t, err)

		err = view.Set(
			string(contractAddress.Bytes()),
			string(contractAddress.Bytes()),
			"contract_names",
			encodedName,
		)
		require.NoError(t, err)
		err = view.Set(
			string(contractAddress.Bytes()),
			string(contractAddress.Bytes()),
			"code.TestContract",
			[]byte(contractCode),
		)
		require.NoError(t, err)

		context := fvm.NewContext(log)

		sth := state.NewStateHolder(state.NewState(
			view,
			state.WithMaxKeySizeAllowed(context.MaxStateKeySize),
			state.WithMaxValueSizeAllowed(context.MaxStateValueSize),
			state.WithMaxInteractionSizeAllowed(context.MaxStateInteractionSize),
		))

		err = txInvoker.Process(vm, &context, proc, sth, programs.NewEmptyPrograms())
		require.Error(t, err)

		require.Contains(t, buffer.String(), "programs")
		require.Contains(t, buffer.String(), "codes")
		require.Equal(t, int(context.MaxNumOfTxRetries), proc.Retried)
	})

	t.Run("parsing error in transaction", func(t *testing.T) {

		rt := fvm.NewInterpreterRuntime()

		buffer := &bytes.Buffer{}
		log := zerolog.New(buffer)
		txInvoker := fvm.NewTransactionInvoker(log)

		vm := fvm.NewVirtualMachine(rt)

		code := `X`

		proc := fvm.Transaction(&flow.TransactionBody{Script: []byte(code)}, 0)

		view := utils.NewSimpleView()
		context := fvm.NewContext(log)

		sth := state.NewStateHolder(state.NewState(
			view,
			state.WithMaxKeySizeAllowed(context.MaxStateKeySize),
			state.WithMaxValueSizeAllowed(context.MaxStateValueSize),
			state.WithMaxInteractionSizeAllowed(context.MaxStateInteractionSize),
		))

		err := txInvoker.Process(vm, &context, proc, sth, programs.NewEmptyPrograms())
		require.Error(t, err)

		require.NotContains(t, buffer.String(), "programs")
		require.NotContains(t, buffer.String(), "codes")
		require.Equal(t, 0, proc.Retried)

	})

	t.Run("checking error in transaction", func(t *testing.T) {

		rt := fvm.NewInterpreterRuntime()

		buffer := &bytes.Buffer{}
		log := zerolog.New(buffer)
		txInvoker := fvm.NewTransactionInvoker(log)

		vm := fvm.NewVirtualMachine(rt)

		code := `transaction(arg: X) { }`

		proc := fvm.Transaction(&flow.TransactionBody{Script: []byte(code)}, 0)

		view := utils.NewSimpleView()
		context := fvm.NewContext(log)

		sth := state.NewStateHolder(state.NewState(
			view,
			state.WithMaxKeySizeAllowed(context.MaxStateKeySize),
			state.WithMaxValueSizeAllowed(context.MaxStateValueSize),
			state.WithMaxInteractionSizeAllowed(context.MaxStateInteractionSize),
		))

		err := txInvoker.Process(vm, &context, proc, sth, programs.NewEmptyPrograms())
		require.Error(t, err)

		require.NotContains(t, buffer.String(), "programs")
		require.NotContains(t, buffer.String(), "codes")
		require.Equal(t, 0, proc.Retried)
	})

	t.Run("retriable errors causes retry", func(t *testing.T) {

		rt := &ErrorReturningRuntime{TxErrors: []error{
			runtime.Error{ // first error
				Err: &runtime.ParsingCheckingError{
					Err: &sema.CheckerError{
						Errors: []error{
							&sema.AlwaysFailingNonResourceCastingTypeError{
								ValueType:  sema.AnyType,
								TargetType: sema.AnyType,
							}, // some dummy error
							&sema.ImportedProgramError{
								Err: &sema.CheckerError{},
								Location: common.AddressLocation{
									Address: common.MustBytesToAddress([]byte{1, 2, 3, 4}),
								},
							},
						},
					},
				},
				Location: common.TransactionLocation{'t', 'x'},
				Codes:    nil,
				Programs: nil,
			},
			nil, // second error, second call to runtime should be successful
		}}

		log := zerolog.Nop()
		txInvoker := fvm.NewTransactionInvoker(log)

		vm := fvm.NewVirtualMachine(rt)
		code := `doesn't matter`

		proc := fvm.Transaction(&flow.TransactionBody{Script: []byte(code)}, 0)

		view := utils.NewSimpleView()
		header := unittest.BlockHeaderFixture()
		context := fvm.NewContext(log, fvm.WithBlockHeader(&header))

		sth := state.NewStateHolder(state.NewState(view))

		err := txInvoker.Process(vm, &context, proc, sth, programs.NewEmptyPrograms())
		assert.Error(t, err)

		require.Equal(t, 1, proc.Retried)
	})
}

type ErrorReturningRuntime struct {
	TxErrors []error
}

func (e *ErrorReturningRuntime) SetResourceOwnerChangeHandlerEnabled(_ bool) {
	panic("SetResourceOwnerChangeHandlerEnabled not expected")
}

var _ runtime.Runtime = &ErrorReturningRuntime{}

func (e *ErrorReturningRuntime) ExecuteTransaction(_ runtime.Script, _ runtime.Context) error {
	if len(e.TxErrors) == 0 {
		panic("no tx errors left")
	}

	errToReturn := e.TxErrors[0]
	e.TxErrors = e.TxErrors[1:]
	return errToReturn
}

func (*ErrorReturningRuntime) ExecuteScript(_ runtime.Script, _ runtime.Context) (cadence.Value, error) {
	panic("ExecuteScript not expected")
}

func (*ErrorReturningRuntime) ParseAndCheckProgram(_ []byte, _ runtime.Context) (*interpreter.Program, error) {
	panic("ParseAndCheckProgram not expected")
}

func (*ErrorReturningRuntime) SetCoverageReport(_ *runtime.CoverageReport) {
	panic("not used coverage")
}

func (*ErrorReturningRuntime) SetContractUpdateValidationEnabled(_ bool) {
	panic("SetContractUpdateValidationEnabled not expected")
}

func (*ErrorReturningRuntime) SetAtreeValidationEnabled(_ bool) {
	panic("SetAtreeValidationEnabled not expected")
}

func (e *ErrorReturningRuntime) ReadStored(_ common.Address, _ cadence.Path, _ runtime.Context) (cadence.Value, error) {
	panic("ReadStored not expected")
}

func (e *ErrorReturningRuntime) ReadLinked(_ common.Address, _ cadence.Path, _ runtime.Context) (cadence.Value, error) {
	panic("ReadLinked not expected")
}

func (e *ErrorReturningRuntime) InvokeContractFunction(_ common.AddressLocation, _ string, _ []interpreter.Value, _ []sema.Type, _ runtime.Context) (cadence.Value, error) {
	panic("InvokeContractFunction not expected")
}

func (e *ErrorReturningRuntime) SetTracingEnabled(_ bool) {
	panic("SetTracingEnabled not expected")
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

func listFilesInDir(t *testing.T, dir string) []string {

	fileInfos, err := ioutil.ReadDir(dir)
	require.NoError(t, err)

	names := make([]string, len(fileInfos))

	for i, info := range fileInfos {
		names[i] = info.Name()
	}

	return names
}
