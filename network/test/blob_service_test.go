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
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/topology"
)

// conditionalTopology is a topology that behaves like the underlying topology when the condition is true,
// otherwise returns an empty identity list.
type conditionalTopology struct {
	top       network.Topology
	condition func() bool
}

var _ network.Topology = (*conditionalTopology)(nil)

func (t *conditionalTopology) GenerateFanout(ids flow.IdentityList, channels network.ChannelList) (flow.IdentityList, error) {
	if t.condition() {
		return t.top.GenerateFanout(ids, channels)
	} else {
		return flow.IdentityList{}, nil
	}
}

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

func (suite *BlobServiceTestSuite) putBlob(ds datastore.Batching, blob blobs.Blob) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	suite.Require().NoError(blockstore.NewBlockstore(ds).Put(ctx, blob))
}

func (suite *BlobServiceTestSuite) SetupTest() {
	suite.numNodes = 3

	logger := zerolog.New(os.Stdout)

	// Bitswap listens to connect events but doesn't iterate over existing connections, and fixing this without
	// race conditions is tricky given the way the code is architected. As a result, libP2P hosts must first listen
	// on Bitswap before connecting to each other, otherwise their Bitswap requests may never reach each other.
	// See https://github.com/ipfs/go-bitswap/issues/525 for more details.
	topologyActive := atomic.NewBool(false)
	tops := make([]network.Topology, suite.numNodes)
	for i := 0; i < suite.numNodes; i++ {
		tops[i] = &conditionalTopology{topology.NewFullyConnectedTopology(), topologyActive.Load}
	}

	ids, mws, networks, _, cancel := GenerateIDsMiddlewaresNetworks(
		suite.T(), suite.numNodes, logger, 100, tops, WithDHTOpts(p2p.AsServer(true)), WithPeerManagerOpts(p2p.WithInterval(time.Second)),
	)
	suite.networks = networks
	suite.cancel = cancel

	blobExchangeChannel := network.Channel("blob-exchange")

	for i, net := range networks {
		ds := sync.MutexWrap(datastore.NewMapDatastore())
		suite.datastores = append(suite.datastores, ds)
		blob := blobs.NewBlob([]byte(fmt.Sprintf("foo%v", i)))
		suite.blobCids = append(suite.blobCids, blob.Cid())
		suite.putBlob(ds, blob)
		blobService, err := net.RegisterBlobService(blobExchangeChannel, ds)
		suite.Require().NoError(err)
		<-blobService.Ready()
		suite.blobServices = append(suite.blobServices, blobService)
	}

	// let nodes connect to each other only after they are all listening on Bitswap
	topologyActive.Store(true)
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

func (suite *BlobServiceTestSuite) TestGetBlobsWithSession() {
	for i, bex := range suite.blobServices {
		// check that we can get all other blobs in a single session
		blobsToGet := make(map[cid.Cid]struct{})
		for j, blobCid := range suite.blobCids {
			if j != i {
				blobsToGet[blobCid] = struct{}{}
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		session := bex.GetSession(ctx)
		for blobCid := range blobsToGet {
			_, err := session.GetBlob(ctx, blobCid)
			suite.Assert().NoError(err)
		}
	}
}

func (suite *BlobServiceTestSuite) TestHas() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var blobChans []<-chan blobs.Blob
	unreceivedBlobs := make([]map[cid.Cid]struct{}, len(suite.blobServices))
	for i, bex := range suite.blobServices {
		unreceivedBlobs[i] = make(map[cid.Cid]struct{})
		// check that peers are notified when we have a new blob
		var blobsToGet []cid.Cid
		for j := 0; j < suite.numNodes; j++ {
			if j != i {
				blob := blobs.NewBlob([]byte(fmt.Sprintf("bar%v", i)))
				blobsToGet = append(blobsToGet, blob.Cid())
				unreceivedBlobs[i][blob.Cid()] = struct{}{}
			}
		}
		blobs := bex.GetBlobs(ctx, blobsToGet)
		blobChans = append(blobChans, blobs)
	}

	// check that blobs are not received until Has is called by the server
	suite.Require().Never(func() bool {
		for _, blobChan := range blobChans {
			select {
			case _, ok := <-blobChan:
				if ok {
					return true
				}
			default:
			}
		}
		return false
	}, time.Second, 100*time.Millisecond)

	for i, bex := range suite.blobServices {
		err := bex.AddBlob(ctx, blobs.NewBlob([]byte(fmt.Sprintf("bar%v", i))))
		suite.Require().NoError(err)
	}

	for i, blobs := range blobChans {
		for blob := range blobs {
			delete(unreceivedBlobs[i], blob.Cid())
		}
		for c := range unreceivedBlobs[i] {
			suite.Assert().Fail("blob %v not received by node %v", c, i)
		}
	}
}
