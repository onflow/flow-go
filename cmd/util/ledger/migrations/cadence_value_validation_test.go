package migrations

import (
	"bytes"
	"fmt"
	"strconv"
	"testing"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func TestValidateCadenceValues(t *testing.T) {
	address, err := common.HexToAddress("0x1")
	require.NoError(t, err)

	domain := common.PathDomainStorage.Identifier()

	t.Run("no mismatch", func(t *testing.T) {
		log := zerolog.New(zerolog.NewTestWriter(t))

		err := validateCadenceValues(
			address,
			createTestPayloads(t, address, domain),
			createTestPayloads(t, address, domain),
			log,
			false,
		)
		require.NoError(t, err)
	})

	t.Run("has mismatch", func(t *testing.T) {
		var w bytes.Buffer
		log := zerolog.New(&w)

		createPayloads := func(nestedArrayValue interpreter.UInt64Value) []*ledger.Payload {

			// Create account status payload
			accountStatus := environment.NewAccountStatus()
			accountStatusPayload := ledger.NewPayload(
				convert.RegisterIDToLedgerKey(
					flow.AccountStatusRegisterID(flow.ConvertAddress(address)),
				),
				accountStatus.ToBytes(),
			)

			mr, err := newMigratorRuntime(
				address,
				[]*ledger.Payload{accountStatusPayload},
				util.RuntimeInterfaceConfig{},
			)
			require.NoError(t, err)

			// Create new storage map
			storageMap := mr.Storage.GetStorageMap(mr.Address, domain, true)

			// Add Cadence ArrayValue with nested CadenceArray
			nestedArray := interpreter.NewArrayValue(
				mr.Interpreter,
				interpreter.EmptyLocationRange,
				&interpreter.VariableSizedStaticType{
					Type: interpreter.PrimitiveStaticTypeUInt64,
				},
				address,
				interpreter.NewUnmeteredUInt64Value(0),
				nestedArrayValue,
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

		oldPayloads := createPayloads(interpreter.NewUnmeteredUInt64Value(1))
		newPayloads := createPayloads(interpreter.NewUnmeteredUInt64Value(2))
		wantErrorMsg := "failed to validate value for address 0000000000000001, domain storage, key 0: failed to validate ([AnyStruct][0]).([UInt64][1]): values differ: 1 (interpreter.UInt64Value) != 2 (interpreter.UInt64Value)"
		wantVerboseMsg := "{\"level\":\"info\",\"address\":\"0000000000000001\",\"domain\":\"storage\",\"key\":\"0\",\"trace\":\"failed to validate ([AnyStruct][0]).([UInt64][1]): values differ: 1 (interpreter.UInt64Value) != 2 (interpreter.UInt64Value)\",\"old value\":\"[[0, 1]]\",\"new value\":\"[[0, 2]]\",\"message\":\"failed to validate value\"}\n"

		// Disable verbose logging
		err := validateCadenceValues(
			address,
			oldPayloads,
			newPayloads,
			log,
			false,
		)
		require.ErrorContains(t, err, wantErrorMsg)
		require.Equal(t, 0, w.Len())

		// Enable verbose logging
		err = validateCadenceValues(
			address,
			oldPayloads,
			newPayloads,
			log,
			true,
		)
		require.ErrorContains(t, err, wantErrorMsg)
		require.Equal(t, wantVerboseMsg, w.String())
	})
}

func createTestPayloads(t *testing.T, address common.Address, domain string) []*ledger.Payload {

	// Create account status payload
	accountStatus := environment.NewAccountStatus()
	accountStatusPayload := ledger.NewPayload(
		convert.RegisterIDToLedgerKey(
			flow.AccountStatusRegisterID(flow.ConvertAddress(address)),
		),
		accountStatus.ToBytes(),
	)

	mr, err := newMigratorRuntime(
		address,
		[]*ledger.Payload{accountStatusPayload},
		util.RuntimeInterfaceConfig{},
	)
	require.NoError(t, err)

	// Create new storage map
	storageMap := mr.Storage.GetStorageMap(mr.Address, domain, true)

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
