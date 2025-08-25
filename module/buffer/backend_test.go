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
	backend *backend[*flow.Proposal]
}

func TestBackendSuite(t *testing.T) {
	suite.Run(t, new(BackendSuite))
}

func (suite *BackendSuite) SetupTest() {
	suite.backend = newBackend[*flow.Proposal]()
}

func (suite *BackendSuite) item() *item[*flow.Proposal] {
	parent := unittest.BlockHeaderFixture()
	return suite.itemWithParent(parent)
}

func (suite *BackendSuite) itemWithParent(parent *flow.Header) *item[*flow.Proposal] {
	block := unittest.BlockWithParentFixture(parent)
	return &item[*flow.Proposal]{
		view:     block.View,
		parentID: block.ParentID,
		block: flow.Slashable[*flow.Proposal]{
			OriginID: unittest.IdentifierFixture(),
			Message:  unittest.ProposalFromBlock(block),
		},
	}
}

func (suite *BackendSuite) Add(item *item[*flow.Proposal]) {
	suite.backend.add(item.block)
}

func (suite *BackendSuite) TestAdd() {
	expected := suite.item()
	suite.backend.add(expected.block)

	actual, ok := suite.backend.byID(expected.block.Message.Block.ID())
	suite.Assert().True(ok)
	suite.Assert().Equal(expected, actual)

	byParent, ok := suite.backend.byParentID(expected.parentID)
	suite.Assert().True(ok)
	suite.Assert().Len(byParent, 1)
	suite.Assert().Equal(expected, byParent[0])
}

func (suite *BackendSuite) TestChildIndexing() {

	parent := suite.item()
	child1 := suite.itemWithParent(parent.block.Message.Block.ToHeader())
	child2 := suite.itemWithParent(parent.block.Message.Block.ToHeader())
	grandchild := suite.itemWithParent(child1.block.Message.Block.ToHeader())
	unrelated := suite.item()

	suite.Add(child1)
	suite.Add(child2)
	suite.Add(grandchild)
	suite.Add(unrelated)

	suite.Run("retrieve by parent ID", func() {
		byParent, ok := suite.backend.byParentID(parent.block.Message.Block.ID())
		suite.Assert().True(ok)
		// should only include direct children
		suite.Assert().Len(byParent, 2)
		suite.Assert().Contains(byParent, child1)
		suite.Assert().Contains(byParent, child2)
	})

	suite.Run("drop for parent ID", func() {
		suite.backend.dropForParent(parent.block.Message.Block.ID())

		// should only drop direct children
		_, exists := suite.backend.byID(child1.block.Message.Block.ID())
		suite.Assert().False(exists)
		_, exists = suite.backend.byID(child2.block.Message.Block.ID())
		suite.Assert().False(exists)

		// grandchildren should be unaffected
		_, exists = suite.backend.byParentID(child1.block.Message.Block.ID())
		suite.Assert().True(exists)
		_, exists = suite.backend.byID(grandchild.block.Message.Block.ID())
		suite.Assert().True(exists)

		// nothing else should be affected
		_, exists = suite.backend.byID(unrelated.block.Message.Block.ID())
		suite.Assert().True(exists)
	})
}

func (suite *BackendSuite) TestPruneByView() {

	const N = 100 // number of items we're testing with
	items := make([]*item[*flow.Proposal], 0, N)

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
			item := suite.itemWithParent(parent.block.Message.Block.ToHeader())
			suite.Add(item)
			items = append(items, item)
		}
	}

	// pick a height to prune that's guaranteed to prune at least one item
	pruneAt := items[rand.Intn(len(items))].view
	suite.backend.pruneByView(pruneAt)

	for _, item := range items {
		view := item.view
		id := item.block.Message.Block.ID()
		parentID := item.parentID

		// check that items below the prune view were removed
		if view <= pruneAt {
			_, exists := suite.backend.byID(id)
			suite.Assert().False(exists)
			_, exists = suite.backend.byParentID(parentID)
			suite.Assert().False(exists)
		}

		// check that other items were not removed
		if view > item.view {
			_, exists := suite.backend.byID(id)
			suite.Assert().True(exists)
			_, exists = suite.backend.byParentID(parentID)
			suite.Assert().True(exists)
		}
	}
}
