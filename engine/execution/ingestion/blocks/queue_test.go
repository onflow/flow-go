package blocks

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool/entity"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestSingleBlockBecomeReady(t *testing.T) {
	t.Parallel()
	// Given a chain
	// R <- A(C1) <- B(C2,C3) <- C() <- D()
	// -    ^------- E(C4,C5) <- F(C6)
	// -             ^-----------G()
	block, coll, commitFor := makeChainABCDEFG()
	blockA := block("A")
	c1, c2 := coll(1), coll(2)

	q := NewBlockQueue(unittest.Logger())

	// verify receving a collection (C1) before its block (A) will be ignored
	executables, err := q.OnCollection(c1)
	require.NoError(t, err)
	requireExecutableHas(t, executables)

	// verify receving a block (A) will return missing collection (C1)
	missing, executables, err := q.OnBlock(blockA, commitFor("R"))
	require.NoError(t, err)
	require.Empty(t, executables)
	requireCollectionHas(t, missing, c1)

	// verify receving a collection (C2) that is not for the block (A) will be ignored
	executables, err = q.OnCollection(c2)
	require.NoError(t, err)
	requireExecutableHas(t, executables)

	// verify after receiving all collections (C1), block (A) becomes executable
	executables, err = q.OnCollection(c1)
	require.NoError(t, err)
	requireExecutableHas(t, executables, blockA)

	// verify after the block (A) is executed, no more block is executable and
	// nothing left in the queue
	executables, err = q.OnBlockExecuted(blockA.ID(), *commitFor("A"))
	require.NoError(t, err)
	requireExecutableHas(t, executables)
	requireQueueIsEmpty(t, q)
}

func TestMultipleBlockBecomesReady(t *testing.T) {
	t.Parallel()
	// Given a chain
	// R <- A(C1) <- B(C2,C3) <- C() <- D()
	// -    ^------- E(C4,C5) <- F(C6)
	// -             ^-----------G()
	block, coll, commitFor := makeChainABCDEFG()
	blockA, blockB, blockC, blockD, blockE, blockF, blockG :=
		block("A"), block("B"), block("C"), block("D"), block("E"), block("F"), block("G")
	c1, c2, c3, c4, c5, c6 := coll(1), coll(2), coll(3), coll(4), coll(5), coll(6)

	q := NewBlockQueue(unittest.Logger())

	// verify receiving blocks without collections will return missing collections and no executables
	missing, executables, err := q.OnBlock(blockA, commitFor("R"))
	require.NoError(t, err)
	require.Empty(t, executables)
	requireCollectionHas(t, missing, c1)

	missing, executables, err = q.OnBlock(blockB, nil)
	require.NoError(t, err)
	require.Empty(t, executables) // because A is not executed
	requireCollectionHas(t, missing, c2, c3)

	// creating forks
	missing, executables, err = q.OnBlock(blockE, nil)
	require.NoError(t, err)
	require.Empty(t, executables) // because A is not executed
	requireCollectionHas(t, missing, c4, c5)

	// creating forks with empty block
	missing, executables, err = q.OnBlock(blockG, nil)
	require.NoError(t, err)
	require.Empty(t, executables) // because E is not executed
	requireCollectionHas(t, missing)

	missing, executables, err = q.OnBlock(blockF, nil)
	require.NoError(t, err)
	require.Empty(t, executables) // because E is not executed
	requireCollectionHas(t, missing, c6)

	missing, executables, err = q.OnBlock(blockC, nil)
	require.NoError(t, err)
	require.Empty(t, executables) // because B is not executed
	require.Empty(t, missing)

	// verify receiving all collections makes block executable
	executables, err = q.OnCollection(c1)
	require.NoError(t, err)
	requireExecutableHas(t, executables, blockA)

	// verify receiving partial collections won't make block executable
	executables, err = q.OnCollection(c2)
	require.NoError(t, err)
	requireExecutableHas(t, executables) // because A is not executed and C3 is not received for B to be executable

	// verify when parent block (A) is executed, the child block (B) will not become executable if
	// some collection (c3) is still missing
	executables, err = q.OnBlockExecuted(blockA.ID(), *commitFor("A"))
	require.NoError(t, err)
	requireExecutableHas(t, executables) // because C3 is not received for B to be executable

	// verify when parent block (A) has been executed, the child block (B) has all the collections
	// it will become executable
	executables, err = q.OnCollection(c3)
	require.NoError(t, err)
	requireExecutableHas(t, executables, blockB) // c2, c3 are received, blockB is executable

	executables, err = q.OnCollection(c5)
	require.NoError(t, err)
	requireExecutableHas(t, executables) // c2, c3 are received, blockB is executable

	executables, err = q.OnCollection(c6)
	require.NoError(t, err)
	requireExecutableHas(t, executables) // c2, c3 are received, blockB is executable

	executables, err = q.OnCollection(c4)
	require.NoError(t, err)
	requireExecutableHas(t, executables, blockE) // c2, c3 are received, blockB is executable

	// verify when parent block (E) is executed, all children block (F,G) will become executable if all
	// collections (C6) have already received
	executables, err = q.OnBlockExecuted(blockE.ID(), *commitFor("E"))
	require.NoError(t, err)
	requireExecutableHas(t, executables, blockF, blockG)

	executables, err = q.OnBlockExecuted(blockB.ID(), *commitFor("B"))
	require.NoError(t, err)
	requireExecutableHas(t, executables, blockC)

	executables, err = q.OnBlockExecuted(blockC.ID(), *commitFor("C"))
	require.NoError(t, err)
	requireExecutableHas(t, executables)

	// verify receiving a block whose parent was executed before
	missing, executables, err = q.OnBlock(blockD, commitFor("C"))
	require.NoError(t, err)
	require.Empty(t, missing)
	requireExecutableHas(t, executables, blockD)

	executables, err = q.OnBlockExecuted(blockD.ID(), *commitFor("D"))
	require.NoError(t, err)
	requireExecutableHas(t, executables)

	executables, err = q.OnBlockExecuted(blockF.ID(), *commitFor("F"))
	require.NoError(t, err)
	requireExecutableHas(t, executables)

	executables, err = q.OnBlockExecuted(blockG.ID(), *commitFor("G"))
	require.NoError(t, err)
	requireExecutableHas(t, executables)

	// verify after all blocks are executed, the queue is empty
	requireQueueIsEmpty(t, q)
}

func TestOnForksWithSameCollections(t *testing.T) {
	t.Parallel()
	// Given a chain
	// R() <- A() <- B(C1, C2) <- C(C3)
	// -      ^----- D(C1, C2) <- E(C3)
	// -      ^----- F(C1, C2, C3)
	block, coll, commitFor := makeChainABCDEF()
	blockA, blockB, blockC, blockD, blockE, blockF :=
		block("A"), block("B"), block("C"), block("D"), block("E"), block("F")
	c1, c2, c3 := coll(1), coll(2), coll(3)

	q := NewBlockQueue(unittest.Logger())

	missing, executables, err := q.OnBlock(blockA, commitFor("R"))
	require.NoError(t, err)
	requireExecutableHas(t, executables, blockA)
	requireCollectionHas(t, missing)

	// receiving block B and D which have the same collections (C1, C2)
	missing, executables, err = q.OnBlock(blockB, nil)
	require.NoError(t, err)
	require.Empty(t, executables)
	requireCollectionHas(t, missing, c1, c2)

	// receiving block F (C1, C2, C3)
	missing, executables, err = q.OnBlock(blockF, nil)
	require.NoError(t, err)
	require.Empty(t, executables)
	requireCollectionHas(t, missing, c3) // c1 and c2 are requested before, only c3 is missing

	// verify receiving D will not return any missing collections because
	// missing collections were returned when receiving B
	missing, executables, err = q.OnBlock(blockD, nil)
	require.NoError(t, err)
	require.Empty(t, executables)
	requireCollectionHas(t, missing)

	// verify receiving all collections makes all blocks executable
	executables, err = q.OnCollection(c1)
	require.NoError(t, err)
	requireExecutableHas(t, executables)

	// A is executed
	executables, err = q.OnBlockExecuted(blockA.ID(), *commitFor("A"))
	require.NoError(t, err)
	requireExecutableHas(t, executables) // because C2 is not received

	executables, err = q.OnCollection(c2)
	require.NoError(t, err)
	requireExecutableHas(t, executables, blockB, blockD)

	// verify if 2 blocks (C, E) having the same collections (C3), if all collections are received,
	// but only one block (C) whose parent (B) is executed, then only that block (C) becomes executable
	// the other block (E) is not executable

	missing, executables, err = q.OnBlock(blockC, nil)
	require.NoError(t, err)
	require.Empty(t, executables)
	requireCollectionHas(t, missing) // because C3 is requested when F is received

	missing, executables, err = q.OnBlock(blockE, nil)
	require.NoError(t, err)
	require.Empty(t, executables)
	requireCollectionHas(t, missing)

	executables, err = q.OnBlockExecuted(blockB.ID(), *commitFor("B"))
	require.NoError(t, err)
	requireExecutableHas(t, executables)

	// verify C and F are executable, because their parent have been executed
	// E is not executable, because E's parent (D) is not executed yet.
	executables, err = q.OnCollection(c3)
	require.NoError(t, err)
	requireExecutableHas(t, executables, blockC, blockF)

	// verify when D is executed, E becomes executable
	executables, err = q.OnBlockExecuted(blockD.ID(), *commitFor("D"))
	require.NoError(t, err)
	requireExecutableHas(t, executables, blockE)

	// verify the remaining blocks (C,E,F) are executed, the queue is empty
	executables, err = q.OnBlockExecuted(blockE.ID(), *commitFor("E"))
	require.NoError(t, err)
	requireExecutableHas(t, executables)

	executables, err = q.OnBlockExecuted(blockF.ID(), *commitFor("F"))
	require.NoError(t, err)
	requireExecutableHas(t, executables)

	executables, err = q.OnBlockExecuted(blockC.ID(), *commitFor("C"))
	require.NoError(t, err)
	requireExecutableHas(t, executables)

	requireQueueIsEmpty(t, q)
}

func TestOnBlockWithMissingParentCommit(t *testing.T) {
	t.Parallel()
	// Given a chain
	// R <- A(C1) <- B(C2,C3) <- C() <- D()
	// -    ^------- E(C4,C5) <- F(C6)
	// -             ^-----------G()

	block, coll, commitFor := makeChainABCDEFG()
	blockA, blockB := block("A"), block("B")
	c1, c2, c3 := coll(1), coll(2), coll(3)

	q := NewBlockQueue(unittest.Logger())

	missing, executables, err := q.OnBlock(blockA, commitFor("R"))
	require.NoError(t, err)
	require.Empty(t, executables)
	requireCollectionHas(t, missing, c1)

	// block A has all the collections and become executable
	executables, err = q.OnCollection(c1)
	require.NoError(t, err)
	requireExecutableHas(t, executables, blockA)

	// the following two calls create an edge case where A is executed,
	// and B is received, however, due to race condition, the parent commit
	// was not saved in the database yet
	executables, err = q.OnBlockExecuted(blockA.ID(), *commitFor("A"))
	require.NoError(t, err)
	requireExecutableHas(t, executables)
	requireQueueIsEmpty(t, q)

	// verify when race condition happens, ErrMissingParent will be returned
	_, _, err = q.OnBlock(blockB, nil)
	require.True(t, errors.Is(err, ErrMissingParent), err)

	// verify if called again with parent commit, it will be successful
	missing, executables, err = q.OnBlock(blockB, commitFor("A"))
	require.NoError(t, err)
	require.Empty(t, executables)
	requireCollectionHas(t, missing, c2, c3)

	// verify after receiving all collections, B becomes executable
	executables, err = q.OnCollection(c2)
	require.NoError(t, err)
	require.Empty(t, executables)

	executables, err = q.OnCollection(c3)
	require.NoError(t, err)
	requireExecutableHas(t, executables, blockB)

	// verify after B is executed, the queue is empty
	executables, err = q.OnBlockExecuted(blockB.ID(), *commitFor("B"))
	require.NoError(t, err)
	requireExecutableHas(t, executables)
	requireQueueIsEmpty(t, q)
}

/* ==== Test utils ==== */

// GetBlock("A") => A
type GetBlock func(name string) *flow.Block

// GetCollection(1) => C1
type GetCollection func(name int) *flow.Collection

// GetCommit("A") => A_FinalState
type GetCommit func(name string) *flow.StateCommitment

// R <- A(C1) <- B(C2,C3) <- C() <- D()
// -    ^------- E(C4,C5) <- F(C6)
// -             ^-----------G()
func makeChainABCDEFG() (GetBlock, GetCollection, GetCommit) {
	cs := unittest.CollectionListFixture(6)
	c1, c2, c3, c4, c5, c6 :=
		cs[0], cs[1], cs[2], cs[3], cs[4], cs[5]
	getCol := func(name int) *flow.Collection {
		if name < 1 || name > len(cs) {
			return nil
		}
		return cs[name-1]
	}

	r := unittest.BlockFixture()
	blockR := &r
	bs := unittest.ChainBlockFixtureWithRoot(blockR.Header, 4)
	blockA, blockB, blockC, blockD := bs[0], bs[1], bs[2], bs[3]
	unittest.AddCollectionsToBlock(blockA, []*flow.Collection{c1})
	unittest.AddCollectionsToBlock(blockB, []*flow.Collection{c2, c3})
	unittest.RechainBlocks(bs)

	bs = unittest.ChainBlockFixtureWithRoot(blockA.Header, 2)
	blockE, blockF := bs[0], bs[1]
	unittest.AddCollectionsToBlock(blockE, []*flow.Collection{c4, c5})
	unittest.AddCollectionsToBlock(blockF, []*flow.Collection{c6})
	unittest.RechainBlocks(bs)

	bs = unittest.ChainBlockFixtureWithRoot(blockE.Header, 1)
	blockG := bs[0]

	blockLookup := map[string]*flow.Block{
		"R": blockR,
		"A": blockA,
		"B": blockB,
		"C": blockC,
		"D": blockD,
		"E": blockE,
		"F": blockF,
		"G": blockG,
	}

	getBlock := func(name string) *flow.Block {
		return blockLookup[name]
	}

	commitLookup := make(map[string]*flow.StateCommitment, len(blockLookup))
	for name := range blockLookup {
		commit := unittest.StateCommitmentFixture()
		commitLookup[name] = &commit
	}

	getCommit := func(name string) *flow.StateCommitment {
		commit, ok := commitLookup[name]
		if !ok {
			panic("commit not found")
		}
		return commit
	}

	return getBlock, getCol, getCommit
}

// R() <- A() <- B(C1, C2) <- C(C3)
// -      ^----- D(C1, C2) <- E(C3)
// -      ^----- F(C1, C2, C3)
func makeChainABCDEF() (GetBlock, GetCollection, GetCommit) {
	cs := unittest.CollectionListFixture(3)
	c1, c2, c3 := cs[0], cs[1], cs[2]
	getCol := func(name int) *flow.Collection {
		if name < 1 || name > len(cs) {
			return nil
		}
		return cs[name-1]
	}

	r := unittest.BlockFixture()
	blockR := &r
	bs := unittest.ChainBlockFixtureWithRoot(blockR.Header, 3)
	blockA, blockB, blockC := bs[0], bs[1], bs[2]
	unittest.AddCollectionsToBlock(blockB, []*flow.Collection{c1, c2})
	unittest.AddCollectionsToBlock(blockC, []*flow.Collection{c3})
	unittest.RechainBlocks(bs)

	bs = unittest.ChainBlockFixtureWithRoot(blockA.Header, 2)
	blockD, blockE := bs[0], bs[1]
	unittest.AddCollectionsToBlock(blockD, []*flow.Collection{c1, c2})
	unittest.AddCollectionsToBlock(blockE, []*flow.Collection{c3})
	unittest.RechainBlocks(bs)

	bs = unittest.ChainBlockFixtureWithRoot(blockA.Header, 1)
	blockF := bs[0]
	unittest.AddCollectionsToBlock(blockF, []*flow.Collection{c1, c2, c3})
	unittest.RechainBlocks(bs)

	blockLookup := map[string]*flow.Block{
		"R": blockR,
		"A": blockA,
		"B": blockB,
		"C": blockC,
		"D": blockD,
		"E": blockE,
		"F": blockF,
	}

	getBlock := func(name string) *flow.Block {
		return blockLookup[name]
	}

	commitLookup := make(map[string]*flow.StateCommitment, len(blockLookup))
	for name := range blockLookup {
		commit := unittest.StateCommitmentFixture()
		commitLookup[name] = &commit
	}

	getCommit := func(name string) *flow.StateCommitment {
		commit, ok := commitLookup[name]
		if !ok {
			panic("commit not found for " + name)
		}
		return commit
	}

	return getBlock, getCol, getCommit
}

func requireExecutableHas(t *testing.T, executables []*entity.ExecutableBlock, bs ...*flow.Block) {
	blocks := make(map[flow.Identifier]*flow.Block, len(bs))
	for _, b := range bs {
		blocks[b.ID()] = b
	}

	for _, e := range executables {
		_, ok := blocks[e.Block.ID()]
		require.True(t, ok)
		delete(blocks, e.Block.ID())
	}

	require.Equal(t, len(bs), len(executables))
	require.Equal(t, 0, len(blocks))
}

func requireCollectionHas(t *testing.T, missing []*MissingCollection, cs ...*flow.Collection) {
	collections := make(map[flow.Identifier]*flow.Collection, len(cs))
	for _, c := range cs {
		collections[c.ID()] = c
	}

	for _, m := range missing {
		_, ok := collections[m.Guarantee.CollectionID]
		require.True(t, ok)
		delete(collections, m.Guarantee.CollectionID)
	}

	require.Equal(t, len(cs), len(missing))
	require.Equal(t, 0, len(collections))
}

func requireQueueIsEmpty(t *testing.T, q *BlockQueue) {
	require.Equal(t, 0, len(q.blocks))
	require.Equal(t, 0, len(q.collections))
	require.Equal(t, 0, len(q.blockIDsByHeight))
}
