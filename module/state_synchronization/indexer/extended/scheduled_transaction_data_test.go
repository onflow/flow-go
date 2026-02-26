package extended_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/common"
	jsoncdc "github.com/onflow/cadence/encoding/json"

	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"

	. "github.com/onflow/flow-go/module/state_synchronization/indexer/extended"
)

// ===== EncodeGetTransactionDataArg =====

// TestEncodeGetTransactionDataArg_Empty verifies that encoding an empty slice produces a
// valid JSON-CDC array that can be decoded.
func TestEncodeGetTransactionDataArg_Empty(t *testing.T) {
	t.Parallel()

	encoded, err := EncodeGetTransactionDataArg(nil)
	require.NoError(t, err)
	require.NotEmpty(t, encoded)

	value, err := jsoncdc.Decode(nil, encoded)
	require.NoError(t, err)

	arr, ok := value.(cadence.Array)
	require.True(t, ok)
	assert.Empty(t, arr.Values)
}

// TestEncodeGetTransactionDataArg_NonEmpty verifies that encoding a non-empty slice produces
// a valid JSON-CDC array with the expected UInt64 values.
func TestEncodeGetTransactionDataArg_NonEmpty(t *testing.T) {
	t.Parallel()

	ids := []uint64{1, 42, 99}
	encoded, err := EncodeGetTransactionDataArg(ids)
	require.NoError(t, err)
	require.NotEmpty(t, encoded)

	value, err := jsoncdc.Decode(nil, encoded)
	require.NoError(t, err)

	arr, ok := value.(cadence.Array)
	require.True(t, ok)
	require.Len(t, arr.Values, 3)

	assert.Equal(t, cadence.UInt64(1), arr.Values[0])
	assert.Equal(t, cadence.UInt64(42), arr.Values[1])
	assert.Equal(t, cadence.UInt64(99), arr.Values[2])
}

// ===== DecodeTransactionDataResults =====

// TestDecodeTransactionDataResults_AllFound verifies that when all Optional elements are present
// (non-nil), DecodeTransactionDataResults returns a map with an entry for every ID.
func TestDecodeTransactionDataResults_AllFound(t *testing.T) {
	t.Parallel()

	sc := systemcontracts.SystemContractsForChain(flow.Testnet)
	owner := unittest.RandomAddressFixture()

	ids := []uint64{5, 7}
	comp5 := makeDecodeTransactionDataOptional(sc, 5, 1, 1000, 300, 100, owner, "A.abc.Contract.Handler", 55)
	comp7 := makeDecodeTransactionDataOptional(sc, 7, 2, 2000, 400, 150, owner, "A.def.Contract.Handler", 77)

	response := encodeOptionalArray(t, comp5, comp7)

	results, err := DecodeTransactionDataResults(response, ids)
	require.NoError(t, err)
	require.Len(t, results, 2)

	tx5, ok := results[5]
	require.True(t, ok)
	assert.Equal(t, uint64(5), tx5.ID)
	assert.Equal(t, access.ScheduledTxStatusScheduled, tx5.Status)
	assert.Equal(t, uint64(55), tx5.TransactionHandlerUUID)

	tx7, ok := results[7]
	require.True(t, ok)
	assert.Equal(t, uint64(7), tx7.ID)
	assert.Equal(t, uint64(77), tx7.TransactionHandlerUUID)
}

// TestDecodeTransactionDataResults_SomeNil verifies that nil Optional elements are omitted
// from the returned map, while non-nil elements are included.
func TestDecodeTransactionDataResults_SomeNil(t *testing.T) {
	t.Parallel()

	sc := systemcontracts.SystemContractsForChain(flow.Testnet)
	owner := unittest.RandomAddressFixture()

	ids := []uint64{1, 2, 3}
	comp1 := makeDecodeTransactionDataOptional(sc, 1, 1, 1000, 100, 50, owner, "A.abc.Contract.Handler", 1)
	nilOpt := cadence.NewOptional(nil)
	comp3 := makeDecodeTransactionDataOptional(sc, 3, 1, 1000, 100, 50, owner, "A.abc.Contract.Handler", 3)

	response := encodeOptionalArray(t, comp1, nilOpt, comp3)

	results, err := DecodeTransactionDataResults(response, ids)
	require.NoError(t, err)
	require.Len(t, results, 2)

	_, ok := results[2]
	assert.False(t, ok, "nil Optional for ID 2 should be omitted")

	_, ok = results[1]
	assert.True(t, ok)
	_, ok = results[3]
	assert.True(t, ok)
}

// TestDecodeTransactionDataResults_WrongCount verifies that an error is returned when the
// number of results does not match the number of IDs.
func TestDecodeTransactionDataResults_WrongCount(t *testing.T) {
	t.Parallel()

	sc := systemcontracts.SystemContractsForChain(flow.Testnet)
	owner := unittest.RandomAddressFixture()

	ids := []uint64{1, 2}
	// Only one element in the response instead of two.
	comp1 := makeDecodeTransactionDataOptional(sc, 1, 1, 1000, 100, 50, owner, "A.abc.Contract.Handler", 1)
	response := encodeOptionalArray(t, comp1)

	_, err := DecodeTransactionDataResults(response, ids)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected 2 results, got 1")
}

// TestDecodeTransactionDataResults_NonArray verifies that an error is returned when the
// response does not decode to a cadence.Array.
func TestDecodeTransactionDataResults_NonArray(t *testing.T) {
	t.Parallel()

	// Encode a single UInt64, not an array.
	notArray, err := jsoncdc.Encode(cadence.UInt64(42))
	require.NoError(t, err)

	_, err = DecodeTransactionDataResults(notArray, []uint64{1})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected Array result")
}

// TestDecodeTransactionDataResults_NonOptionalElement verifies that an error is returned when
// an array element is not a cadence.Optional.
func TestDecodeTransactionDataResults_NonOptionalElement(t *testing.T) {
	t.Parallel()

	// Encode an array with a UInt64 instead of Optional.
	notOptional := cadence.UInt64(99)
	arr := cadence.NewArray([]cadence.Value{notOptional})
	response, err := jsoncdc.Encode(arr)
	require.NoError(t, err)

	_, err = DecodeTransactionDataResults(response, []uint64{1})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected Optional at index 0")
}

// TestDecodeTransactionDataResults_MalformedComposite verifies that an error is returned when
// a non-nil Optional contains a value that is not a valid TransactionData composite.
func TestDecodeTransactionDataResults_MalformedComposite(t *testing.T) {
	t.Parallel()

	// An Optional wrapping a plain UInt64 (not a Composite).
	badOpt := cadence.NewOptional(cadence.UInt64(42))
	arr := cadence.NewArray([]cadence.Value{badOpt})
	response, err := jsoncdc.Encode(arr)
	require.NoError(t, err)

	_, err = DecodeTransactionDataResults(response, []uint64{1})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode TransactionData at index 0")
}

// TestDecodeTransactionDataResults_Empty verifies that an empty IDs slice returns an empty map.
func TestDecodeTransactionDataResults_Empty(t *testing.T) {
	t.Parallel()

	arr := cadence.NewArray([]cadence.Value{})
	response, err := jsoncdc.Encode(arr)
	require.NoError(t, err)

	results, err := DecodeTransactionDataResults(response, []uint64{})
	require.NoError(t, err)
	assert.Empty(t, results)
}

// ===== Test Helpers =====

// makeDecodeTransactionDataOptional creates a cadence Optional wrapping a TransactionData
// struct. Used for DecodeTransactionDataResults tests, which expect Optional-wrapped elements.
func makeDecodeTransactionDataOptional(
	sc *systemcontracts.SystemContracts,
	id uint64,
	priority uint8,
	timestamp uint64,
	executionEffort uint64,
	fees uint64,
	owner flow.Address,
	typeIdentifier string,
	uuid uint64,
) cadence.Value {
	addr := common.Address(sc.FlowTransactionScheduler.Address)
	loc := common.NewAddressLocation(nil, addr, sc.FlowTransactionScheduler.Name)

	priorityEnumType := cadence.NewEnumType(
		loc,
		"Priority",
		cadence.UInt8Type,
		[]cadence.Field{{Identifier: "rawValue", Type: cadence.UInt8Type}},
		nil,
	)
	priorityEnum := cadence.NewEnum([]cadence.Value{cadence.UInt8(priority)}).WithType(priorityEnumType)

	typ := cadence.NewStructType(
		loc,
		"TransactionData",
		[]cadence.Field{
			{Identifier: "id", Type: cadence.UInt64Type},
			{Identifier: "priority", Type: priorityEnumType},
			{Identifier: "timestamp", Type: cadence.UFix64Type},
			{Identifier: "executionEffort", Type: cadence.UInt64Type},
			{Identifier: "fees", Type: cadence.UFix64Type},
			{Identifier: "transactionHandlerOwner", Type: cadence.AddressType},
			{Identifier: "transactionHandlerTypeIdentifier", Type: cadence.StringType},
			{Identifier: "transactionHandlerUUID", Type: cadence.UInt64Type},
			{Identifier: "transactionHandlerPublicPath", Type: cadence.NewOptionalType(cadence.PublicPathType)},
		},
		nil,
	)
	comp := cadence.NewStruct([]cadence.Value{
		cadence.UInt64(id),
		priorityEnum,
		cadence.UFix64(timestamp),
		cadence.UInt64(executionEffort),
		cadence.UFix64(fees),
		cadence.NewAddress(owner),
		cadence.String(typeIdentifier),
		cadence.UInt64(uuid),
		cadence.NewOptional(nil),
	}).WithType(typ)
	return cadence.NewOptional(comp)
}

// encodeOptionalArray encodes a slice of cadence.Value elements as a JSON-CDC array.
func encodeOptionalArray(t *testing.T, elems ...cadence.Value) []byte {
	t.Helper()
	arr := cadence.NewArray(elems)
	encoded, err := jsoncdc.Encode(arr)
	require.NoError(t, err)
	return encoded
}
