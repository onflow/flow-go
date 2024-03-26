package migrations

import (
	"fmt"
	"testing"

	"github.com/rs/zerolog"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"

	"github.com/stretchr/testify/require"
)

func TestDiffCadenceValues(t *testing.T) {
	address, err := common.HexToAddress("0x1")
	require.NoError(t, err)

	domain := common.PathDomainStorage.Identifier()

	t.Run("no diff", func(t *testing.T) {
		writer := &testReportWriter{}

		diffReporter := NewCadenceValueDiffReporter(address, writer, true)

		diffReporter.DiffStates(
			createTestPayloads(t, address, domain),
			createTestPayloads(t, address, domain),
			[]string{domain},
		)
		require.NoError(t, err)
		require.Equal(t, 0, len(writer.entries))
	})

	t.Run("one storage map doesn't exist", func(t *testing.T) {
		writer := &testReportWriter{}

		diffReporter := NewCadenceValueDiffReporter(address, writer, true)

		diffReporter.DiffStates(
			createTestPayloads(t, address, domain),
			nil,
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
		writer := &testReportWriter{}

		diffReporter := NewCadenceValueDiffReporter(address, writer, true)

		diffReporter.DiffStates(
			createTestPayloads(t, address, domain),
			createStorageMapPayloads(t, address, domain, []string{"unique_key"}, []interpreter.Value{interpreter.UInt64Value(0)}),
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
		writer := &testReportWriter{}

		diffReporter := NewCadenceValueDiffReporter(address, writer, true)

		diffReporter.DiffStates(
			createStorageMapPayloads(t, address, domain, []string{"0", "1"}, []interpreter.Value{interpreter.UInt64Value(0), interpreter.UInt64Value(0)}),
			createStorageMapPayloads(t, address, domain, []string{"2", "0"}, []interpreter.Value{interpreter.UInt64Value(0), interpreter.UInt64Value(0)}),
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
		writer := &testReportWriter{}

		diffReporter := NewCadenceValueDiffReporter(address, writer, false)

		diffReporter.DiffStates(
			createStorageMapPayloads(t, address, domain, []string{"0", "1"}, []interpreter.Value{interpreter.UInt64Value(100), interpreter.UInt64Value(101)}),
			createStorageMapPayloads(t, address, domain, []string{"0", "1"}, []interpreter.Value{interpreter.UInt64Value(111), interpreter.UInt64Value(101)}),
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
		writer := &testReportWriter{}

		diffReporter := NewCadenceValueDiffReporter(address, writer, false)

		diffReporter.DiffStates(
			createStorageMapPayloads(t, address, domain, []string{"0", "1"}, []interpreter.Value{interpreter.UInt64Value(100), interpreter.UInt64Value(101)}),
			createStorageMapPayloads(t, address, domain, []string{"0", "1"}, []interpreter.Value{interpreter.UInt64Value(111), interpreter.UInt64Value(102)}),
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
		writer := &testReportWriter{}

		diffReporter := NewCadenceValueDiffReporter(address, writer, false)

		createPayloads := func(arrayValues []interpreter.Value) []*ledger.Payload {

			// Create account status payload
			accountStatus := environment.NewAccountStatus()
			accountStatusPayload := ledger.NewPayload(
				convert.RegisterIDToLedgerKey(
					flow.AccountStatusRegisterID(flow.ConvertAddress(address)),
				),
				accountStatus.ToBytes(),
			)

			mr, err := NewMigratorRuntime(
				zerolog.Nop(),
				address,
				[]*ledger.Payload{accountStatusPayload},
				util.RuntimeInterfaceConfig{},
			)
			require.NoError(t, err)

			// Create new storage map
			storageMap := mr.Storage.GetStorageMap(mr.Address, domain, true)

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

			err = mr.Storage.Commit(mr.Interpreter, false)
			require.NoError(t, err)

			// finalize the transaction
			result, err := mr.TransactionState.FinalizeMainTransaction()
			require.NoError(t, err)

			payloads := make([]*ledger.Payload, 0, len(result.WriteSet))
			for id, value := range result.WriteSet {
				key := convert.RegisterIDToLedgerKey(id)
				payloads = append(payloads, ledger.NewPayload(key, value))
			}

			return payloads
		}

		diffReporter.DiffStates(
			createPayloads([]interpreter.Value{interpreter.UInt64Value(0), interpreter.UInt64Value(2), interpreter.UInt64Value(4)}),
			createPayloads([]interpreter.Value{interpreter.UInt64Value(1), interpreter.UInt64Value(3), interpreter.UInt64Value(5)}),
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
		writer := &testReportWriter{}

		diffReporter := NewCadenceValueDiffReporter(address, writer, false)

		createPayloads := func(dictValues []interpreter.Value) []*ledger.Payload {

			// Create account status payload
			accountStatus := environment.NewAccountStatus()
			accountStatusPayload := ledger.NewPayload(
				convert.RegisterIDToLedgerKey(
					flow.AccountStatusRegisterID(flow.ConvertAddress(address)),
				),
				accountStatus.ToBytes(),
			)

			mr, err := NewMigratorRuntime(
				zerolog.Nop(),
				address,
				[]*ledger.Payload{accountStatusPayload},
				util.RuntimeInterfaceConfig{},
			)
			require.NoError(t, err)

			// Create new storage map
			storageMap := mr.Storage.GetStorageMap(mr.Address, domain, true)

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

			err = mr.Storage.Commit(mr.Interpreter, false)
			require.NoError(t, err)

			// finalize the transaction
			result, err := mr.TransactionState.FinalizeMainTransaction()
			require.NoError(t, err)

			payloads := make([]*ledger.Payload, 0, len(result.WriteSet))
			for id, value := range result.WriteSet {
				key := convert.RegisterIDToLedgerKey(id)
				payloads = append(payloads, ledger.NewPayload(key, value))
			}

			return payloads
		}

		diffReporter.DiffStates(
			createPayloads(
				[]interpreter.Value{interpreter.NewUnmeteredStringValue("dict_key_0"),
					interpreter.UInt64Value(0),
					interpreter.NewUnmeteredStringValue("dict_key_1"),
					interpreter.UInt64Value(2),
				}),
			createPayloads(
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
		writer := &testReportWriter{}

		diffReporter := NewCadenceValueDiffReporter(address, writer, false)

		createPayloads := func(compositeFields []string, compositeValues []interpreter.Value) []*ledger.Payload {

			// Create account status payload
			accountStatus := environment.NewAccountStatus()
			accountStatusPayload := ledger.NewPayload(
				convert.RegisterIDToLedgerKey(
					flow.AccountStatusRegisterID(flow.ConvertAddress(address)),
				),
				accountStatus.ToBytes(),
			)

			mr, err := NewMigratorRuntime(
				zerolog.Nop(),
				address,
				[]*ledger.Payload{accountStatusPayload},
				util.RuntimeInterfaceConfig{},
			)
			require.NoError(t, err)

			// Create new storage map
			storageMap := mr.Storage.GetStorageMap(mr.Address, domain, true)

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

			err = mr.Storage.Commit(mr.Interpreter, false)
			require.NoError(t, err)

			// finalize the transaction
			result, err := mr.TransactionState.FinalizeMainTransaction()
			require.NoError(t, err)

			payloads := make([]*ledger.Payload, 0, len(result.WriteSet))
			for id, value := range result.WriteSet {
				key := convert.RegisterIDToLedgerKey(id)
				payloads = append(payloads, ledger.NewPayload(key, value))
			}

			return payloads
		}

		diffReporter.DiffStates(
			createPayloads(
				[]string{
					"Field_0",
					"Field_1",
				},
				[]interpreter.Value{
					interpreter.UInt64Value(0),
					interpreter.UInt64Value(2),
				}),
			createPayloads(
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
		writer := &testReportWriter{}

		diffReporter := NewCadenceValueDiffReporter(address, writer, true)

		createPayloads := func(compositeFields []string, compositeValues []interpreter.Value) []*ledger.Payload {

			// Create account status payload
			accountStatus := environment.NewAccountStatus()
			accountStatusPayload := ledger.NewPayload(
				convert.RegisterIDToLedgerKey(
					flow.AccountStatusRegisterID(flow.ConvertAddress(address)),
				),
				accountStatus.ToBytes(),
			)

			mr, err := NewMigratorRuntime(
				zerolog.Nop(),
				address,
				[]*ledger.Payload{accountStatusPayload},
				util.RuntimeInterfaceConfig{},
			)
			require.NoError(t, err)

			// Create new storage map
			storageMap := mr.Storage.GetStorageMap(mr.Address, domain, true)

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

			err = mr.Storage.Commit(mr.Interpreter, false)
			require.NoError(t, err)

			// finalize the transaction
			result, err := mr.TransactionState.FinalizeMainTransaction()
			require.NoError(t, err)

			payloads := make([]*ledger.Payload, 0, len(result.WriteSet))
			for id, value := range result.WriteSet {
				key := convert.RegisterIDToLedgerKey(id)
				payloads = append(payloads, ledger.NewPayload(key, value))
			}

			return payloads
		}

		diffReporter.DiffStates(
			createPayloads(
				[]string{
					"Field_0",
					"Field_1",
				},
				[]interpreter.Value{
					interpreter.UInt64Value(0),
					interpreter.UInt64Value(2),
				}),
			createPayloads(
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

		// Test storge map value diff log (only with verbose logging)
		diff := writer.entries[2].(difference)
		require.Equal(t, diffKindString[storageMapValueDiffKind], diff.Kind)
		require.Equal(t, address.Hex(), diff.Address)
		require.Equal(t, domain, diff.Domain)
		require.Equal(t, "storage[key_0]", diff.Trace)
		require.Equal(t, "[S.test.Test(Field_1: 2, Field_0: 0)]", diff.OldValue)
		require.Equal(t, "[S.test.Test(Field_1: 3, Field_0: 1)]", diff.NewValue)
	})
}

func createStorageMapPayloads(t *testing.T, address common.Address, domain string, keys []string, values []interpreter.Value) []*ledger.Payload {

	// Create account status payload
	accountStatus := environment.NewAccountStatus()
	accountStatusPayload := ledger.NewPayload(
		convert.RegisterIDToLedgerKey(
			flow.AccountStatusRegisterID(flow.ConvertAddress(address)),
		),
		accountStatus.ToBytes(),
	)

	mr, err := NewMigratorRuntime(
		zerolog.Nop(),
		address,
		[]*ledger.Payload{accountStatusPayload},
		util.RuntimeInterfaceConfig{},
	)
	require.NoError(t, err)

	// Create new storage map
	storageMap := mr.Storage.GetStorageMap(mr.Address, domain, true)

	for i, k := range keys {
		storageMap.WriteValue(
			mr.Interpreter,
			interpreter.StringStorageMapKey(k),
			values[i],
		)
	}

	err = mr.Storage.Commit(mr.Interpreter, false)
	require.NoError(t, err)

	// finalize the transaction
	result, err := mr.TransactionState.FinalizeMainTransaction()
	require.NoError(t, err)

	payloads := make([]*ledger.Payload, 0, len(result.WriteSet))
	for id, value := range result.WriteSet {
		key := convert.RegisterIDToLedgerKey(id)
		payloads = append(payloads, ledger.NewPayload(key, value))
	}

	return payloads
}
