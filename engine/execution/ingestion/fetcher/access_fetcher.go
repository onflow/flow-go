package fetcher

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"github.com/sethvargo/go-retry"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/onflow/flow/protobuf/go/flow/access"

	"github.com/onflow/flow-go/engine/common/requester"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/engine/execution/ingestion"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/utils/grpcutils"
)

type AccessCollectionFetcher struct {
	*component.ComponentManager
	log zerolog.Logger

	handler  requester.HandleFunc
	client   access.AccessAPIClient
	chain    flow.Chain
	originID flow.Identifier

	guaranteeInfos chan guaranteeInfo
}

type guaranteeInfo struct {
	blockID flow.Identifier
	height  uint64
	colID   flow.Identifier
}

var _ ingestion.CollectionFetcher = (*AccessCollectionFetcher)(nil)
var _ ingestion.CollectionRequester = (*AccessCollectionFetcher)(nil)

func NewAccessCollectionFetcher(
	logger zerolog.Logger, accessURL string, nodeID flow.Identifier, chain flow.Chain) (
	*AccessCollectionFetcher, error) {
	collectionRPCConn, err := grpc.Dial(
		accessURL,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(grpcutils.DefaultMaxMsgSize)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to collection rpc: %w", err)
	}

	noopHandler := func(flow.Identifier, flow.Entity) {}
	e := &AccessCollectionFetcher{
		log:            logger.With().Str("engine", "collection_fetcher").Logger(),
		handler:        noopHandler,
		client:         access.NewAccessAPIClient(collectionRPCConn),
		chain:          chain,
		originID:       nodeID,
		guaranteeInfos: make(chan guaranteeInfo, 100),
	}

	builder := component.NewComponentManagerBuilder().AddWorker(e.launchWorker)

	e.ComponentManager = builder.Build()

	return e, nil
}

func (f *AccessCollectionFetcher) FetchCollection(blockID flow.Identifier, height uint64, guarantee *flow.CollectionGuarantee) error {
	f.guaranteeInfos <- guaranteeInfo{
		blockID: blockID,
		height:  height,
		colID:   guarantee.CollectionID,
	}

	return nil
}

func (f *AccessCollectionFetcher) Force() {
}

func (f *AccessCollectionFetcher) WithHandle(handler requester.HandleFunc) {
	f.handler = handler
}

func (f *AccessCollectionFetcher) launchWorker(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	for {
		select {
		case <-ctx.Done():
			return
		case guaranteeInfo := <-f.guaranteeInfos:
			err := f.fetchCollection(ctx, guaranteeInfo)
			if err != nil {
				ctx.Throw(fmt.Errorf("failed to fetch collection: %w", err))
			}
		}
	}
}

func (f *AccessCollectionFetcher) fetchCollection(ctx irrecoverable.SignalerContext, g guaranteeInfo) error {
	backoff := retry.NewConstant(3 * time.Second)
	return retry.Do(ctx, backoff, func(ctx context.Context) error {
		f.log.Debug().Hex("blockID", g.blockID[:]).Uint64("height", g.height).Hex("col_id", g.colID[:]).Msgf("fetching collection")
		resp, err := f.client.GetFullCollectionByID(context.Background(),
			&access.GetFullCollectionByIDRequest{
				Id: g.colID[:],
			})
		if err != nil {
			f.log.Error().Err(err).Hex("blockID", g.blockID[:]).Uint64("height", g.height).
				Msgf("failed to fetch collection %v", g.colID)
			return retry.RetryableError(err)
		}

		col, err := convert.MessageToFullCollection(resp.Transactions, f.chain)
		if err != nil {
			f.log.Error().Err(err).Hex("blockID", g.blockID[:]).Uint64("height", g.height).
				Msgf("failed to convert collection %v", g.colID)
			return err
		}

		f.handler(f.originID, col)
		return nil
	})
}
