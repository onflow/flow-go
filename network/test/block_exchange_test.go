package test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/sync"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/p2p"
)

type BlockExchangeTestSuite struct {
	suite.Suite

	cancel         context.CancelFunc
	networks       []*p2p.Network
	blockExchanges []network.BlockExchange
	blockCids      []cid.Cid
	numNetworks    int
}

func TestBlockExchange(t *testing.T) {
	suite.Run(t, new(BlockExchangeTestSuite))
}

func (suite *BlockExchangeTestSuite) SetupTest() {
	suite.numNetworks = 3

	logger := zerolog.New(os.Stdout)

	_, _, networks, _, cancel := GenerateIDsMiddlewaresNetworks(
		suite.T(), suite.numNetworks, logger, 100, nil, false, nil, []dht.Option{p2p.AsServer(true)},
	)
	suite.networks = networks
	suite.cancel = cancel

	blockExchangeChannel := network.Channel("block-exchange")

	for i, net := range networks {
		bstore := blockstore.NewBlockstore(sync.MutexWrap(datastore.NewMapDatastore()))
		block := blocks.NewBlock([]byte(fmt.Sprintf("foo%v", i)))
		suite.blockCids = append(suite.blockCids, block.Cid())
		require.NoError(suite.T(), bstore.Put(block))
		bex, err := net.RegisterBlockExchange(blockExchangeChannel, bstore)
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
		unreceivedBlocks := make(map[cid.Cid]struct{})
		for j, blockCid := range suite.blockCids {
			if j != i {
				blocksToGet = append(blocksToGet, blockCid)
				unreceivedBlocks[blockCid] = struct{}{}
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		blocks, err := bex.GetBlocks(ctx, blocksToGet...)
		suite.Require().NoError(err)

		for block := range blocks {
			delete(unreceivedBlocks, block.Cid())
		}

		for c := range unreceivedBlocks {
			suite.T().Errorf("Block %v not received by node %v", c, i)
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
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var blockChans []<-chan blocks.Block
		session := bex.GetSession(ctx)
		for blockCid := range blocksToGet {
			blocks, err := session.GetBlocks(ctx, blockCid)
			suite.Require().NoError(err)
			blockChans = append(blockChans, blocks)
		}
		for block := range util.MergeChannels(blockChans).(<-chan blocks.Block) {
			delete(blocksToGet, block.Cid())
		}
		for blockCid := range blocksToGet {
			suite.T().Errorf("block %v not received by node %v", blockCid, i)
		}
	}
}

func (suite *BlockExchangeTestSuite) TestHas() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var blockChans []<-chan blocks.Block
	unreceivedBlocks := make([]map[cid.Cid]struct{}, len(suite.blockExchanges))
	for i, bex := range suite.blockExchanges {
		unreceivedBlocks[i] = make(map[cid.Cid]struct{})
		// check that peers are updated when we get a new block
		var blocksToGet []cid.Cid
		for j := 0; j < suite.numNetworks; j++ {
			if j != i {
				block := blocks.NewBlock([]byte(fmt.Sprintf("bar%v", i)))
				blocksToGet = append(blocksToGet, block.Cid())
				unreceivedBlocks[i][block.Cid()] = struct{}{}
			}
		}
		blocks, err := bex.GetBlocks(ctx, blocksToGet...)
		suite.Require().NoError(err)
		blockChans = append(blockChans, blocks)
	}
	for i, bex := range suite.blockExchanges {
		err := bex.HasBlock(blocks.NewBlock([]byte(fmt.Sprintf("bar%v", i))))
		suite.Require().NoError(err)
	}
	for i, blocks := range blockChans {
		for block := range blocks {
			delete(unreceivedBlocks[i], block.Cid())
		}
		for c := range unreceivedBlocks[i] {
			suite.Assert().Fail("block %v not received by node %v", c, i)
		}
	}
}
