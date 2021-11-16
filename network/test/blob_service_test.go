package test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/sync"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/topology"
)

type BlobServiceTestSuite struct {
	suite.Suite

	cancel       context.CancelFunc
	networks     []network.Network
	blobServices []network.BlobService
	datastores   []datastore.Batching
	blobCids     []cid.Cid
	numNodes     int
}

func TestBlobExchange(t *testing.T) {
	suite.Run(t, new(BlobServiceTestSuite))
}

func (suite *BlobServiceTestSuite) putBlob(ds datastore.Batching, blob network.Blob) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	suite.Require().NoError(blockstore.NewBlockstore(ds).Put(ctx, blob))
}

func (suite *BlobServiceTestSuite) SetupTest() {
	suite.numNodes = 3

	logger := zerolog.New(os.Stdout)

	tops := make([]network.Topology, suite.numNodes)
	for i := 0; i < suite.numNodes; i++ {
		tops[i] = topology.NewFullyConnectedTopology()
	}
	ids, mws, networks, _, cancel := GenerateIDsMiddlewaresNetworks(
		suite.T(), suite.numNodes, logger, 100, tops, false, nil, []dht.Option{p2p.AsServer(true)},
	)
	suite.networks = networks
	suite.cancel = cancel

	suite.Require().Eventually(func() bool {
		for i, mw := range mws {
			for j := i + 1; j < suite.numNodes; j++ {
				connected, err := mw.IsConnected(ids[j].NodeID)
				suite.Require().NoError(err)
				if !connected {
					return false
				}
			}
		}
		return true
	}, 3*time.Second, 100*time.Millisecond)

	blobExchangeChannel := network.Channel("blob-exchange")

	for i, net := range networks {
		ds := sync.MutexWrap(datastore.NewMapDatastore())
		suite.datastores = append(suite.datastores, ds)
		blob := network.NewBlob([]byte(fmt.Sprintf("foo%v", i)))
		suite.blobCids = append(suite.blobCids, blob.Cid())
		suite.putBlob(ds, blob)
		blobService, err := net.RegisterBlobService(blobExchangeChannel, ds)
		suite.Require().NoError(err)
		suite.blobServices = append(suite.blobServices, blobService)
	}
}

func (suite *BlobServiceTestSuite) TearDownTest() {
	suite.cancel()
	netDoneChans := make([]<-chan struct{}, len(suite.networks))
	for i, net := range suite.networks {
		netDoneChans[i] = net.Done()
	}
	<-util.AllClosed(netDoneChans...)
}

func (suite *BlobServiceTestSuite) TestGetBlobs() {
	for i, bex := range suite.blobServices {
		// check that we can get all other blobs
		var blobsToGet []cid.Cid
		unreceivedBlobs := make(map[cid.Cid]struct{})
		for j, blobCid := range suite.blobCids {
			if j != i {
				blobsToGet = append(blobsToGet, blobCid)
				unreceivedBlobs[blobCid] = struct{}{}
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		blobs := bex.GetBlobs(ctx, blobsToGet)

		for blob := range blobs {
			delete(unreceivedBlobs, blob.Cid())
		}

		for c := range unreceivedBlobs {
			suite.T().Errorf("Blob %v not received by node %v", c, i)
		}
	}
}

// func (suite *BlobServiceTestSuite) TestGetBlobsWithSession() {
// 	for i, bex := range suite.blobExchanges {
// 		// check that we can get all other blobs in a single session
// 		blobsToGet := make(map[cid.Cid]struct{})
// 		for j, blobCid := range suite.blobCids {
// 			if j != i {
// 				blobsToGet[blobCid] = struct{}{}
// 			}
// 		}
// 		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
// 		defer cancel()
// 		var blobChans []<-chan blobs.Blob
// 		session := bex.GetSession(ctx)
// 		for blobCid := range blobsToGet {
// 			blobs, err := session.GetBlobs(ctx, blobCid)
// 			suite.Require().NoError(err)
// 			blobChans = append(blobChans, blobs)
// 		}
// 		for blob := range util.MergeChannels(blobChans).(<-chan blobs.Blob) {
// 			delete(blobsToGet, blob.Cid())
// 		}
// 		for blobCid := range blobsToGet {
// 			suite.T().Errorf("blob %v not received by node %v", blobCid, i)
// 		}
// 	}
// }

// func (suite *BlobServiceTestSuite) TestHas() {
// 	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
// 	defer cancel()

// 	var blobChans []<-chan blobs.Blob
// 	unreceivedBlobs := make([]map[cid.Cid]struct{}, len(suite.blobExchanges))
// 	for i, bex := range suite.blobExchanges {
// 		unreceivedBlobs[i] = make(map[cid.Cid]struct{})
// 		// check that peers are notified when we have a new blob
// 		var blobsToGet []cid.Cid
// 		for j := 0; j < suite.numNodes; j++ {
// 			if j != i {
// 				blob := blobs.NewBlob([]byte(fmt.Sprintf("bar%v", i)))
// 				blobsToGet = append(blobsToGet, blob.Cid())
// 				unreceivedBlobs[i][blob.Cid()] = struct{}{}
// 			}
// 		}
// 		blobs, err := bex.GetBlobs(ctx, blobsToGet...)
// 		suite.Require().NoError(err)
// 		blobChans = append(blobChans, blobs)
// 	}

// 	// check that blobs are not received until Has is called by the server
// 	suite.Require().Never(func() bool {
// 		for _, blobChan := range blobChans {
// 			select {
// 			case <-blobChan:
// 				return true
// 			default:
// 			}
// 		}
// 		return false
// 	}, time.Second, 100*time.Millisecond)

// 	for i, bex := range suite.blobExchanges {
// 		err := bex.HasBlob(blobs.NewBlob([]byte(fmt.Sprintf("bar%v", i))))
// 		suite.Require().NoError(err)
// 	}

// 	for i, blobs := range blobChans {
// 		for blob := range blobs {
// 			delete(unreceivedBlobs[i], blob.Cid())
// 		}
// 		for c := range unreceivedBlobs[i] {
// 			suite.Assert().Fail("blob %v not received by node %v", c, i)
// 		}
// 	}
// }
