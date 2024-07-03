package metrics

import (
	"fmt"
	"sync"

	"github.com/rs/zerolog"
)

// provider is responsible for providing the metrics for the rpc endpoint
// it has a circular buffer of metrics for the last N finalized and executed blocks.
type provider struct {
	log zerolog.Logger

	mu *sync.RWMutex

	bufferSize               uint
	bufferIndex              uint
	blockHeightAtBufferIndex uint64

	buffer [][]TransactionExecutionMetrics
}

func newProvider(
	log zerolog.Logger,
	bufferSize uint,
	blockHeightAtBufferIndex uint64,
) *provider {
	if bufferSize == 0 {
		panic("buffer size must be greater than zero")
	}

	return &provider{
		mu:                       &sync.RWMutex{},
		log:                      log,
		bufferSize:               bufferSize,
		blockHeightAtBufferIndex: blockHeightAtBufferIndex,
		bufferIndex:              0,
		buffer:                   make([][]TransactionExecutionMetrics, bufferSize),
	}
}

func (p *provider) Push(
	height uint64,
	data []TransactionExecutionMetrics,
) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if height <= p.blockHeightAtBufferIndex {
		p.log.Warn().
			Uint64("height", height).
			Uint64("latestHeight", p.blockHeightAtBufferIndex).
			Msg("received metrics for a block that is older or equal than the most recent block")
		return
	}
	if height > p.blockHeightAtBufferIndex+1 {
		p.log.Warn().
			Uint64("height", height).
			Uint64("latestHeight", p.blockHeightAtBufferIndex).
			Msg("received metrics for a block that is not the next block")

		// Fill in the gap with nil
		for i := p.blockHeightAtBufferIndex; i < height-1; i++ {
			p.pushData(nil)
		}
	}

	p.pushData(data)
}

func (p *provider) pushData(data []TransactionExecutionMetrics) {
	p.bufferIndex = (p.bufferIndex + 1) % p.bufferSize
	p.blockHeightAtBufferIndex++
	p.buffer[p.bufferIndex] = data
}

func (p *provider) GetTransactionExecutionMetricsAfter(height uint64) (GetTransactionExecutionMetricsAfterResponse, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	data := make(map[uint64][]TransactionExecutionMetrics)

	if height+1 > p.blockHeightAtBufferIndex {
		return data, nil
	}

	// start index is the lowest block height that is in the buffer
	startIndex := uint64(0)
	if p.blockHeightAtBufferIndex < uint64(p.bufferSize) {
		startIndex = 0
	} else {
		startIndex = p.blockHeightAtBufferIndex - uint64(p.bufferSize)
	}

	// if the starting index is lower than the height we only need to return the data for
	// the blocks that are later than the given height
	if height+1 > startIndex {
		startIndex = height + 1
	}

	for i := startIndex; i <= p.blockHeightAtBufferIndex; i++ {
		// 0 <= diff
		diff := uint(p.blockHeightAtBufferIndex - i)

		//  0 <= diff < bufferSize
		// we add bufferSize to avoid negative values
		index := (p.bufferIndex + (p.bufferSize - diff)) % p.bufferSize
		d := p.buffer[index]
		if len(d) == 0 {
			continue
		}

		data[i] = p.buffer[index]
	}

	return data, nil
}

var NoTransactionExecutionMetricsError = fmt.Errorf("no transaction execution metrics available")
