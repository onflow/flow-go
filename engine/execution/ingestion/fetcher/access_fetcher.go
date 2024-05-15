package fetcher

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/onflow/flow/protobuf/go/flow/access"

	"github.com/onflow/flow-go/engine/common/requester"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/engine/execution/ingestion"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/utils/grpcutils"
)

type AccessCollectionFetcher struct {
	*component.ComponentManager
	log zerolog.Logger

	handler  requester.HandleFunc
	client   access.AccessAPIClient
	chain    flow.Chain
	originID flow.Identifier
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
	return &AccessCollectionFetcher{
		log:      logger.With().Str("engine", "collection_fetcher").Logger(),
		handler:  nil,
		client:   access.NewAccessAPIClient(collectionRPCConn),
		chain:    chain,
		originID: nodeID,
	}, nil
}

func (f *AccessCollectionFetcher) FetchCollection(blockID flow.Identifier, height uint64, guarantee *flow.CollectionGuarantee) error {
	go func(colID flow.Identifier) {
		f.log.Debug().Hex("blockID", blockID[:]).Uint64("height", height).Hex("col_id", colID[:]).Msgf("fetching collection")

		resp, err := f.client.GetFullCollectionByID(context.Background(), &access.GetFullCollectionByIDRequest{
			Id: colID[:],
		})
		if err != nil {
			log.Error().Err(err).Msgf("failed to fetch collection %v", colID)
			return
		}

		if f.handler != nil {
			col, err := convert.MessageToFullCollection(resp.Transactions, f.chain)
			if err != nil {
				log.Error().Err(err).Msgf("failed to convert collection %v", colID)
				return
			}
			f.handler(f.originID, col)
		}
	}(guarantee.CollectionID)
	return nil
}

func (f *AccessCollectionFetcher) Force() {
}

func (f *AccessCollectionFetcher) WithHandle(handler requester.HandleFunc) {
	f.handler = handler
}

func (f *AccessCollectionFetcher) Ready() <-chan struct{} {
	ready := make(chan struct{})
	close(ready)
	return ready
}

func (f *AccessCollectionFetcher) Done() <-chan struct{} {
	done := make(chan struct{})
	close(done)
	return done
}
