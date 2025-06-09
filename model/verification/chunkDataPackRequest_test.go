package verification_test

import (
	"bytes"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
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

	thisChunkIDReqInfo.Targets = thisChunkIDReqInfo.Targets.Sort(flow.Canonical[flow.Identity])

	require.Equal(t, thisChunkIDReqInfo.Agrees, thisReq1.Agrees.Union(thisReq2.Agrees))
	require.Equal(t, thisChunkIDReqInfo.Disagrees, thisReq1.Disagrees.Union(thisReq2.Disagrees))
	require.Equal(t, thisChunkIDReqInfo.Targets, thisReq1.Targets.Union(thisReq2.Targets))

	// there is only one request for otherChunkID, so its request info must be the same as the
	// original request.
	otherChunkIDReqInfo := reqInfoMap[otherChunkID]
	require.Equal(t, *otherChunkIDReqInfo, otherReq.ChunkDataPackRequestInfo)
}

// TestNewChunkDataPackRequest tests the NewChunkDataPackRequest constructor with valid and invalid inputs.
//
// Valid Case:
//
// 1. Valid input with non-empty locator, non-zero ChunkID, and all required lists:
//   - Should successfully construct a ChunkDataPackRequest.
//
// Invalid Cases:
//
// 2. Invalid input with empty locator:
//   - Should return an error indicating the locator is empty.
//
// 3. Invalid input with zero ChunkID:
//   - Should return an error indicating chunk ID must not be zero.
//
// 4. Invalid input with nil Agrees list:
//   - Should return an error indicating agrees list must not be nil.
//
// 5. Invalid input with nil Disagrees list:
//   - Should return an error indicating disagrees list must not be nil.
//
// 6. Invalid input with nil Targets list:
//   - Should return an error indicating targets list must not be nil.
//
// 7. Invalid input with non-execution node in Targets list:
//   - Should return an error indicating only execution node identities are allowed in the Targets list.
func TestNewChunkDataPackRequest(t *testing.T) {
	chunkDataPackRequestInfo := unittest.ChunkDataPackRequestInfoFixture()
	locator := *unittest.ChunkLocatorFixture(unittest.IdentifierFixture(), 0)

	t.Run("valid input with all required fields", func(t *testing.T) {
		request, err := verification.NewChunkDataPackRequest(
			verification.UntrustedChunkDataPackRequest{
				Locator:                  locator,
				ChunkDataPackRequestInfo: *chunkDataPackRequestInfo,
			},
		)

		require.NoError(t, err)
		require.NotNil(t, request)
	})

	t.Run("invalid input with empty locator", func(t *testing.T) {
		_, err := verification.NewChunkDataPackRequest(
			verification.UntrustedChunkDataPackRequest{
				Locator:                  chunks.Locator{},
				ChunkDataPackRequestInfo: *chunkDataPackRequestInfo,
			},
		)

		require.Error(t, err)
		require.Contains(t, err.Error(), "locator is empty")
	})

	t.Run("invalid input with zero ChunkID", func(t *testing.T) {
		info := *chunkDataPackRequestInfo
		info.ChunkID = flow.ZeroID

		_, err := verification.NewChunkDataPackRequest(
			verification.UntrustedChunkDataPackRequest{
				Locator:                  locator,
				ChunkDataPackRequestInfo: info,
			},
		)

		require.Error(t, err)
		require.Contains(t, err.Error(), "chunk ID must not be zero")
	})

	t.Run("invalid input with nil Agrees", func(t *testing.T) {
		info := *chunkDataPackRequestInfo
		info.Agrees = nil

		_, err := verification.NewChunkDataPackRequest(
			verification.UntrustedChunkDataPackRequest{
				Locator:                  locator,
				ChunkDataPackRequestInfo: info,
			},
		)

		require.Error(t, err)
		require.Contains(t, err.Error(), "agrees list must not be nil")
	})

	t.Run("invalid input with nil Disagrees", func(t *testing.T) {
		info := *chunkDataPackRequestInfo
		info.Disagrees = nil

		_, err := verification.NewChunkDataPackRequest(
			verification.UntrustedChunkDataPackRequest{
				Locator:                  locator,
				ChunkDataPackRequestInfo: info,
			},
		)

		require.Error(t, err)
		require.Contains(t, err.Error(), "disagrees list must not be nil")
	})

	t.Run("invalid input with nil Targets", func(t *testing.T) {
		info := *chunkDataPackRequestInfo
		info.Targets = nil

		_, err := verification.NewChunkDataPackRequest(
			verification.UntrustedChunkDataPackRequest{
				Locator:                  locator,
				ChunkDataPackRequestInfo: info,
			},
		)

		require.Error(t, err)
		require.Contains(t, err.Error(), "targets list must not be nil")
	})

	t.Run("invalid input with non-execution node in Targets list", func(t *testing.T) {
		info := *chunkDataPackRequestInfo

		// Append a non-execution identity
		info.Targets = append(info.Targets, unittest.IdentityFixture(
			unittest.WithNodeID(unittest.IdentifierFixture()),
			unittest.WithRole(flow.RoleAccess),
		))

		_, err := verification.NewChunkDataPackRequest(
			verification.UntrustedChunkDataPackRequest{
				Locator:                  locator,
				ChunkDataPackRequestInfo: info,
			},
		)

		require.Error(t, err)
		require.Contains(t, err.Error(), "only execution nodes identities must be provided in target list")
	})
}
