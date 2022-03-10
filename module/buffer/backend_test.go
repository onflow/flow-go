package buffer

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

type BackendSuite struct {
	suite.Suite
	backend *backend
}

func TestBackendSuite(t *testing.T) {
	suite.Run(t, new(BackendSuite))
}

func (suite *BackendSuite) SetupTest() {
	suite.backend = newBackend()
}

func (suite *BackendSuite) Item() *item {
	parent := unittest.BlockHeaderFixture()
	return suite.ItemWithParent(&parent)
}

func (suite *BackendSuite) ItemWithParent(parent *flow.Header) *item {
	header := unittest.BlockHeaderWithParentFixture(parent)
	return &item{
		header:   &header,
		payload:  unittest.IdentifierFixture(),
		originID: unittest.IdentifierFixture(),
	}
}

func (suite *BackendSuite) Add(item *item) {
	suite.backend.add(item.originID, item.header, item.payload)
}

func (suite *BackendSuite) TestAdd() {
	expected := suite.Item()
	suite.backend.add(expected.originID, expected.header, expected.payload)

	actual, ok := suite.backend.byID(expected.header.ID())
	suite.Assert().True(ok)
	suite.Assert().Equal(expected, actual)

	byParent, ok := suite.backend.byParentID(expected.header.ParentID)
	suite.Assert().True(ok)
	suite.Assert().Len(byParent, 1)
	suite.Assert().Equal(expected, byParent[0])
}

func (suite *BackendSuite) TestChildIndexing() {

	parent := suite.Item()
	child1 := suite.ItemWithParent(parent.header)
	child2 := suite.ItemWithParent(parent.header)
	grandchild := suite.ItemWithParent(child1.header)
	unrelated := suite.Item()

	suite.Add(child1)
	suite.Add(child2)
	suite.Add(grandchild)
	suite.Add(unrelated)

	suite.Run("retrieve by parent ID", func() {
		byParent, ok := suite.backend.byParentID(parent.header.ID())
		suite.Assert().True(ok)
		// should only include direct children
		suite.Assert().Len(byParent, 2)
		suite.Assert().Contains(byParent, child1)
		suite.Assert().Contains(byParent, child2)
	})

	suite.Run("drop for parent ID", func() {
		suite.backend.dropForParent(parent.header.ID())

		// should only drop direct children
		_, exists := suite.backend.byID(child1.header.ID())
		suite.Assert().False(exists)
		_, exists = suite.backend.byID(child2.header.ID())
		suite.Assert().False(exists)

		// grandchildren should be unaffected
		_, exists = suite.backend.byParentID(child1.header.ID())
		suite.Assert().True(exists)
		_, exists = suite.backend.byID(grandchild.header.ID())
		suite.Assert().True(exists)

		// nothing else should be affected
		_, exists = suite.backend.byID(unrelated.header.ID())
		suite.Assert().True(exists)
	})
}

func (suite *BackendSuite) TestPruneByView() {

	const N = 100 // number of items we're testing with
	items := make([]*item, 0, N)

	// build a pending buffer
	for i := 0; i < N; i++ {

		// 10% of the time, add a new unrelated pending block
		if i%10 == 0 {
			item := suite.Item()
			suite.Add(item)
			items = append(items, item)
			continue
		}

		// 90% of the time, build on an existing block
		if i%2 == 1 {
			parent := items[rand.Intn(len(items))]
			item := suite.ItemWithParent(parent.header)
			suite.Add(item)
			items = append(items, item)
		}
	}

	// pick a height to prune that's guaranteed to prune at least one item
	pruneAt := items[rand.Intn(len(items))].header.View
	suite.backend.pruneByView(pruneAt)

	for _, item := range items {
		view := item.header.View
		id := item.header.ID()
		parentID := item.header.ParentID

		// check that items below the prune view were removed
		if view <= pruneAt {
			_, exists := suite.backend.byID(id)
			suite.Assert().False(exists)
			_, exists = suite.backend.byParentID(parentID)
			suite.Assert().False(exists)
		}

		// check that other items were not removed
		if view > item.header.View {
			_, exists := suite.backend.byID(id)
			suite.Assert().True(exists)
			_, exists = suite.backend.byParentID(parentID)
			suite.Assert().True(exists)
		}
	}
}
