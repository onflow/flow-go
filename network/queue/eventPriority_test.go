package queue

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	libp2pmessage "github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/network/codec"
)

// messagePriorityExpectation pairs a registered wire message type with the internal
// type the queue actually receives for it (the result of ToInternal, applied by the
// network layer before enqueueing) and the priority that internal type must map to.
type messagePriorityExpectation struct {
	wire     any      // a registered wire message type (model/messages)
	internal any      // the internal type produced by wire.ToInternal()
	want     Priority // expected priority of the internal type
}

// messagePriorityExpectations is the authoritative spec binding every registered
// wire message type to its queue priority. It must stay in sync with:
//   - network/codec/codes.go (the set of registered message codes)
//   - the ToInternal implementations in model/messages (the internal type each
//     wire message converts to before enqueueing)
//   - getPriorityByType (the priority of each internal type)
//
// Note the mapping is many-to-one: several wire types convert to the same internal
// type (e.g. consensus and cluster block votes both become [flow.BlockVote]).
var messagePriorityExpectations = []messagePriorityExpectation{
	// consensus
	{&messages.Proposal{}, &flow.Proposal{}, HighPriority},
	{&messages.BlockVote{}, &flow.BlockVote{}, HighPriority},
	{&messages.TimeoutObject{}, &model.TimeoutObject{}, HighPriority},

	// cluster consensus (effectively collections)
	{&messages.ClusterProposal{}, &cluster.Proposal{}, HighPriority},
	{&messages.ClusterBlockVote{}, &flow.BlockVote{}, HighPriority},
	{&messages.ClusterBlockResponse{}, &cluster.BlockResponse{}, HighPriority},
	{&messages.ClusterTimeoutObject{}, &model.TimeoutObject{}, HighPriority},

	// protocol state sync
	{&messages.SyncRequest{}, &flow.SyncRequest{}, LowPriority},
	{&messages.SyncResponse{}, &flow.SyncResponse{}, LowPriority},
	{&messages.RangeRequest{}, &flow.RangeRequest{}, MediumPriority},
	{&messages.BatchRequest{}, &flow.BatchRequest{}, MediumPriority},
	{&messages.BlockResponse{}, &flow.BlockResponse{}, HighPriority},

	// collection guarantees & transactions
	{&messages.CollectionGuarantee{}, &flow.CollectionGuarantee{}, HighPriority},
	{&messages.TransactionBody{}, &flow.TransactionBody{}, HighPriority},

	// core messages for execution & verification
	{&messages.ExecutionReceipt{}, &flow.ExecutionReceipt{}, HighPriority},
	{&messages.ResultApproval{}, &flow.ResultApproval{}, HighPriority},

	// data exchange for execution of blocks
	{&messages.ChunkDataRequest{}, &flow.ChunkDataRequest{}, HighPriority},
	{&messages.ChunkDataResponse{}, &flow.ChunkDataResponse{}, HighPriority},

	// result approvals
	{&messages.ApprovalRequest{}, &flow.ApprovalRequest{}, MediumPriority},
	{&messages.ApprovalResponse{}, &flow.ApprovalResponse{}, MediumPriority},

	// generic entity exchange engines
	{&messages.EntityRequest{}, &flow.EntityRequest{}, LowPriority},
	{&messages.EntityResponse{}, &flow.EntityResponse{}, LowPriority},

	// dkg
	{&messages.DKGMessage{}, &flow.DKGMessage{}, MediumPriority},

	// test message
	{&libp2pmessage.TestMessage{}, &flow.TestMessage{}, LowPriority},
}

// TestGetPriorityByType_InternalTypes verifies that each internal message type the
// queue can receive maps to its intended priority.
func TestGetPriorityByType_InternalTypes(t *testing.T) {
	for _, exp := range messagePriorityExpectations {
		t.Run(fmt.Sprintf("%T", exp.internal), func(t *testing.T) {
			require.Equal(t, exp.want, getPriorityByType(exp.internal),
				"internal type %T (wire type %T) must map to priority %d", exp.internal, exp.wire, exp.want)
		})
	}
}

// TestGetPriorityByType_CoversAllRegisteredWireTypes guards against the priority
// table silently rotting: it verifies that every message type registered with the
// codec is covered by messagePriorityExpectations, in both directions. If a new
// message type is registered without a priority decision, or an entry goes stale,
// this test fails.
func TestGetPriorityByType_CoversAllRegisteredWireTypes(t *testing.T) {
	byWireType := make(map[string]messagePriorityExpectation, len(messagePriorityExpectations))
	for _, exp := range messagePriorityExpectations {
		wireType := fmt.Sprintf("%T", exp.wire)
		_, dup := byWireType[wireType]
		require.False(t, dup, "duplicate expectation for wire type %s", wireType)
		byWireType[wireType] = exp
	}

	// CodeMin is a sentinel (iota + 1), not a valid message code; valid codes
	// start at CodeMin + 1. The range is not contiguous: deprecated codes (e.g. 15,
	// deprecated as of Mainnet 27) permanently error, so skip them. The stale-entry
	// check below still fails if a case is removed from the codec switch.
	for code := codec.CodeMin + 1; code < codec.CodeMax; code++ {
		wire, wireType, err := codec.InterfaceFromMessageCode(code)
		if codec.IsErrUnknownMsgCode(err) {
			continue
		}
		require.NoError(t, err)
		require.Equal(t, fmt.Sprintf("%T", wire), wireType, "codec should report the wire type name")

		exp, ok := byWireType[wireType]
		require.True(t, ok, "registered message code %d (wire type %s) has no priority expectation", code, wireType)
		delete(byWireType, wireType)

		// The priority is exercised through the exported path the queue uses.
		priority, err := GetEventPriority(QMessage{Payload: exp.internal, Size: 512})
		require.NoError(t, err)
		require.NotZero(t, priority)
	}

	require.Empty(t, byWireType, "stale priority expectations for unregistered wire types: %v", byWireType)
}

// TestGetEventPriority_TypeDominatesSize verifies that type priority strictly
// dominates size priority: any high-priority type outranks any medium-priority
// type at any size, and likewise medium over low.
func TestGetEventPriority_TypeDominatesSize(t *testing.T) {
	sizes := []struct {
		name string
		size int
	}{
		{"small", 512},        // <= 1 KiB
		{"medium", 512 * KiB}, // > 1 KiB, <= 1 MiB
		{"large", 2 * MiB},    // > 1 MiB
	}

	// expected priorities for the 3x3 type-by-size matrix under the weighted average
	cases := []struct {
		name    string
		payload any
		want    []Priority // one per size bucket: small, medium, large
	}{
		{"low type priority", &flow.SyncRequest{}, []Priority{3, 2, 1}},
		{"medium type priority", &flow.RangeRequest{}, []Priority{6, 5, 5}},
		{"high type priority", &flow.Proposal{}, []Priority{10, 9, 9}},
	}

	priorities := make(map[string]map[string]Priority)
	for _, tc := range cases {
		priorities[tc.name] = make(map[string]Priority)
		for i, s := range sizes {
			t.Run(tc.name+"/"+s.name, func(t *testing.T) {
				priority, err := GetEventPriority(QMessage{Payload: tc.payload, Size: s.size})
				require.NoError(t, err)
				require.Equal(t, tc.want[i], priority)
			})
			priorities[tc.name][s.name] = tc.want[i]
		}
	}

	// cross-type dominance holds at every size combination
	for _, hs := range sizes {
		for _, ms := range sizes {
			require.Greater(t, priorities["high type priority"][hs.name], priorities["medium type priority"][ms.name])
			for _, ls := range sizes {
				require.Greater(t, priorities["medium type priority"][ms.name], priorities["low type priority"][ls.name])
			}
		}
	}
}
