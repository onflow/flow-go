package migrations

import (
	"fmt"
	"runtime"
	"strconv"
	"testing"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"

	"github.com/stretchr/testify/require"
)

func TestDiffCadenceValues(t *testing.T) {
	t.Parallel()

	address, err := common.HexToAddress("0x1")
	require.NoError(t, err)

	domain := common.PathDomainStorage.Identifier()

	t.Run("no diff", func(t *testing.T) {
		t.Parallel()

		writer := &testReportWriter{}

		diffReporter := NewCadenceValueDiffReporter(address, flow.Emulator, writer, true, runtime.NumCPU())

		diffReporter.DiffStates(
			createTestRegisters(t, address, domain),
			createTestRegisters(t, address, domain),
			[]string{domain},
		)
		require.NoError(t, err)
		require.Equal(t, 0, len(writer.entries))
	})

	t.Run("one storage map doesn't exist", func(t *testing.T) {
		t.Parallel()

		writer := &testReportWriter{}

		diffReporter := NewCadenceValueDiffReporter(address, flow.Emulator, writer, true, runtime.NumCPU())

		diffReporter.DiffStates(
			createTestRegisters(t, address, domain),
			registers.NewByAccount(),
			[]string{domain},
		)
		require.NoError(t, err)
		require.Equal(t, 1, len(writer.entries))

		diff := writer.entries[0].(difference)
		require.Equal(t, diffKindString[storageMapExistDiffKind], diff.Kind)
		require.Equal(t, address.Hex(), diff.Address)
		require.Equal(t, domain, diff.Domain)
	})

	t.Run("storage maps have different sets of keys", func(t *testing.T) {
		t.Parallel()

		writer := &testReportWriter{}

		diffReporter := NewCadenceValueDiffReporter(address, flow.Emulator, writer, true, runtime.NumCPU())

		diffReporter.DiffStates(
			createTestRegisters(t, address, domain),
			createStorageMapRegisters(t, address, domain, []string{"unique_key"}, []interpreter.Value{interpreter.UInt64Value(0)}),
			[]string{domain},
		)
		require.NoError(t, err)

		// 2 differences:
		// - unique keys in old storage map
		// - unique keys in new storage map
		require.Equal(t, 2, len(writer.entries))

		for _, entry := range writer.entries {
			diff := entry.(difference)
			require.Equal(t, diffKindString[storageMapKeyDiffKind], diff.Kind)
			require.Equal(t, address.Hex(), diff.Address)
			require.Equal(t, domain, diff.Domain)
		}
	})

	t.Run("storage maps have overlapping keys", func(t *testing.T) {
		t.Parallel()

		writer := &testReportWriter{}

		diffReporter := NewCadenceValueDiffReporter(address, flow.Emulator, writer, true, runtime.NumCPU())

		diffReporter.DiffStates(
			createStorageMapRegisters(t, address, domain, []string{"0", "1"}, []interpreter.Value{interpreter.UInt64Value(0), interpreter.UInt64Value(0)}),
			createStorageMapRegisters(t, address, domain, []string{"2", "0"}, []interpreter.Value{interpreter.UInt64Value(0), interpreter.UInt64Value(0)}),
			[]string{domain},
		)
		require.NoError(t, err)

		// 2 entries:
		// - unique keys in old storage map
		// - unique keys in new storage map
		require.Equal(t, 2, len(writer.entries))

		for _, entry := range writer.entries {
			diff := entry.(difference)
			require.Equal(t, diffKindString[storageMapKeyDiffKind], diff.Kind)
			require.Equal(t, address.Hex(), diff.Address)
			require.Equal(t, domain, diff.Domain)
		}
	})

	t.Run("storage maps have one different value", func(t *testing.T) {
		t.Parallel()

		writer := &testReportWriter{}

		diffReporter := NewCadenceValueDiffReporter(address, flow.Emulator, writer, false, runtime.NumCPU())

		diffReporter.DiffStates(
			createStorageMapRegisters(t, address, domain, []string{"0", "1"}, []interpreter.Value{interpreter.UInt64Value(100), interpreter.UInt64Value(101)}),
			createStorageMapRegisters(t, address, domain, []string{"0", "1"}, []interpreter.Value{interpreter.UInt64Value(111), interpreter.UInt64Value(101)}),
			[]string{domain},
		)
		require.NoError(t, err)

		// 1 entries:
		// - different value
		require.Equal(t, 1, len(writer.entries))

		diff := writer.entries[0].(difference)
		require.Equal(t, diffKindString[cadenceValueDiffKind], diff.Kind)
		require.Equal(t, address.Hex(), diff.Address)
		require.Equal(t, domain, diff.Domain)
		require.Equal(t, "storage[0]", diff.Trace)
		require.Equal(t, "100", diff.OldValue)
		require.Equal(t, "111", diff.NewValue)
	})

	t.Run("storage maps have multiple different values", func(t *testing.T) {
		t.Parallel()

		writer := &testReportWriter{}

		diffReporter := NewCadenceValueDiffReporter(address, flow.Emulator, writer, false, runtime.NumCPU())

		diffReporter.DiffStates(
			createStorageMapRegisters(t, address, domain, []string{"0", "1"}, []interpreter.Value{interpreter.UInt64Value(100), interpreter.UInt64Value(101)}),
			createStorageMapRegisters(t, address, domain, []string{"0", "1"}, []interpreter.Value{interpreter.UInt64Value(111), interpreter.UInt64Value(102)}),
			[]string{domain},
		)
		require.NoError(t, err)

		// 2 entries with 2 different values:
		require.Equal(t, 2, len(writer.entries))

		for _, entry := range writer.entries {
			diff := entry.(difference)
			require.Equal(t, diffKindString[cadenceValueDiffKind], diff.Kind)
			require.Equal(t, address.Hex(), diff.Address)
			require.Equal(t, domain, diff.Domain)
			require.True(t, diff.Trace == "storage[0]" || diff.Trace == "storage[1]")
		}
	})

	t.Run("nested array value has different elements", func(t *testing.T) {
		t.Parallel()

		writer := &testReportWriter{}

		diffReporter := NewCadenceValueDiffReporter(address, flow.Emulator, writer, false, runtime.NumCPU())

		createRegisters := func(arrayValues []interpreter.Value) registers.Registers {

			// Create account status payload
			accountStatus := environment.NewAccountStatus()
			accountStatusPayload := ledger.NewPayload(
				convert.RegisterIDToLedgerKey(
					flow.AccountStatusRegisterID(flow.ConvertAddress(address)),
				),
				accountStatus.ToBytes(),
			)

			// at least one payload otherwise the migration will not get called
			registersByAccount, err := registers.NewByAccountFromPayloads(
				[]*ledger.Payload{
					accountStatusPayload,
				},
			)
			require.NoError(t, err)

			mr, err := NewInterpreterMigrationRuntime(
				registersByAccount.AccountRegisters(string(address[:])),
				flow.Emulator,
				InterpreterMigrationRuntimeConfig{},
			)
			require.NoError(t, err)

			// Create new storage map
			storageMap := mr.Storage.GetStorageMap(address, domain, true)

			nestedArray := interpreter.NewArrayValue(
				mr.Interpreter,
				interpreter.EmptyLocationRange,
				&interpreter.VariableSizedStaticType{
					Type: interpreter.PrimitiveStaticTypeUInt64,
				},
				address,
				arrayValues...,
			)

			storageMap.WriteValue(
				mr.Interpreter,
				interpreter.StringStorageMapKey(fmt.Sprintf("key_%d", storageMap.Count())),
				interpreter.NewArrayValue(
					mr.Interpreter,
					interpreter.EmptyLocationRange,
					&interpreter.VariableSizedStaticType{
						Type: interpreter.PrimitiveStaticTypeAnyStruct,
					},
					address,
					nestedArray,
				),
			)

			err = mr.Storage.NondeterministicCommit(mr.Interpreter, false)
			require.NoError(t, err)

			// finalize the transaction
			result, err := mr.TransactionState.FinalizeMainTransaction()
			require.NoError(t, err)

			payloads := make([]*ledger.Payload, 0, len(result.WriteSet))
			for id, value := range result.WriteSet {
				key := convert.RegisterIDToLedgerKey(id)
				payloads = append(payloads, ledger.NewPayload(key, value))
			}

			registers, err := registers.NewByAccountFromPayloads(payloads)
			require.NoError(t, err)

			return registers
		}

		diffReporter.DiffStates(
			createRegisters([]interpreter.Value{
				interpreter.UInt64Value(0),
				interpreter.UInt64Value(2),
				interpreter.UInt64Value(4),
			}),
			createRegisters([]interpreter.Value{
				interpreter.UInt64Value(1),
				interpreter.UInt64Value(3),
				interpreter.UInt64Value(5),
			}),
			[]string{domain},
		)
		require.NoError(t, err)

		// 3 entries:
		// - different value
		require.Equal(t, 3, len(writer.entries))

		for _, entry := range writer.entries {
			diff := entry.(difference)
			require.Equal(t, diffKindString[cadenceValueDiffKind], diff.Kind)
			require.Equal(t, address.Hex(), diff.Address)
			require.Equal(t, domain, diff.Domain)
			require.True(t, diff.Trace == "storage[key_0][0][0]" || diff.Trace == "storage[key_0][0][1]" || diff.Trace == "storage[key_0][0][2]")

			switch diff.Trace {
			case "storage[key_0][0][0]":
				require.Equal(t, "0", diff.OldValue)
				require.Equal(t, "1", diff.NewValue)

			case "storage[key_0][0][1]":
				require.Equal(t, "2", diff.OldValue)
				require.Equal(t, "3", diff.NewValue)

			case "storage[key_0][0][2]":
				require.Equal(t, "4", diff.OldValue)
				require.Equal(t, "5", diff.NewValue)
			}
		}
	})

	t.Run("nested dict value has different elements", func(t *testing.T) {
		t.Parallel()

		writer := &testReportWriter{}

		diffReporter := NewCadenceValueDiffReporter(address, flow.Emulator, writer, false, runtime.NumCPU())

		createRegisters := func(dictValues []interpreter.Value) registers.Registers {

			// Create account status payload
			accountStatus := environment.NewAccountStatus()
			accountStatusPayload := ledger.NewPayload(
				convert.RegisterIDToLedgerKey(
					flow.AccountStatusRegisterID(flow.ConvertAddress(address)),
				),
				accountStatus.ToBytes(),
			)

			// at least one payload otherwise the migration will not get called
			registersByAccount, err := registers.NewByAccountFromPayloads(
				[]*ledger.Payload{
					accountStatusPayload,
				},
			)
			require.NoError(t, err)

			mr, err := NewInterpreterMigrationRuntime(
				registersByAccount.AccountRegisters(string(address[:])),
				flow.Emulator,
				InterpreterMigrationRuntimeConfig{},
			)
			require.NoError(t, err)

			// Create new storage map
			storageMap := mr.Storage.GetStorageMap(address, domain, true)

			nestedDict := interpreter.NewDictionaryValueWithAddress(
				mr.Interpreter,
				interpreter.EmptyLocationRange,
				&interpreter.DictionaryStaticType{
					KeyType:   interpreter.PrimitiveStaticTypeAnyStruct,
					ValueType: interpreter.PrimitiveStaticTypeAnyStruct,
				},
				address,
				dictValues...,
			)

			storageMap.WriteValue(
				mr.Interpreter,
				interpreter.StringStorageMapKey(fmt.Sprintf("key_%d", storageMap.Count())),
				interpreter.NewArrayValue(
					mr.Interpreter,
					interpreter.EmptyLocationRange,
					&interpreter.VariableSizedStaticType{
						Type: interpreter.PrimitiveStaticTypeAnyStruct,
					},
					address,
					nestedDict,
				),
			)

			err = mr.Storage.NondeterministicCommit(mr.Interpreter, false)
			require.NoError(t, err)

			// finalize the transaction
			result, err := mr.TransactionState.FinalizeMainTransaction()
			require.NoError(t, err)

			payloads := make([]*ledger.Payload, 0, len(result.WriteSet))
			for id, value := range result.WriteSet {
				key := convert.RegisterIDToLedgerKey(id)
				payloads = append(payloads, ledger.NewPayload(key, value))
			}

			registers, err := registers.NewByAccountFromPayloads(payloads)
			require.NoError(t, err)

			return registers
		}

		diffReporter.DiffStates(
			createRegisters(
				[]interpreter.Value{interpreter.NewUnmeteredStringValue("dict_key_0"),
					interpreter.UInt64Value(0),
					interpreter.NewUnmeteredStringValue("dict_key_1"),
					interpreter.UInt64Value(2),
				}),
			createRegisters(
				[]interpreter.Value{interpreter.NewUnmeteredStringValue("dict_key_0"),
					interpreter.UInt64Value(1),
					interpreter.NewUnmeteredStringValue("dict_key_1"),
					interpreter.UInt64Value(3),
				}),
			[]string{domain},
		)
		require.NoError(t, err)

		// 2 entries:
		// - different value
		require.Equal(t, 2, len(writer.entries))

		for _, entry := range writer.entries {
			diff := entry.(difference)
			require.Equal(t, diffKindString[cadenceValueDiffKind], diff.Kind)
			require.Equal(t, address.Hex(), diff.Address)
			require.Equal(t, domain, diff.Domain)
			require.True(t, diff.Trace == "storage[key_0][0][\"dict_key_0\"]" || diff.Trace == "storage[key_0][0][\"dict_key_1\"]")

			switch diff.Trace {
			case "storage[key_0][0][\"dict_key_0\"]":
				require.Equal(t, "0", diff.OldValue)
				require.Equal(t, "1", diff.NewValue)

			case "storage[key_0][0][\"dict_key_1\"]":
				require.Equal(t, "2", diff.OldValue)
				require.Equal(t, "3", diff.NewValue)
			}
		}
	})

	t.Run("nested composite value has different elements", func(t *testing.T) {
		t.Parallel()

		writer := &testReportWriter{}

		diffReporter := NewCadenceValueDiffReporter(address, flow.Emulator, writer, false, runtime.NumCPU())

		createRegisters := func(compositeFields []string, compositeValues []interpreter.Value) registers.Registers {

			// Create account status payload
			accountStatus := environment.NewAccountStatus()
			accountStatusPayload := ledger.NewPayload(
				convert.RegisterIDToLedgerKey(
					flow.AccountStatusRegisterID(flow.ConvertAddress(address)),
				),
				accountStatus.ToBytes(),
			)

			// at least one payload otherwise the migration will not get called
			registersByAccount, err := registers.NewByAccountFromPayloads(
				[]*ledger.Payload{
					accountStatusPayload,
				},
			)
			require.NoError(t, err)

			mr, err := NewInterpreterMigrationRuntime(
				registersByAccount.AccountRegisters(string(address[:])),
				flow.Emulator,
				InterpreterMigrationRuntimeConfig{},
			)
			require.NoError(t, err)

			// Create new storage map
			storageMap := mr.Storage.GetStorageMap(address, domain, true)

			var fields []interpreter.CompositeField

			for i, fieldName := range compositeFields {
				fields = append(fields, interpreter.CompositeField{Name: fieldName, Value: compositeValues[i]})
			}

			nestedComposite := interpreter.NewCompositeValue(
				mr.Interpreter,
				interpreter.EmptyLocationRange,
				common.StringLocation("test"),
				"Test",
				common.CompositeKindStructure,
				fields,
				address,
			)

			storageMap.WriteValue(
				mr.Interpreter,
				interpreter.StringStorageMapKey(fmt.Sprintf("key_%d", storageMap.Count())),
				interpreter.NewArrayValue(
					mr.Interpreter,
					interpreter.EmptyLocationRange,
					&interpreter.VariableSizedStaticType{
						Type: interpreter.PrimitiveStaticTypeAnyStruct,
					},
					address,
					nestedComposite,
				),
			)

			err = mr.Storage.NondeterministicCommit(mr.Interpreter, false)
			require.NoError(t, err)

			// finalize the transaction
			result, err := mr.TransactionState.FinalizeMainTransaction()
			require.NoError(t, err)

			payloads := make([]*ledger.Payload, 0, len(result.WriteSet))
			for id, value := range result.WriteSet {
				key := convert.RegisterIDToLedgerKey(id)
				payloads = append(payloads, ledger.NewPayload(key, value))
			}

			registers, err := registers.NewByAccountFromPayloads(payloads)
			require.NoError(t, err)

			return registers
		}

		diffReporter.DiffStates(
			createRegisters(
				[]string{
					"Field_0",
					"Field_1",
				},
				[]interpreter.Value{
					interpreter.UInt64Value(0),
					interpreter.UInt64Value(2),
				}),
			createRegisters(
				[]string{
					"Field_0",
					"Field_1",
				},
				[]interpreter.Value{
					interpreter.UInt64Value(1),
					interpreter.UInt64Value(3),
				}),
			[]string{domain},
		)
		require.NoError(t, err)

		// 2 entries:
		// - different value
		require.Equal(t, 2, len(writer.entries))

		for _, entry := range writer.entries {
			diff := entry.(difference)
			require.Equal(t, diffKindString[cadenceValueDiffKind], diff.Kind)
			require.Equal(t, address.Hex(), diff.Address)
			require.Equal(t, domain, diff.Domain)
			require.True(t, diff.Trace == "storage[key_0][0].Field_0" || diff.Trace == "storage[key_0][0].Field_1")

			switch diff.Trace {
			case "storage[key_0][0].Field_0":
				require.Equal(t, "0", diff.OldValue)
				require.Equal(t, "1", diff.NewValue)

			case "storage[key_0][0].Field_1":
				require.Equal(t, "2", diff.OldValue)
				require.Equal(t, "3", diff.NewValue)
			}
		}
	})

	t.Run("nested composite value has different elements with verbose logging", func(t *testing.T) {
		t.Parallel()

		writer := &testReportWriter{}

		diffReporter := NewCadenceValueDiffReporter(address, flow.Emulator, writer, true, runtime.NumCPU())

		createRegisters := func(compositeFields []string, compositeValues []interpreter.Value) registers.Registers {

			// Create account status payload
			accountStatus := environment.NewAccountStatus()
			accountStatusPayload := ledger.NewPayload(
				convert.RegisterIDToLedgerKey(
					flow.AccountStatusRegisterID(flow.ConvertAddress(address)),
				),
				accountStatus.ToBytes(),
			)

			// at least one payload otherwise the migration will not get called
			registersByAccount, err := registers.NewByAccountFromPayloads(
				[]*ledger.Payload{
					accountStatusPayload,
				},
			)
			require.NoError(t, err)

			mr, err := NewInterpreterMigrationRuntime(
				registersByAccount.AccountRegisters(string(address[:])),
				flow.Emulator,
				InterpreterMigrationRuntimeConfig{},
			)
			require.NoError(t, err)

			// Create new storage map
			storageMap := mr.Storage.GetStorageMap(address, domain, true)

			var fields []interpreter.CompositeField

			for i, fieldName := range compositeFields {
				fields = append(fields, interpreter.CompositeField{Name: fieldName, Value: compositeValues[i]})
			}

			nestedComposite := interpreter.NewCompositeValue(
				mr.Interpreter,
				interpreter.EmptyLocationRange,
				common.StringLocation("test"),
				"Test",
				common.CompositeKindStructure,
				fields,
				address,
			)

			storageMap.WriteValue(
				mr.Interpreter,
				interpreter.StringStorageMapKey(fmt.Sprintf("key_%d", storageMap.Count())),
				interpreter.NewArrayValue(
					mr.Interpreter,
					interpreter.EmptyLocationRange,
					&interpreter.VariableSizedStaticType{
						Type: interpreter.PrimitiveStaticTypeAnyStruct,
					},
					address,
					nestedComposite,
				),
			)

			err = mr.Storage.NondeterministicCommit(mr.Interpreter, false)
			require.NoError(t, err)

			// finalize the transaction
			result, err := mr.TransactionState.FinalizeMainTransaction()
			require.NoError(t, err)

			payloads := make([]*ledger.Payload, 0, len(result.WriteSet))
			for id, value := range result.WriteSet {
				key := convert.RegisterIDToLedgerKey(id)
				payloads = append(payloads, ledger.NewPayload(key, value))
			}

			registers, err := registers.NewByAccountFromPayloads(payloads)
			require.NoError(t, err)

			return registers
		}

		diffReporter.DiffStates(
			createRegisters(
				[]string{
					"Field_0",
					"Field_1",
				},
				[]interpreter.Value{
					interpreter.UInt64Value(0),
					interpreter.UInt64Value(2),
				}),
			createRegisters(
				[]string{
					"Field_0",
					"Field_1",
				},
				[]interpreter.Value{
					interpreter.UInt64Value(1),
					interpreter.UInt64Value(3),
				}),
			[]string{domain},
		)
		require.NoError(t, err)

		// 3 entries:
		// - 2 different values
		// - verbose logging of storage map element
		require.Equal(t, 3, len(writer.entries))

		// Test 2 cadence value diff logs
		for _, entry := range writer.entries[:2] {
			diff := entry.(difference)
			require.Equal(t, diffKindString[cadenceValueDiffKind], diff.Kind)
			require.Equal(t, address.Hex(), diff.Address)
			require.Equal(t, domain, diff.Domain)
			require.True(t, diff.Trace == "storage[key_0][0].Field_0" || diff.Trace == "storage[key_0][0].Field_1")

			switch diff.Trace {
			case "storage[key_0][0].Field_0":
				require.Equal(t, "0", diff.OldValue)
				require.Equal(t, "1", diff.NewValue)

			case "storage[key_0][0].Field_1":
				require.Equal(t, "2", diff.OldValue)
				require.Equal(t, "3", diff.NewValue)
			}
		}

		// Test storage map value diff log (only with verbose logging)
		diff := writer.entries[2].(difference)
		require.Equal(t, diffKindString[storageMapValueDiffKind], diff.Kind)
		require.Equal(t, address.Hex(), diff.Address)
		require.Equal(t, domain, diff.Domain)
		require.Equal(t, "storage[key_0]", diff.Trace)
		require.Equal(t, "[S.test.Test(Field_1: 2, Field_0: 0)]", diff.OldValue)
		require.Equal(t, "[S.test.Test(Field_1: 3, Field_0: 1)]", diff.NewValue)
	})
}

func createStorageMapRegisters(
	t *testing.T,
	address common.Address,
	domain string,
	keys []string,
	values []interpreter.Value,
) registers.Registers {

	// Create account status payload
	accountStatus := environment.NewAccountStatus()
	accountStatusPayload := ledger.NewPayload(
		convert.RegisterIDToLedgerKey(
			flow.AccountStatusRegisterID(flow.ConvertAddress(address)),
		),
		accountStatus.ToBytes(),
	)

	// at least one payload otherwise the migration will not get called
	registersByAccount, err := registers.NewByAccountFromPayloads(
		[]*ledger.Payload{
			accountStatusPayload,
		},
	)
	require.NoError(t, err)

	mr, err := NewInterpreterMigrationRuntime(
		registersByAccount.AccountRegisters(string(address[:])),
		flow.Emulator,
		InterpreterMigrationRuntimeConfig{},
	)
	require.NoError(t, err)

	// Create new storage map
	storageMap := mr.Storage.GetStorageMap(address, domain, true)

	for i, k := range keys {
		storageMap.WriteValue(
			mr.Interpreter,
			interpreter.StringStorageMapKey(k),
			values[i],
		)
	}

	err = mr.Storage.NondeterministicCommit(mr.Interpreter, false)
	require.NoError(t, err)

	// finalize the transaction
	result, err := mr.TransactionState.FinalizeMainTransaction()
	require.NoError(t, err)

	payloads := make([]*ledger.Payload, 0, len(result.WriteSet))
	for id, value := range result.WriteSet {
		key := convert.RegisterIDToLedgerKey(id)
		payloads = append(payloads, ledger.NewPayload(key, value))
	}

	registers, err := registers.NewByAccountFromPayloads(payloads)
	require.NoError(t, err)

	return registers
}

func createTestRegisters(t *testing.T, address common.Address, domain string) registers.Registers {

	// Create account status payload
	accountStatus := environment.NewAccountStatus()
	accountStatusPayload := ledger.NewPayload(
		convert.RegisterIDToLedgerKey(
			flow.AccountStatusRegisterID(flow.ConvertAddress(address)),
		),
		accountStatus.ToBytes(),
	)

	registersByAccount, err := registers.NewByAccountFromPayloads(
		[]*ledger.Payload{
			accountStatusPayload,
		},
	)
	require.NoError(t, err)

	mr, err := NewInterpreterMigrationRuntime(
		registersByAccount,
		flow.Emulator,
		InterpreterMigrationRuntimeConfig{},
	)
	require.NoError(t, err)

	// Create new storage map
	storageMap := mr.Storage.GetStorageMap(address, domain, true)

	// Add Cadence UInt64Value
	storageMap.WriteValue(
		mr.Interpreter,
		interpreter.StringStorageMapKey(strconv.FormatUint(storageMap.Count(), 10)),
		interpreter.NewUnmeteredUInt64Value(1),
	)

	// Add Cadence SomeValue
	storageMap.WriteValue(
		mr.Interpreter,
		interpreter.StringStorageMapKey(strconv.FormatUint(storageMap.Count(), 10)),
		interpreter.NewUnmeteredSomeValueNonCopying(interpreter.NewUnmeteredStringValue("InnerValueString")),
	)

	// Add Cadence ArrayValue
	const arrayCount = 10
	i := uint64(0)
	storageMap.WriteValue(
		mr.Interpreter,
		interpreter.StringStorageMapKey(strconv.FormatUint(storageMap.Count(), 10)),
		interpreter.NewArrayValueWithIterator(
			mr.Interpreter,
			&interpreter.VariableSizedStaticType{
				Type: interpreter.PrimitiveStaticTypeAnyStruct,
			},
			address,
			0,
			func() interpreter.Value {
				if i == arrayCount {
					return nil
				}
				v := interpreter.NewUnmeteredUInt64Value(i)
				i++
				return v
			},
		),
	)

	// Add Cadence DictionaryValue
	const dictCount = 10
	dictValues := make([]interpreter.Value, 0, dictCount*2)
	for i := 0; i < dictCount; i++ {
		k := interpreter.NewUnmeteredUInt64Value(uint64(i))
		v := interpreter.NewUnmeteredStringValue(fmt.Sprintf("value %d", i))
		dictValues = append(dictValues, k, v)
	}

	storageMap.WriteValue(
		mr.Interpreter,
		interpreter.StringStorageMapKey(strconv.FormatUint(storageMap.Count(), 10)),
		interpreter.NewDictionaryValueWithAddress(
			mr.Interpreter,
			interpreter.EmptyLocationRange,
			&interpreter.DictionaryStaticType{
				KeyType:   interpreter.PrimitiveStaticTypeUInt64,
				ValueType: interpreter.PrimitiveStaticTypeString,
			},
			address,
			dictValues...,
		),
	)

	// Add Cadence CompositeValue
	storageMap.WriteValue(
		mr.Interpreter,
		interpreter.StringStorageMapKey(strconv.FormatUint(storageMap.Count(), 10)),
		interpreter.NewCompositeValue(
			mr.Interpreter,
			interpreter.EmptyLocationRange,
			common.StringLocation("test"),
			"Test",
			common.CompositeKindStructure,
			[]interpreter.CompositeField{
				{Name: "field1", Value: interpreter.NewUnmeteredStringValue("value1")},
				{Name: "field2", Value: interpreter.NewUnmeteredStringValue("value2")},
			},
			address,
		),
	)

	// Add Cadence DictionaryValue with nested CadenceArray
	nestedArrayValue := interpreter.NewArrayValue(
		mr.Interpreter,
		interpreter.EmptyLocationRange,
		&interpreter.VariableSizedStaticType{
			Type: interpreter.PrimitiveStaticTypeUInt64,
		},
		address,
		interpreter.NewUnmeteredUInt64Value(0),
	)

	storageMap.WriteValue(
		mr.Interpreter,
		interpreter.StringStorageMapKey(strconv.FormatUint(storageMap.Count(), 10)),
		interpreter.NewArrayValue(
			mr.Interpreter,
			interpreter.EmptyLocationRange,
			&interpreter.VariableSizedStaticType{
				Type: interpreter.PrimitiveStaticTypeAnyStruct,
			},
			address,
			nestedArrayValue,
		),
	)

	err = mr.Storage.NondeterministicCommit(mr.Interpreter, false)
	require.NoError(t, err)

	// finalize the transaction
	result, err := mr.TransactionState.FinalizeMainTransaction()
	require.NoError(t, err)

	payloads := make([]*ledger.Payload, 0, len(result.WriteSet))
	for id, value := range result.WriteSet {
		key := convert.RegisterIDToLedgerKey(id)
		payloads = append(payloads, ledger.NewPayload(key, value))
	}

	registers, err := registers.NewByAccountFromPayloads(payloads)
	require.NoError(t, err)

	return registers
}
