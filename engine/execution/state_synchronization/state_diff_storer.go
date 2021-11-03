package state_synchronization

import (
	"bytes"
	"fmt"
	"io"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	blockstore "github.com/ipfs/go-ipfs-blockstore"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
)

const MAX_BLOCK_SIZE = 1e6 // 1MB

type ExecutionStateDiff struct {
	Collections        []*flow.Collection
	Events             []*flow.Event
	TrieUpdate         []*ledger.TrieUpdate
	TransactionResults []*flow.TransactionResult
}

type StateDiffStorer struct {
	blockWriter *BlockWriter
	codec       encoding.Codec
	compressor  network.Compressor
}

func NewStateDiffStorer(
	codec encoding.Codec,
	compressor network.Compressor,
	bstore blockstore.Blockstore,
) (*StateDiffStorer, error) {
	bw := &BlockWriter{
		maxBlockSize: MAX_BLOCK_SIZE,
		bstore:       bstore,
	}
	return &StateDiffStorer{bw, codec, compressor}, nil
}

func (s *StateDiffStorer) writeBlocks(v interface{}) ([]cid.Cid, error) {
	comp, err := s.compressor.NewWriter(s.blockWriter)
	if err != nil {
		return nil, err
	}
	enc := s.codec.NewEncoder(comp)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	if err := comp.Close(); err != nil {
		return nil, err
	}
	if err := s.blockWriter.Flush(); err != nil {
		return nil, err
	}
	cids := s.blockWriter.GetWrittenCids()
	s.blockWriter.Reset()
	return cids, nil
}

func (s *StateDiffStorer) Store(sd *ExecutionStateDiff) (cid.Cid, error) {
	cids, err := s.writeBlocks(sd)
	if err != nil {
		return cid.Undef, fmt.Errorf("failed to write state diff blocks: %w", err)
	}

	for {
		if len(cids) == 1 {
			return cids[0], nil
		}

		if cids, err = s.writeBlocks(cids); err != nil {
			return cid.Undef, fmt.Errorf("failed to write cid blocks: %w", err)
		}
	}
}

func (s *StateDiffStorer) readBlocks(cids []cid.Cid, v interface{}) error {
	buf := &bytes.Buffer{}
	comp, err := s.compressor.NewReader(buf)
	if err != nil {
		return err
	}
	dec := s.codec.NewDecoder(comp)
	for _, c := range cids {
		block, err := s.blockWriter.bstore.Get(c)
		if err != nil {
			return fmt.Errorf("failed to get block %v from blockstore: %w", c, err)
		}
		_, _ = buf.Write(block.RawData()) // never returns error
	}
	return dec.Decode(v)
}

// Load loads the ExecutionStateDiff represented by the given CID from the blockstore.
// Since blocks are limited to MAX_BLOCK_SIZE bytes, it's possible that the data was
// stored in multiple chunks, and hence the root CID may point to a block for which
// the data itself is a list of concatenated CIDs.
func (s *StateDiffStorer) Load(c cid.Cid) (*ExecutionStateDiff, error) {
	cids := []cid.Cid{c}
	for {
		var sd ExecutionStateDiff
		if err := s.readBlocks(cids, &sd); err == nil {
			return &sd, nil
		}
		if err := s.readBlocks(cids, &cids); err != nil {
			return nil, fmt.Errorf("could not parse blocks as cids: %w", err)
		}
	}
}

var _ io.Writer = (*BlockWriter)(nil)

type BlockWriter struct {
	maxBlockSize int
	bstore       blockstore.Blockstore
	cids         []cid.Cid
	buf          []byte // unflushed bytes from previous write
}

func (bw *BlockWriter) Reset() {
	bw.cids = nil
	bw.buf = nil
}

func (bw *BlockWriter) GetWrittenCids() []cid.Cid {
	return bw.cids
}

func (bw *BlockWriter) Flush() error {
	if len(bw.buf) > 0 {
		if err := bw.writeBlock(bw.buf); err != nil {
			return err
		}
		bw.buf = nil
	}
	return nil
}

func (bw *BlockWriter) writeBlock(data []byte) error {
	block := blocks.NewBlock(data)
	if err := bw.bstore.Put(block); err != nil {
		return fmt.Errorf("failed to put block %v into blockstore: %w", block.Cid(), err)
	}
	bw.cids = append(bw.cids, block.Cid())
	return nil
}

func (bw *BlockWriter) Write(p []byte) (n int, err error) {
	var chunk []byte
	if leftover := len(bw.buf); leftover > 0 {
		fill := bw.maxBlockSize - leftover
		if fill > len(p) {
			bw.buf = append(bw.buf, p...)
			return len(p), nil
		}
		chunk, p = append(bw.buf, p[:fill]...), p[fill:]
		if err = bw.writeBlock(chunk); err != nil {
			return
		} else {
			n += fill
		}
		bw.buf = nil
	}
	for len(p) >= bw.maxBlockSize {
		chunk, p = p[:bw.maxBlockSize], p[bw.maxBlockSize:]
		if err = bw.writeBlock(chunk); err != nil {
			return
		} else {
			n += bw.maxBlockSize
		}
	}
	if len(p) > 0 {
		bw.buf = make([]byte, 0, bw.maxBlockSize)
		bw.buf = append(bw.buf, p...)
		n += len(bw.buf)
	}
	return
}
