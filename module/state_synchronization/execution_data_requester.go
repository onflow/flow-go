package state_synchronization

import (
	"bytes"
	"context"
	"fmt"
	"sync"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/network"
)

// ExecutionDataRequester requests ExecutionData for given root CIDs from the network.
type ExecutionDataRequester struct {
	blockExchange  network.BlockExchange
	activeRequests sync.Map
	serializer     *serializer
}

// RequestExecutionData requests the execution data for the given root CID.
func (s *ExecutionDataRequester) RequestExecutionData(ctx context.Context, rootCid cid.Cid) ExecutionDataRequest {
	req, exists := s.activeRequests.LoadOrStore(rootCid, &executionDataRequestImpl{
		cid:  rootCid,
		done: make(chan struct{}),
	})
	request := req.(*executionDataRequestImpl)
	if !exists {
		go s.handleRequest(ctx, request)
	}

	return request
}

func (s *ExecutionDataRequester) getData(ctx context.Context, session network.BlockExchangeFetcher, cids []cid.Cid) (reader, error) {
	pendingCids := atomic.NewUint32(uint32(len(cids)))
	indices := make(map[cid.Cid]int)
	for i, cid := range cids {
		indices[cid] = i
	}

	buf := make([]byte, (len(cids)-1)*MAX_BLOCK_SIZE)
	var lastBlock blocks.Block

	done, err := session.GetBlocks(cids...).ForEach(func(b blocks.Block) {
		i := indices[b.Cid()]
		if i == len(cids)-1 {
			lastBlock = b
			return
		}
		start := i * MAX_BLOCK_SIZE
		end := start + len(b.RawData())
		copy(buf[start:end], b.RawData())
		pendingCids.Dec()
	}).Send(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get blocks: %w", err)
	}

	<-done

	// check if there are any unfullfilled requests
	if pendingCids.Load() > 0 {
		if util.CheckClosed(ctx.Done()) {
			return nil, ctx.Err()
		} else {
			// TODO: In practice, we will only get here if the parent context for all
			// of Bitswap was canceled. Hence, we just use the generic context.Canceled.
			// Nevertheless, maybe we could define a custom error type instead?
			return nil, context.Canceled
		}
	}

	r := bytes.NewBuffer(buf)
	if _, err := r.Write(lastBlock.RawData()); err != nil {
		return nil, fmt.Errorf("failed to write block to buffer: %w", err)
	}

	return r, nil
}

func (s *ExecutionDataRequester) handleRequest(ctx context.Context, request *executionDataRequestImpl) {
	session := s.blockExchange.GetSession(ctx)

	cids := []cid.Cid{request.cid}

	for {
		r, err := s.getData(ctx, session, cids)
		if err != nil {
			request.setError(err)
			return
		}

		v, err := s.serializer.deserialize(r)
		if err != nil {
			request.setError(fmt.Errorf("failed to deserialize execution data: %w", err))
			return
		}

		switch v := v.(type) {
		case *ExecutionData:
			request.setExecutionData(v)
			return
		case *[]cid.Cid:
			cids = *v
		}
	}
}
