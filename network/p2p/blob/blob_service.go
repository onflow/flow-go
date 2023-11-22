package blob

import (
	"context"
	"errors"
	"time"

	"github.com/hashicorp/go-multierror"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	provider "github.com/ipfs/go-ipfs-provider"
	"github.com/ipfs/go-ipfs-provider/simple"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/core/routing"
	"github.com/onflow/go-bitswap"
	bsmsg "github.com/onflow/go-bitswap/message"
	bsnet "github.com/onflow/go-bitswap/network"
	"github.com/rs/zerolog"
	"golang.org/x/time/rate"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/p2p/p2plogging"
	"github.com/onflow/flow-go/utils/logging"

	ipld "github.com/ipfs/go-ipld-format"
)

type blobService struct {
	prefix string
	component.Component
	blockService blockservice.BlockService
	blockStore   blockstore.Blockstore
	reprovider   provider.Reprovider
	config       *BlobServiceConfig
}

var _ network.BlobService = (*blobService)(nil)
var _ component.Component = (*blobService)(nil)

type BlobServiceConfig struct {
	ReprovideInterval time.Duration    // the interval at which the DHT provider entries are refreshed
	BitswapOptions    []bitswap.Option // options to pass to the Bitswap service
}

// WithReprovideInterval sets the interval at which DHT provider entries are refreshed
func WithReprovideInterval(d time.Duration) network.BlobServiceOption {
	return func(bs network.BlobService) {
		bs.(*blobService).config.ReprovideInterval = d
	}
}

// WithBitswapOptions sets additional options for Bitswap exchange
func WithBitswapOptions(opts ...bitswap.Option) network.BlobServiceOption {
	return func(bs network.BlobService) {
		bs.(*blobService).config.BitswapOptions = opts
	}
}

// WithHashOnRead sets whether or not the blobstore will rehash the blob data on read
// When set, calls to GetBlob will fail with an error if the hash of the data in storage does not
// match its CID
func WithHashOnRead(enabled bool) network.BlobServiceOption {
	return func(bs network.BlobService) {
		bs.(*blobService).blockStore.HashOnRead(enabled)
	}
}

// WithRateLimit sets a rate limit on reads from the underlying datastore that allows up
// to r bytes per second and permits bursts of at most b bytes. Note that b should be
// set to at least the max blob size, otherwise blobs larger than b cannot be read from
// the blobstore.
func WithRateLimit(r float64, b int) network.BlobServiceOption {
	return func(bs network.BlobService) {
		blobService := bs.(*blobService)
		blobService.blockStore = newRateLimitedBlockStore(blobService.blockStore, blobService.prefix, r, b)
	}
}

// NewBlobService creates a new BlobService.
func NewBlobService(
	host host.Host,
	r routing.ContentRouting,
	prefix string,
	ds datastore.Batching,
	metrics module.BitswapMetrics,
	logger zerolog.Logger,
	opts ...network.BlobServiceOption,
) *blobService {
	bsNetwork := bsnet.NewFromIpfsHost(host, r, bsnet.Prefix(protocol.ID(prefix)))
	bs := &blobService{
		prefix: prefix,
		config: &BlobServiceConfig{
			ReprovideInterval: 12 * time.Hour,
		},
		blockStore: blockstore.NewBlockstore(ds),
	}

	for _, opt := range opts {
		opt(bs)
	}

	cm := component.NewComponentManagerBuilder().
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			btswp := bitswap.New(ctx, bsNetwork, bs.blockStore, bs.config.BitswapOptions...)
			bs.blockService = blockservice.New(bs.blockStore, btswp)

			ready()

			ticker := time.NewTicker(15 * time.Second)
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					stat, err := btswp.Stat()
					if err != nil {
						logger.Err(err).Str("component", "blob_service").Str("prefix", prefix).Msg("failed to get bitswap stats")
						continue
					}

					metrics.Peers(prefix, len(stat.Peers))
					metrics.Wantlist(prefix, len(stat.Wantlist))
					metrics.BlobsReceived(prefix, stat.BlocksReceived)
					metrics.DataReceived(prefix, stat.DataReceived)
					metrics.BlobsSent(prefix, stat.BlocksSent)
					metrics.DataSent(prefix, stat.DataSent)
					metrics.DupBlobsReceived(prefix, stat.DupBlksReceived)
					metrics.DupDataReceived(prefix, stat.DupDataReceived)
					metrics.MessagesReceived(prefix, stat.MessagesReceived)
				}
			}
		}).
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			bs.reprovider = simple.NewReprovider(ctx, bs.config.ReprovideInterval, r, simple.NewBlockstoreProvider(bs.blockStore))

			ready()

			bs.reprovider.Run()
		}).
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			ready()

			<-bs.Ready() // wait for variables to be initialized
			<-ctx.Done()

			var err *multierror.Error

			err = multierror.Append(err, bs.reprovider.Close())
			err = multierror.Append(err, bs.blockService.Close())

			if err.ErrorOrNil() != nil {
				ctx.Throw(err)
			}
		}).
		Build()

	bs.Component = cm

	return bs
}

func (bs *blobService) TriggerReprovide(ctx context.Context) error {
	return bs.reprovider.Trigger(ctx)
}

func (bs *blobService) GetBlob(ctx context.Context, c cid.Cid) (blobs.Blob, error) {
	blob, err := bs.blockService.GetBlock(ctx, c)
	if ipld.IsNotFound(err) {
		return nil, network.ErrBlobNotFound
	}

	return blob, err
}

func (bs *blobService) GetBlobs(ctx context.Context, ks []cid.Cid) <-chan blobs.Blob {
	return bs.blockService.GetBlocks(ctx, ks)
}

func (bs *blobService) AddBlob(ctx context.Context, b blobs.Blob) error {
	return bs.blockService.AddBlock(ctx, b)
}

func (bs *blobService) AddBlobs(ctx context.Context, blobs []blobs.Blob) error {
	return bs.blockService.AddBlocks(ctx, blobs)
}

func (bs *blobService) DeleteBlob(ctx context.Context, c cid.Cid) error {
	return bs.blockService.DeleteBlock(ctx, c)
}

func (bs *blobService) GetSession(ctx context.Context) network.BlobGetter {
	return &blobServiceSession{blockservice.NewSession(ctx, bs.blockService)}
}

type blobServiceSession struct {
	session *blockservice.Session
}

var _ network.BlobGetter = (*blobServiceSession)(nil)

func (s *blobServiceSession) GetBlob(ctx context.Context, c cid.Cid) (blobs.Blob, error) {
	return s.session.GetBlock(ctx, c)
}

func (s *blobServiceSession) GetBlobs(ctx context.Context, ks []cid.Cid) <-chan blobs.Blob {
	return s.session.GetBlocks(ctx, ks)
}

type rateLimitedBlockStore struct {
	blockstore.Blockstore
	limiter *rate.Limiter
	metrics module.RateLimitedBlockstoreMetrics
}

var rateLimitedError = errors.New("rate limited")

func newRateLimitedBlockStore(bs blockstore.Blockstore, prefix string, r float64, b int) *rateLimitedBlockStore {
	return &rateLimitedBlockStore{
		Blockstore: bs,
		limiter:    rate.NewLimiter(rate.Limit(r), b),
		metrics:    metrics.NewRateLimitedBlockstoreCollector(prefix),
	}
}

func (r *rateLimitedBlockStore) Get(ctx context.Context, c cid.Cid) (blocks.Block, error) {
	size, err := r.Blockstore.GetSize(ctx, c)
	if err != nil {
		return nil, err
	}

	allowed := r.limiter.AllowN(time.Now(), size)
	if !allowed {
		return nil, rateLimitedError
	}

	r.metrics.BytesRead(size)

	return r.Blockstore.Get(ctx, c)
}

// AuthorizedRequester returns a callback function used by bitswap to authorize block requests
// A request is authorized if the peer is
// * known by the identity provider
// * not ejected
// * an Access node
// * in the allowedNodes list (if non-empty)
func AuthorizedRequester(
	allowedNodes map[flow.Identifier]bool,
	identityProvider module.IdentityProvider,
	logger zerolog.Logger,
) func(peer.ID, cid.Cid) bool {
	return func(peerID peer.ID, _ cid.Cid) bool {
		lg := logger.With().
			Str("component", "blob_service").
			Str("peer_id", p2plogging.PeerId(peerID)).
			Logger()

		id, ok := identityProvider.ByPeerID(peerID)

		if !ok {
			lg.Warn().
				Bool(logging.KeySuspicious, true).
				Msg("rejecting request from unknown peer")
			return false
		}

		lg = lg.With().
			Str("peer_node_id", id.NodeID.String()).
			Str("role", id.Role.String()).
			Logger()

		// TODO: when execution data verification is enabled, add verification nodes here
		if (id.Role != flow.RoleExecution && id.Role != flow.RoleAccess) || id.Ejected {
			lg.Warn().
				Bool(logging.KeySuspicious, true).
				Msg("rejecting request from peer: unauthorized")
			return false
		}

		// allow list is only for Access nodes
		if id.Role == flow.RoleAccess && len(allowedNodes) > 0 && !allowedNodes[id.NodeID] {
			// honest peers not on the allowed list have no way to know and will continue to request
			// blobs. therefore, these requests do not indicate suspicious behavior
			lg.Debug().Msg("rejecting request from peer: not in allowed list")
			return false
		}

		lg.Debug().Msg("accepting request from peer")
		return true
	}
}

type Tracer struct {
	logger zerolog.Logger
}

func NewTracer(logger zerolog.Logger) *Tracer {
	return &Tracer{
		logger,
	}
}

func (t *Tracer) logMsg(msg bsmsg.BitSwapMessage, s string) {
	evt := t.logger.Debug()

	wantlist := zerolog.Arr()
	for _, entry := range msg.Wantlist() {
		wantlist = wantlist.Interface(entry)
	}
	evt.Array("wantlist", wantlist)

	blks := zerolog.Arr()
	for _, blk := range msg.Blocks() {
		blks = blks.Str(blk.Cid().String())
	}
	evt.Array("blocks", blks)

	haves := zerolog.Arr()
	for _, have := range msg.Haves() {
		haves = haves.Str(have.String())
	}
	evt.Array("haves", haves)

	dontHaves := zerolog.Arr()
	for _, dontHave := range msg.DontHaves() {
		dontHaves = dontHaves.Str(dontHave.String())
	}
	evt.Array("dontHaves", dontHaves)

	evt.Int32("pendingBytes", msg.PendingBytes())

	evt.Msg(s)
}

func (t *Tracer) MessageReceived(pid peer.ID, msg bsmsg.BitSwapMessage) {
	if t.logger.Debug().Enabled() {
		t.logMsg(msg, "bitswap message received")
	}
}

func (t *Tracer) MessageSent(pid peer.ID, msg bsmsg.BitSwapMessage) {
	if t.logger.Debug().Enabled() {
		t.logMsg(msg, "bitswap message sent")
	}
}
