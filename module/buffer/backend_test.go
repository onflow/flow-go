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

func (suite *BackendSuite) item() *item {
	parent := unittest.BlockHeaderFixture()
	return suite.itemWithParent(parent)
}

func (suite *BackendSuite) itemWithParent(parent *flow.Header) *item {
	header := unittest.BlockHeaderWithParentFixture(parent)
	return &item{
		header: flow.Slashable[*flow.Header]{
			OriginID: unittest.IdentifierFixture(),
			Message:  header,
		},
		payload: unittest.IdentifierFixture(),
	}
}

func (suite *BackendSuite) Add(item *item) {
	suite.backend.add(item.header, item.payload)
}

func (suite *BackendSuite) TestAdd() {
	expected := suite.item()
	suite.backend.add(expected.header, expected.payload)

	actual, ok := suite.backend.byID(expected.header.Message.ID())
	suite.Assert().True(ok)
	suite.Assert().Equal(expected, actual)

	byParent, ok := suite.backend.byParentID(expected.header.Message.ParentID)
	suite.Assert().True(ok)
	suite.Assert().Len(byParent, 1)
	suite.Assert().Equal(expected, byParent[0])
}

func (suite *BackendSuite) TestChildIndexing() {

	parent := suite.item()
	child1 := suite.itemWithParent(parent.header.Message)
	child2 := suite.itemWithParent(parent.header.Message)
	grandchild := suite.itemWithParent(child1.header.Message)
	unrelated := suite.item()

	suite.Add(child1)
	suite.Add(child2)
	suite.Add(grandchild)
	suite.Add(unrelated)

	suite.Run("retrieve by parent ID", func() {
		byParent, ok := suite.backend.byParentID(parent.header.Message.ID())
		suite.Assert().True(ok)
		// should only include direct children
		suite.Assert().Len(byParent, 2)
		suite.Assert().Contains(byParent, child1)
		suite.Assert().Contains(byParent, child2)
	})

	suite.Run("drop for parent ID", func() {
		suite.backend.dropForParent(parent.header.Message.ID())

		// should only drop direct children
		_, exists := suite.backend.byID(child1.header.Message.ID())
		suite.Assert().False(exists)
		_, exists = suite.backend.byID(child2.header.Message.ID())
		suite.Assert().False(exists)

		// grandchildren should be unaffected
		_, exists = suite.backend.byParentID(child1.header.Message.ID())
		suite.Assert().True(exists)
		_, exists = suite.backend.byID(grandchild.header.Message.ID())
		suite.Assert().True(exists)

		// nothing else should be affected
		_, exists = suite.backend.byID(unrelated.header.Message.ID())
		suite.Assert().True(exists)
	})
}

func (suite *BackendSuite) TestPruneByView() {

	const N = 100 // number of items we're testing with
	items := make([]*item, 0, N)

	// build a pending buffer
	for i := 0; i < N; i++ {

		// 10% of the time, add a new unrelated pending header
		if i%10 == 0 {
			item := suite.item()
			suite.Add(item)
			items = append(items, item)
			continue
		}

		// 90% of the time, build on an existing header
		if i%2 == 1 {
			parent := items[rand.Intn(len(items))]
			item := suite.itemWithParent(parent.header.Message)
			suite.Add(item)
			items = append(items, item)
		}
	}

	// pick a height to prune that's guaranteed to prune at least one item
	pruneAt := items[rand.Intn(len(items))].header.Message.View
	suite.backend.pruneByView(pruneAt)

	for _, item := range items {
		view := item.header.Message.View
		id := item.header.Message.ID()
		parentID := item.header.Message.ParentID

		// check that items below the prune view were removed
		if view <= pruneAt {
			_, exists := suite.backend.byID(id)
			suite.Assert().False(exists)
			_, exists = suite.backend.byParentID(parentID)
			suite.Assert().False(exists)
		}

		// check that other items were not removed
		if view > item.header.Message.View {
			_, exists := suite.backend.byID(id)
			suite.Assert().True(exists)
			_, exists = suite.backend.byParentID(parentID)
			suite.Assert().True(exists)
		}
	}
}
