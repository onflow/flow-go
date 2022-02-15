package verification_test

import (
	"bytes"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/order"
	"github.com/onflow/flow-go/model/verification"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestChunkDataPackRequestList_UniqueRequestInfo tests UniqueRequestInfo method of the ChunkDataPackRequest lists
// against combining and merging requests with duplicate chunk IDs.
func TestChunkDataPackRequestList_UniqueRequestInfo(t *testing.T) {
	reqList := verification.ChunkDataPackRequestList{}

	// adds two requests for same chunk ID
	thisChunkID := unittest.IdentifierFixture()
	thisReq1 := unittest.ChunkDataPackRequestFixture(unittest.WithChunkID(thisChunkID))
	thisReq2 := unittest.ChunkDataPackRequestFixture(unittest.WithChunkID(thisChunkID))
	require.NotEqual(t, thisReq1, thisReq2, "fixture request must be distinct")
	reqList = append(reqList, thisReq1)
	reqList = append(reqList, thisReq2)

	// creates a distinct request for distinct chunk ID
	otherChunkID := unittest.IdentifierFixture()
	require.NotEqual(t, thisChunkID, otherChunkID)
	otherReq := unittest.ChunkDataPackRequestFixture(unittest.WithChunkID(otherChunkID))
	reqList = append(reqList, otherReq)

	// extracts unique request info, must be only two request info one for thisChunkID, and
	// one for otherChunkID
	uniqueReqInfo := reqList.UniqueRequestInfo()
	require.Equal(t, 2, len(uniqueReqInfo)) // only two unique chunk IDs

	// lays out the list into map for testing.
	reqInfoMap := make(map[flow.Identifier]*verification.ChunkDataPackRequestInfo)
	for _, reqInfo := range uniqueReqInfo {
		reqInfoMap[reqInfo.ChunkID] = reqInfo
	}

	// since thisReq1 and thisReq2 share the same chunk ID, the request info must have union of their
	// agrees, disagrees, and targets.
	thisChunkIDReqInfo := reqInfoMap[thisChunkID]

	// pre-sorting these because the call to 'Union' will sort them, and the 'Equal' test requires
	// that the order be the same
	sort.Slice(thisChunkIDReqInfo.Agrees, func(p, q int) bool {
		return bytes.Compare(thisChunkIDReqInfo.Agrees[p][:], thisChunkIDReqInfo.Agrees[q][:]) < 0
	})
	sort.Slice(thisChunkIDReqInfo.Disagrees, func(p, q int) bool {
		return bytes.Compare(thisChunkIDReqInfo.Disagrees[p][:], thisChunkIDReqInfo.Disagrees[q][:]) < 0
	})

	thisChunkIDReqInfo.Targets = thisChunkIDReqInfo.Targets.Sort(order.Canonical)

	require.Equal(t, thisChunkIDReqInfo.Agrees, thisReq1.Agrees.Union(thisReq2.Agrees))
	require.Equal(t, thisChunkIDReqInfo.Disagrees, thisReq1.Disagrees.Union(thisReq2.Disagrees))
	require.Equal(t, thisChunkIDReqInfo.Targets, thisReq1.Targets.Union(thisReq2.Targets))

	// there is only one request for otherChunkID, so its request info must be the same as the
	// original request.
	otherChunkIDReqInfo := reqInfoMap[otherChunkID]
	require.Equal(t, *otherChunkIDReqInfo, otherReq.ChunkDataPackRequestInfo)
}
