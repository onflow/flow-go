package flow

// headerBodyFieldBitIndex enumerates required fields in HeaderBody so that HeaderBodyBuilder
// can enforce that all required fields are explicitly set (including to zero values) prior to building.
type headerBodyFieldBitIndex int

const (
	chainIDFieldBitIndex headerBodyFieldBitIndex = iota
	parentIDFieldBitIndex
	heightFieldBitIndex
	timestampFieldBitIndex
	viewFieldBitIndex
	parentViewFieldBitIndex
	parentVoterIndicesFieldBitIndex
	parentVoterSigDataFieldBitIndex
	proposerIDFieldBitIndex
	numHeaderBodyFields // always keep this last
)

//// TestClusterBlockMalleability checks that cluster.Block is not malleable: any change in its data
//// should result in a different ID.
//// Because our NewHeaderBody constructor enforces ParentView < View we use
//// WithFieldGenerator to safely pass it.
//func TestClusterBlockMalleability(t *testing.T) {
//
//	clusterBlock := unittest.ClusterBlockFixture()
//	unittest.RequireEntityNonMalleable(
//		t,
//		clusterBlock,
//		unittest.WithFieldGenerator("Header.ParentView", func() uint64 {
//			return clusterBlock.Header.View - 1 // ParentView must stay below View, so set it to View-1
//		}),
//		unittest.WithFieldGenerator("Header.Timestamp", func() time.Time { return time.Now().UTC() }),
//		unittest.WithFieldGenerator("Payload.Collection", func() flow.Collection {
//			return unittest.CollectionFixture(3)
//		}),
//	)
//}
