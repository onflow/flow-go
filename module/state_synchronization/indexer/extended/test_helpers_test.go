package extended

import (
	"testing"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/common"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
)

// MakeTransactionDataComposite creates a cadence Struct value representing a
// FlowTransactionScheduler.TransactionData with the given fields. sc is used to
// derive the correct contract address location required for JSON-CDC encoding.
func MakeTransactionDataComposite(
	sc *systemcontracts.SystemContracts,
	id uint64,
	priority uint8,
	timestamp uint64,
	executionEffort uint64,
	fees uint64,
	owner flow.Address,
	typeIdentifier string,
	uuid uint64,
) cadence.Composite {
	addr := common.Address(sc.FlowTransactionScheduler.Address)
	loc := common.NewAddressLocation(nil, addr, sc.FlowTransactionScheduler.Name)
	typ := cadence.NewStructType(
		loc,
		"TransactionData",
		[]cadence.Field{
			{Identifier: "id", Type: cadence.UInt64Type},
			{Identifier: "priority", Type: cadence.UInt8Type},
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
	return cadence.NewStruct([]cadence.Value{
		cadence.UInt64(id),
		cadence.UInt8(priority),
		cadence.UFix64(timestamp),
		cadence.UInt64(executionEffort),
		cadence.UFix64(fees),
		cadence.NewAddress(owner),
		cadence.String(typeIdentifier),
		cadence.UInt64(uuid),
		cadence.NewOptional(nil),
	}).WithType(typ)
}

// MakeJITScriptResponse encodes a slice of TransactionData composites as a JSON-CDC array,
// the format returned by a getTransactionData script execution.
func MakeJITScriptResponse(t *testing.T, composites ...cadence.Composite) []byte {
	t.Helper()
	values := make([]cadence.Value, len(composites))
	for i, c := range composites {
		values[i] = c
	}
	encoded, err := jsoncdc.Encode(cadence.NewArray(values))
	require.NoError(t, err)
	return encoded
}

// encodeUInt64Args returns a slice of JSON-CDC encoded UInt64 values, one per id.
// This mirrors the per-ID encoding used by buildArgs when constructing the
// arguments slice for ExecuteAtBlockHeight.
func encodeUInt64Args(t *testing.T, ids ...uint64) [][]byte {
	t.Helper()
	args, err := buildArgs(ids)
	require.NoError(t, err)
	return args
}
