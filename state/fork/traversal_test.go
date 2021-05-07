package fork

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	mockstorage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestTraverse(t *testing.T) {
	suite.Run(t, new(TraverseSuite))
}

type TraverseSuite struct {
	suite.Suite

	byID     map[flow.Identifier]*flow.Header
	byHeight map[uint64]*flow.Header
	headers  *mockstorage.Headers
	genesis  flow.Header
}

func (s *TraverseSuite) SetupTest() {
	// create a storage.Headers mock with a backing map
	s.byID = make(map[flow.Identifier]*flow.Header)
	s.byHeight = make(map[uint64]*flow.Header)
	s.headers = new(mockstorage.Headers)
	s.headers.On("ByBlockID", mock.Anything).Return(
		func(id flow.Identifier) *flow.Header {
			return s.byID[id]
		},
		func(id flow.Identifier) error {
			_, ok := s.byID[id]
			if !ok {
				return storage.ErrNotFound
			}
			return nil
		})

	// populate the mocked header storage with genesis and 10 child blocks
	genesis := unittest.BlockHeaderFixture()
	genesis.Height = 0
	s.byID[genesis.ID()] = &genesis
	s.byHeight[genesis.Height] = &genesis
	s.genesis = genesis

	parent := &genesis
	for i := 0; i < 10; i++ {
		child := unittest.BlockHeaderWithParentFixture(parent)
		s.byID[child.ID()] = &child
		s.byHeight[child.Height] = &child
		parent = &child
	}
}

// TestTraverse_MissingForkHead tests the behaviour of block traversing for the
// case where the fork head is an unknown block. We expect:
// * traversal errors
// * traversal does _not_ invoke the visitor callback
func (s *TraverseSuite) TestTraverse_MissingForkHead() {
	unknownForkHead := unittest.IdentifierFixture()

	visitor := func(_ *flow.Header) error {
		s.Require().Fail("visitor should not be called")
		return nil
	}

	s.Run("TraverseBackward from non-existent start block", func() {
		err := TraverseBackward(s.headers, unknownForkHead, visitor, IncludingBlock(s.genesis.ID()))
		s.Require().Error(err)
	})

	// should return error and not call callback when start block doesn't exist
	s.Run("non-existent start block", func() {
		err := TraverseForward(s.headers, unknownForkHead, visitor, IncludingBlock(s.genesis.ID()))
		s.Require().Error(err)
	})
}

// TestTraverse_VisitorError tests the behaviour of block traversing for the
// case where the visitor callback errors. We expect
// * the visitor error is propagated by the block traversal
func (s *TraverseSuite) TestTraverse_VisitorError() {
	forkHead := s.byHeight[8].ID()

	visitorError := errors.New("some visitor error")
	visitor := func(_ *flow.Header) error { return visitorError }

	s.Run("TraverseBackward with visitor error", func() {
		err := TraverseBackward(s.headers, forkHead, visitor, IncludingHeight(1))
		s.Require().ErrorIs(err, visitorError)
	})

	s.Run("TraverseForward with visitor error", func() {
		err := TraverseForward(s.headers, forkHead, visitor, IncludingHeight(1))
		s.Require().ErrorIs(err, visitorError)
	})
}

// TestTraverse_UnknownTerminalBlock tests the behaviour of block traversing
// for the case where the terminal block is unknown
func (s *TraverseSuite) TestTraverse_UnknownTerminalBlock() {
	forkHead := s.byHeight[8].ID()
	unknownTerminal := unittest.IdentifierFixture()
	visitor := func(_ *flow.Header) error {
		s.Require().Fail("visitor should not be called")
		return nil
	}

	s.Run("backwards traversal with non-existent terminal block (inclusive)", func() {
		err := TraverseBackward(s.headers, forkHead, visitor, IncludingBlock(unknownTerminal))
		s.Require().Error(err)
	})

	s.Run("backwards traversal with non-existent terminal block (exclusive)", func() {
		err := TraverseBackward(s.headers, forkHead, visitor, ExcludingBlock(unknownTerminal))
		s.Require().Error(err)
	})

	s.Run("forward traversal with non-existent terminal block (inclusive)", func() {
		err := TraverseForward(s.headers, forkHead, visitor, IncludingBlock(unknownTerminal))
		s.Require().Error(err)
	})

	s.Run("forward traversal with non-existent terminal block (exclusive)", func() {
		err := TraverseForward(s.headers, forkHead, visitor, ExcludingBlock(unknownTerminal))
		s.Require().Error(err)
	})
}

// TestTraverseBackward_DownToBlock tests different happy-path scenarios for reverse
// block traversing where the terminal block (lowest block) is specified by its ID
func (s *TraverseSuite) TestTraverseBackward_DownToBlock() {

	// edge case where start == end and the end block is _excluded_
	s.Run("zero blocks to traverse", func() {
		start := s.byHeight[5].ID()
		end := s.byHeight[5].ID()

		err := TraverseBackward(s.headers, start, func(header *flow.Header) error {
			s.Require().Fail("visitor should not be called")
			return nil
		}, ExcludingBlock(end))
		s.Require().NoError(err)
	})

	// edge case where start == end and the end block is _included_
	s.Run("single block to traverse", func() {
		start := s.byHeight[5].ID()
		end := s.byHeight[5].ID()

		called := 0
		err := TraverseBackward(s.headers, start, func(header *flow.Header) error {
			// should call callback for single block in traversal path
			s.Require().Equal(start, header.ID())
			// track calls - should only be called once
			called++
			return nil
		}, IncludingBlock(end))
		s.Require().NoError(err)
		s.Require().Equal(1, called)
	})

	// should call the callback exactly once for each block in traversal path
	// and not return an error
	s.Run("multi-block traversal including terminal block", func() {
		startHeight := uint64(8)
		endHeight := uint64(4)

		start := s.byHeight[startHeight].ID()
		end := s.byHeight[endHeight].ID()

		// assert that we are receiving the correct block at each height
		height := startHeight
		err := TraverseBackward(s.headers, start, func(header *flow.Header) error {
			expectedID := s.byHeight[height].ID()
			s.Require().Equal(expectedID, header.ID())
			height--
			return nil
		}, IncludingBlock(end))
		s.Require().NoError(err)
		s.Require().Equal(endHeight, height+1)
	})

	// should call the callback exactly once for each block in traversal path
	// and not return an error
	s.Run("multi-block traversal excluding terminal block", func() {
		startHeight := uint64(8)
		endHeight := uint64(4)

		start := s.byHeight[startHeight].ID()
		end := s.byHeight[endHeight].ID()

		// assert that we are receiving the correct block at each height
		height := startHeight
		err := TraverseBackward(s.headers, start, func(header *flow.Header) error {
			expectedID := s.byHeight[height].ID()
			s.Require().Equal(expectedID, header.ID())
			height--
			return nil
		}, ExcludingBlock(end))
		s.Require().NoError(err)
		s.Require().Equal(endHeight, height)
	})

	// edge case where we traverse only the genesis block
	s.Run("traversing only genesis block", func() {
		genesisID := s.genesis.ID()

		called := 0
		err := TraverseBackward(s.headers, genesisID, func(header *flow.Header) error {
			// should call callback for single block in traversal path
			s.Require().Equal(genesisID, header.ID())
			// track calls - should only be called once
			called++
			return nil
		}, IncludingBlock(genesisID))
		s.Require().NoError(err)
		s.Require().Equal(1, called)
	})
}

// TestTraverseBackward_DownToHeight tests different happy-path scenarios for reverse
// block traversing where the terminal block (lowest block) is specified by height
func (s *TraverseSuite) TestTraverseBackward_DownToHeight() {

	// edge case where start == end and the end block is _excluded_
	s.Run("zero blocks to traverse", func() {
		startHeight := uint64(5)
		start := s.byHeight[startHeight].ID()

		err := TraverseBackward(s.headers, start, func(header *flow.Header) error {
			s.Require().Fail("visitor should not be called")
			return nil
		}, ExcludingHeight(startHeight))
		s.Require().NoError(err)
	})

	// edge case where start == end and the end block is _included_
	s.Run("single block to traverse", func() {
		startHeight := uint64(5)
		start := s.byHeight[startHeight].ID()

		called := 0
		err := TraverseBackward(s.headers, start, func(header *flow.Header) error {
			// should call callback for single block in traversal path
			s.Require().Equal(start, header.ID())
			// track calls - should only be called once
			called++
			return nil
		}, IncludingHeight(startHeight))
		s.Require().NoError(err)
		s.Require().Equal(1, called)
	})

	// should call the callback exactly once for each block in traversal path
	// and not return an error
	s.Run("multi-block traversal including terminal block", func() {
		startHeight := uint64(8)
		endHeight := uint64(4)
		start := s.byHeight[startHeight].ID()

		// assert that we are receiving the correct block at each height
		height := startHeight
		err := TraverseBackward(s.headers, start, func(header *flow.Header) error {
			expectedID := s.byHeight[height].ID()
			s.Require().Equal(expectedID, header.ID())
			height--
			return nil
		}, IncludingHeight(endHeight))
		s.Require().NoError(err)
		s.Require().Equal(endHeight, height+1)
	})

	// should call the callback exactly once for each block in traversal path
	// and not return an error
	s.Run("multi-block traversal excluding terminal block", func() {
		startHeight := uint64(8)
		endHeight := uint64(4)
		start := s.byHeight[startHeight].ID()

		// assert that we are receiving the correct block at each height
		height := startHeight
		err := TraverseBackward(s.headers, start, func(header *flow.Header) error {
			expectedID := s.byHeight[height].ID()
			s.Require().Equal(expectedID, header.ID())
			height--
			return nil
		}, ExcludingHeight(endHeight))
		s.Require().NoError(err)
		s.Require().Equal(endHeight, height)
	})

	// edge case where we traverse only the genesis block
	s.Run("traversing only genesis block", func() {
		genesisID := s.genesis.ID()

		called := 0
		err := TraverseBackward(s.headers, genesisID, func(header *flow.Header) error {
			// should call callback for single block in traversal path
			s.Require().Equal(genesisID, header.ID())
			// track calls - should only be called once
			called++
			return nil
		}, IncludingHeight(s.genesis.Height))
		s.Require().NoError(err)
		s.Require().Equal(1, called)
	})
}

// TestTraverseForward_UpFromBlock tests different happy-path scenarios for parent-first
// block traversing where the terminal block (lowest block) is specified by its ID
func (s *TraverseSuite) TestTraverseForward_UpFromBlock() {

	// edge case where start == end and the terminal block is _excluded_
	s.Run("zero blocks to traverse", func() {
		upperBlock := s.byHeight[5].ID()
		lowerBlock := s.byHeight[5].ID()

		err := TraverseForward(s.headers, upperBlock, func(header *flow.Header) error {
			s.Require().Fail("visitor should not be called")
			return nil
		}, ExcludingBlock(lowerBlock))
		s.Require().NoError(err)
	})

	// should call the callback exactly once and not return an error when start == end
	s.Run("single-block traversal", func() {
		upperBlock := s.byHeight[5].ID()
		lowerBlock := s.byHeight[5].ID()

		called := 0
		err := TraverseForward(s.headers, upperBlock, func(header *flow.Header) error {
			// should call callback for single block in traversal path
			s.Require().Equal(upperBlock, header.ID())
			// track calls - should only be called once
			called++
			return nil
		}, IncludingBlock(lowerBlock))
		s.Require().NoError(err)
		s.Require().Equal(1, called)
	})

	// should call the callback exactly once for each block in traversal path
	// and not return an error
	s.Run("multi-block traversal including terminal block", func() {
		upperHeight := uint64(8)
		lowerHeight := uint64(4)

		upperBlock := s.byHeight[upperHeight].ID()
		lowerBlock := s.byHeight[lowerHeight].ID()

		// assert that we are receiving the correct block at each height
		height := lowerHeight
		err := TraverseForward(s.headers, upperBlock, func(header *flow.Header) error {
			expectedID := s.byHeight[height].ID()
			s.Require().Equal(height, header.Height)
			s.Require().Equal(expectedID, header.ID())
			height++
			return nil
		}, IncludingBlock(lowerBlock))
		s.Require().NoError(err)
		s.Require().Equal(height, upperHeight+1)
	})

	// should call the callback exactly once for each block in traversal path
	// and not return an error
	s.Run("multi-block traversal excluding terminal block", func() {
		upperHeight := uint64(8)
		lowerHeight := uint64(4)

		upperBlock := s.byHeight[upperHeight].ID()
		lowerBlock := s.byHeight[lowerHeight].ID()

		// assert that we are receiving the correct block at each height
		height := lowerHeight + 1
		err := TraverseForward(s.headers, upperBlock, func(header *flow.Header) error {
			expectedID := s.byHeight[height].ID()
			s.Require().Equal(height, header.Height)
			s.Require().Equal(expectedID, header.ID())
			height++
			return nil
		}, ExcludingBlock(lowerBlock))
		s.Require().NoError(err)
		s.Require().Equal(height, upperHeight+1)
	})

	// edge case where we traverse only the genesis block
	s.Run("traversing only genesis block", func() {
		genesisID := s.genesis.ID()

		called := 0
		err := TraverseForward(s.headers, genesisID, func(header *flow.Header) error {
			// should call callback for single block in traversal path
			s.Require().Equal(genesisID, header.ID())
			// track calls - should only be called once
			called++
			return nil
		}, IncludingBlock(genesisID))
		s.Require().NoError(err)
		s.Require().Equal(1, called)
	})
}

// TestTraverseForward_UpFromHeight tests different happy-path scenarios for parent-first
// block traversing where the terminal block (lowest block) is specified by height
func (s *TraverseSuite) TestTraverseForward_UpFromHeight() {

	// edge case where start == end and the terminal block is _excluded_
	s.Run("zero blocks to traverse", func() {
		upperHeight := uint64(5)
		upperBlock := s.byHeight[upperHeight].ID()

		err := TraverseForward(s.headers, upperBlock, func(header *flow.Header) error {
			s.Require().Fail("visitor should not be called")
			return nil
		}, ExcludingHeight(upperHeight))
		s.Require().NoError(err)
	})

	// should call the callback exactly once and not return an error when start == end
	s.Run("single-block traversal", func() {
		upperHeight := uint64(5)
		upperBlock := s.byHeight[upperHeight].ID()

		called := 0
		err := TraverseForward(s.headers, upperBlock, func(header *flow.Header) error {
			// should call callback for single block in traversal path
			s.Require().Equal(upperBlock, header.ID())
			// track calls - should only be called once
			called++
			return nil
		}, IncludingHeight(upperHeight))
		s.Require().NoError(err)
		s.Require().Equal(1, called)
	})

	// should call the callback exactly once for each block in traversal path
	// and not return an error
	s.Run("multi-block traversal including terminal block", func() {
		upperHeight := uint64(8)
		lowerHeight := uint64(4)
		upperBlock := s.byHeight[upperHeight].ID()

		// assert that we are receiving the correct block at each height
		height := lowerHeight
		err := TraverseForward(s.headers, upperBlock, func(header *flow.Header) error {
			expectedID := s.byHeight[height].ID()
			s.Require().Equal(height, header.Height)
			s.Require().Equal(expectedID, header.ID())
			height++
			return nil
		}, IncludingHeight(lowerHeight))
		s.Require().NoError(err)
		s.Require().Equal(height, upperHeight+1)
	})

	// should call the callback exactly once for each block in traversal path
	// and not return an error
	s.Run("multi-block traversal excluding terminal block", func() {
		upperHeight := uint64(8)
		lowerHeight := uint64(4)
		upperBlock := s.byHeight[upperHeight].ID()

		// assert that we are receiving the correct block at each height
		height := lowerHeight + 1
		err := TraverseForward(s.headers, upperBlock, func(header *flow.Header) error {
			expectedID := s.byHeight[height].ID()
			s.Require().Equal(height, header.Height)
			s.Require().Equal(expectedID, header.ID())
			height++
			return nil
		}, ExcludingHeight(lowerHeight))
		s.Require().NoError(err)
		s.Require().Equal(height, upperHeight+1)
	})

	// edge case where we traverse only the genesis block
	s.Run("traversing only genesis block", func() {
		genesisID := s.genesis.ID()

		called := 0
		err := TraverseForward(s.headers, genesisID, func(header *flow.Header) error {
			// should call callback for single block in traversal path
			s.Require().Equal(genesisID, header.ID())
			// track calls - should only be called once
			called++
			return nil
		}, IncludingHeight(s.genesis.Height))
		s.Require().NoError(err)
		s.Require().Equal(1, called)
	})
}

// TestTraverse_OnDifferentForkThanTerminalBlock tests that block traversing
// errors if the end block is on a different Fork. This is only applicable
// when terminal block (lowest block) is specified by its ID.
func (s *TraverseSuite) TestTraverse_OnDifferentForkThanTerminalBlock() {
	forkHead := s.byHeight[8].ID()
	noopVisitor := func(header *flow.Header) error { return nil }

	// make other fork
	otherForkHead := &s.genesis
	otherForkByHeight := make(map[uint64]*flow.Header)
	for i := 0; i < 10; i++ {
		child := unittest.BlockHeaderWithParentFixture(otherForkHead)
		s.byID[child.ID()] = &child
		otherForkByHeight[child.Height] = &child
		otherForkHead = &child
	}
	terminalBlockID := otherForkByHeight[2].ID()

	s.Run("forwards traversal with terminal block (on different fork) included ", func() {
		// assert that we are receiving the correct block at each height
		err := TraverseForward(s.headers, forkHead, noopVisitor, ExcludingBlock(terminalBlockID))
		s.Require().Error(err)
	})

	s.Run("forwards traversal with terminal block (on different fork) excluded ", func() {
		// assert that we are receiving the correct block at each height
		err := TraverseForward(s.headers, forkHead, noopVisitor, IncludingBlock(terminalBlockID))
		s.Require().Error(err)
	})

	s.Run("backwards traversal with terminal block (on different fork) included ", func() {
		// assert that we are receiving the correct block at each height
		err := TraverseBackward(s.headers, forkHead, noopVisitor, ExcludingBlock(terminalBlockID))
		s.Require().Error(err)
	})

	s.Run("backwards traversal with terminal block (on different fork) excluded ", func() {
		// assert that we are receiving the correct block at each height
		err := TraverseBackward(s.headers, forkHead, noopVisitor, IncludingBlock(terminalBlockID))
		s.Require().Error(err)
	})

}
