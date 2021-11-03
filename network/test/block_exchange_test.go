package test

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	datastore "github.com/ipfs/go-datastore/examples"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/utils/unittest"
)

type BlockExchangeTestSuite struct {
	suite.Suite

	cancel         context.CancelFunc
	networks       []*p2p.Network
	blockExchanges []network.BlockExchange
	blockCids      []cid.Cid
	numNetworks    int
}

func (suite *BlockExchangeTestSuite) SetupTest() {
	suite.numNetworks = 3

	logger := zerolog.New(os.Stdout)

	_, _, networks, _, cancel := GenerateIDsMiddlewaresNetworks(
		suite.T(), suite.numNetworks, logger, 100, nil, false, nil, []dht.Option{p2p.AsServer(true)},
	)
	suite.networks = networks
	suite.cancel = cancel

	var blockstores []blockstore.Blockstore

	blockExchangeChannel := network.Channel("block-exchange")

	for i, net := range networks {
		bstore := makeBlockstore(suite.T(), fmt.Sprintf("bs%v", i))
		block := blocks.NewBlock([]byte(fmt.Sprintf("foo%v", i)))
		suite.blockCids = append(suite.blockCids, block.Cid())
		require.NoError(suite.T(), bstore.Put(block))
		blockstores = append(blockstores, bstore)
		bex, err := net.RegisterBlockExchange(blockExchangeChannel, blockstores[i])
		require.NoError(suite.T(), err)
		suite.blockExchanges = append(suite.blockExchanges, bex)
	}
}

func (suite *BlockExchangeTestSuite) TearDownTest() {
	suite.cancel()
	netDoneChans := make([]<-chan struct{}, len(suite.networks))
	for i, net := range suite.networks {
		netDoneChans[i] = net.Done()
	}
	<-util.AllClosed(netDoneChans...)
}

func (suite *BlockExchangeTestSuite) TestGetBlocks() {
	for i, bex := range suite.blockExchanges {
		// check that we can get all other blocks
		var blocksToGet []cid.Cid
		for j, blockCid := range suite.blockCids {
			if j != i {
				blocksToGet = append(blocksToGet, blockCid)
			}
		}

		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(5*time.Second))

		blocksReceived := make(map[cid.Cid]struct{})
		done, err := bex.GetBlocks(blocksToGet...).ForEach(func(b blocks.Block) {
			blocksReceived[b.Cid()] = struct{}{}
		}).Send(ctx)
		require.NoError(suite.T(), err)

		unittest.AssertClosesBefore(suite.T(), done, 5*time.Second)
		cancel()

		for _, blockCid := range blocksToGet {
			_, blockReceived := blocksReceived[blockCid]
			assert.True(suite.T(), blockReceived, "block %v not received by node %v", blockCid, i)
		}
	}
}

func (suite *BlockExchangeTestSuite) TestGetBlocksWithSession() {
	for i, bex := range suite.blockExchanges {
		// check that we can get all other blocks in a single session
		blocksToGet := make(map[cid.Cid]struct{})
		for j, blockCid := range suite.blockCids {
			if j != i {
				blocksToGet[blockCid] = struct{}{}
			}
		}

		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(5*time.Second))

		var doneChans []<-chan struct{}
		session := bex.GetSession(ctx)
		for blockCid := range blocksToGet {
			done, err := session.GetBlocks(blockCid).ForEach(func(b blocks.Block) {
				delete(blocksToGet, blockCid)
			}).Send(ctx)
			require.NoError(suite.T(), err)
			doneChans = append(doneChans, done)
		}

		<-util.AllClosed(doneChans...)
		cancel()

		for blockCid := range blocksToGet {
			assert.Fail(suite.T(), "missing block", "block %v not received by node %v", blockCid, i)
		}
	}
}

func (suite *BlockExchangeTestSuite) TestHas() {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(10*time.Second))
	var doneChans []<-chan struct{}

	blocksReceived := make(map[int]map[cid.Cid]bool)
	for i, bex := range suite.blockExchanges {
		blocksReceived[i] = make(map[cid.Cid]bool)

		// check that peers are updated when we get a new block
		var blocksToGet []cid.Cid
		for j := 0; j < suite.numNetworks; j++ {
			if j != i {
				block := blocks.NewBlock([]byte(fmt.Sprintf("bar%v", i)))
				blocksToGet = append(blocksToGet, block.Cid())
				blocksReceived[i][block.Cid()] = false
			}
		}

		done, err := bex.GetBlocks(blocksToGet...).ForEach(func(b blocks.Block) {
			blocksReceived[i][b.Cid()] = true
		}).Send(ctx)
		require.NoError(suite.T(), err)
		doneChans = append(doneChans, done)
	}

	for i, bex := range suite.blockExchanges {
		err := bex.HasBlock(blocks.NewBlock([]byte(fmt.Sprintf("bar%v", i))))
		require.NoError(suite.T(), err)
	}

	<-util.AllClosed(doneChans...)
	cancel()

	for i, cids := range blocksReceived {
		for c, received := range cids {
			assert.True(suite.T(), received, "block %v not received by node %v", c, i)
		}
	}
}

func makeBlockstore(t *testing.T, name string) blockstore.Blockstore {
	dsDir := filepath.Join(os.TempDir(), name)
	err := os.Mkdir(dsDir, fs.ModeDir)
	require.NoError(t, err)

	ds, err := datastore.NewDatastore(dsDir)
	require.NoError(t, err)

	return blockstore.NewBlockstore(ds.(*datastore.Datastore))
}
