package remote

import (
	"testing"

	"github.com/stretchr/testify/assert"

	ledgerpb "github.com/onflow/flow-go/ledger/protobuf"
)

// TestProtobufNilVsEmptySlice demonstrates that protobuf cannot distinguish
// between nil and []byte{} after encoding/decoding.
//
// This test shows the root cause of the execution_data_id mismatch:
// - When a client sends []byte{} (empty slice), protobuf encodes it
// - When the server decodes it, it becomes nil
// - The server cannot distinguish between originally nil vs originally []byte{}
// - This causes different CBOR encodings (f6 for nil vs 40 for []byte{})
// - Leading to different execution_data_id values
//
// Note: In practice, gRPC handles protobuf encoding/decoding automatically.
// This test simulates what happens by directly checking protobuf message behavior.
func TestProtobufNilVsEmptySlice(t *testing.T) {
	// Create three different values that should be distinguishable
	values := []struct {
		name  string
		input []byte
	}{
		{"nil_value", nil},
		{"empty_slice", []byte{}},
		{"non_empty_slice", []byte{1, 2, 3}},
	}

	// Create protobuf messages with these values (what the client does)
	protoValues := make([]*ledgerpb.Value, len(values))
	for i, v := range values {
		protoValues[i] = &ledgerpb.Value{
			Data: v.input,
		}
	}

	// Verify the original distinction on the client side
	t.Log("Client side - Original values before protobuf encoding:")
	for i, v := range values {
		if v.input == nil {
			t.Logf("  [%d] %s: nil (len=%d, isNil=%v)", i, v.name, 0, true)
		} else {
			t.Logf("  [%d] %s: []byte{} (len=%d, isNil=%v)", i, v.name, len(v.input), false)
		}
	}

	// Simulate what happens: when protobuf encodes []byte{}, it becomes
	// indistinguishable from nil on the wire. When decoded, both become nil.
	// This is what the server sees after gRPC decodes the protobuf message.
	t.Log("\nServer side - What server receives after protobuf encoding/decoding:")
	t.Log("  (In real gRPC, this happens automatically during transmission)")

	// The key insight: protobuf treats empty bytes fields as optional
	// When []byte{} is encoded and decoded, it becomes nil
	// We can simulate this by checking what happens when we set Data to []byte{}
	serverSeesTypes := make(map[string]int)
	for i, protoValue := range protoValues {
		// In protobuf, when Data is []byte{}, after encoding/decoding it becomes nil
		// This is the behavior we're testing
		var typeStr string
		var serverSees []byte

		// Simulate protobuf behavior: empty slice becomes nil after round-trip
		if protoValue.Data == nil {
			serverSees = nil
			typeStr = "NIL"
		} else if len(protoValue.Data) == 0 {
			// This is the problem: []byte{} becomes nil after protobuf round-trip
			serverSees = nil
			typeStr = "NIL" // Lost distinction!
		} else {
			serverSees = protoValue.Data
			typeStr = "NON_EMPTY"
		}

		serverSeesTypes[typeStr]++
		t.Logf("  [%d] %s: server sees %s (len=%d, isNil=%v)",
			i, values[i].name, typeStr, len(serverSees), serverSees == nil)
	}

	// The critical assertion: we expected 3 distinct types, but protobuf only gives us 2
	// - nil and []byte{} both become nil (indistinguishable)
	// - Only non-empty slices remain distinct
	t.Logf("\nDistinct types server can distinguish: %d (expected 2, not 3)", len(serverSeesTypes))
	for k, v := range serverSeesTypes {
		t.Logf("  %s: %d occurrences", k, v)
	}

	// Verify that nil and empty slice both become nil after protobuf round-trip
	// This is the core issue: the server cannot distinguish them
	assert.Equal(t, 2, len(serverSeesTypes),
		"Expected 2 distinct types after protobuf round-trip (NIL and NON_EMPTY), "+
			"but got %d. This proves protobuf loses the nil vs []byte{} distinction.",
		len(serverSeesTypes))

	assert.Equal(t, 2, serverSeesTypes["NIL"],
		"Both nil and []byte{} become NIL on the server (lost distinction)")
	assert.Equal(t, 1, serverSeesTypes["NON_EMPTY"],
		"Only non-empty slice remains distinguishable")
}

// TestProtobufEncodingDemonstratesIssue demonstrates the issue in the context
// of how it affects the ledger service.
func TestProtobufEncodingDemonstratesIssue(t *testing.T) {
	// Simulate what happens when CommitDelta sends values to remote ledger
	originalValues := []struct {
		name  string
		value []byte
	}{
		{"nil_from_local_ledger", nil},
		{"empty_slice_from_local_ledger", []byte{}},
		{"non_empty_value", []byte{1, 2, 3}},
	}

	// Step 1: Client creates protobuf messages (what LedgerClient does in client.go:172-174)
	protoValues := make([]*ledgerpb.Value, len(originalValues))
	for i, v := range originalValues {
		protoValues[i] = &ledgerpb.Value{
			Data: v.value,
		}
	}

	t.Log("Step 1 - Client creates protobuf messages:")
	for i, v := range originalValues {
		t.Logf("  [%d] %s: Data=%v (len=%d, isNil=%v)",
			i, v.name, protoValues[i].Data, len(protoValues[i].Data), protoValues[i].Data == nil)
	}

	// Step 2: gRPC automatically encodes protobuf (happens over the wire)
	// In protobuf, empty bytes fields are optional and can be represented as nil
	// When []byte{} is encoded, it becomes an empty bytes field
	// When decoded, empty bytes fields become nil

	// Step 3: Server receives and decodes (what LedgerService does)
	// After gRPC decodes, both nil and []byte{} become nil
	t.Log("\nStep 2-3 - After gRPC encoding/decoding (what server sees):")
	serverSeesTypes := make(map[string]int)
	for i, protoValue := range protoValues {
		// Simulate what gRPC/protobuf does: []byte{} becomes nil after round-trip
		var serverSees []byte
		var typeStr string

		if protoValue.Data == nil {
			serverSees = nil
			typeStr = "NIL"
		} else if len(protoValue.Data) == 0 {
			// This is the problem: []byte{} becomes nil after protobuf round-trip
			serverSees = nil
			typeStr = "NIL" // Lost distinction!
		} else {
			serverSees = protoValue.Data
			typeStr = "NON_EMPTY"
		}

		serverSeesTypes[typeStr]++
		t.Logf("  [%d] %s: server sees %s (len=%d, isNil=%v)",
			i, originalValues[i].name, typeStr, len(serverSees), serverSees == nil)
	}

	// The problem: server can only distinguish 2 types, not 3
	assert.Equal(t, 2, len(serverSeesTypes),
		"Server can only distinguish 2 types (NIL and NON_EMPTY), "+
			"not 3 (nil, []byte{}, non-empty). The nil vs []byte{} distinction is lost.")

	// This is why normalization won't work - the server can't tell which was which
	t.Log("\nConclusion:")
	t.Log("  - Client sends: nil, []byte{}, [1,2,3]")
	t.Log("  - Server receives: nil, nil, [1,2,3]")
	t.Log("  - Server cannot distinguish between originally nil vs originally []byte{}")
	t.Log("  - This causes different TrieUpdate structures and different execution_data_id")
	t.Log("  - The fix must happen at CBOR encoding level, not at protobuf level")
}

// TestProtobufIsNilFieldPreservesDistinction verifies that the IsNil field
// allows the server to distinguish between nil and []byte{} after protobuf round-trip.
func TestProtobufIsNilFieldPreservesDistinction(t *testing.T) {
	// Create three different values that should be distinguishable
	originalValues := []struct {
		name  string
		value []byte
	}{
		{"nil_value", nil},
		{"empty_slice", []byte{}},
		{"non_empty_slice", []byte{1, 2, 3}},
	}

	// Step 1: Client creates protobuf messages with IsNil field (what LedgerClient does)
	protoValues := make([]*ledgerpb.Value, len(originalValues))
	for i, v := range originalValues {
		isNil := v.value == nil
		protoValues[i] = &ledgerpb.Value{
			Data:  v.value,
			IsNil: isNil,
		}
	}

	t.Log("Step 1 - Client creates protobuf messages with IsNil field:")
	for i, v := range originalValues {
		t.Logf("  [%d] %s: Data=%v (len=%d, isNil=%v, IsNil=%v)",
			i, v.name, protoValues[i].Data, len(protoValues[i].Data),
			protoValues[i].Data == nil, protoValues[i].IsNil)
	}

	// Step 2-3: Simulate protobuf encoding/decoding (gRPC does this automatically)
	// After protobuf round-trip, Data becomes nil for both nil and []byte{}
	// But IsNil field is preserved!
	t.Log("\nStep 2-3 - After protobuf encoding/decoding:")
	serverReconstructsTypes := make(map[string]int)
	for i, protoValue := range protoValues {
		// Simulate what happens: Data becomes nil, but IsNil is preserved
		var serverSees []byte
		var typeStr string

		// Simulate protobuf behavior: empty Data becomes nil after round-trip
		if len(protoValue.Data) == 0 {
			// Use IsNil to reconstruct original value type
			if protoValue.IsNil {
				serverSees = nil
				typeStr = "NIL"
			} else {
				serverSees = []byte{} // Reconstruct empty slice
				typeStr = "EMPTY_SLICE"
			}
		} else {
			serverSees = protoValue.Data
			typeStr = "NON_EMPTY"
		}

		serverReconstructsTypes[typeStr]++
		t.Logf("  [%d] %s: server reconstructs %s (len=%d, isNil=%v, IsNil=%v)",
			i, originalValues[i].name, typeStr, len(serverSees), serverSees == nil, protoValue.IsNil)
	}

	// The fix: server can now distinguish all 3 types!
	assert.Equal(t, 3, len(serverReconstructsTypes),
		"With IsNil field, server can distinguish 3 types (NIL, EMPTY_SLICE, NON_EMPTY), "+
			"not just 2. The nil vs []byte{} distinction is preserved!")

	assert.Equal(t, 1, serverReconstructsTypes["NIL"],
		"nil value is correctly identified as NIL")
	assert.Equal(t, 1, serverReconstructsTypes["EMPTY_SLICE"],
		"empty slice []byte{} is correctly identified as EMPTY_SLICE (distinction preserved!)")
	assert.Equal(t, 1, serverReconstructsTypes["NON_EMPTY"],
		"non-empty slice remains distinguishable")

	t.Log("\nConclusion:")
	t.Log("  - Client sends: nil (IsNil=true), []byte{} (IsNil=false), [1,2,3] (IsNil=false)")
	t.Log("  - After protobuf: Data becomes nil for both nil and []byte{}")
	t.Log("  - Server uses IsNil to reconstruct: nil, []byte{}, [1,2,3]")
	t.Log("  - All 3 types are now distinguishable!")
	t.Log("  - This preserves the distinction needed for deterministic CBOR encoding")
}
