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

// ExecutionData represents the execution data of a block
type ExecutionData struct {
	BlockID            flow.Identifier
	Collections        []*flow.Collection
	Events             []*flow.Event
	TrieUpdate         []*ledger.TrieUpdate
	TransactionResults []*flow.TransactionResult
}

// ExecutionDataStorer handles storing and loading execution data from a blockstore
type ExecutionDataStorer struct {
	blockWriter *BlockWriter
	serializer  *serializer
}

func NewExecutionDataStorer(
	codec encoding.Codec,
	compressor network.Compressor,
	bstore blockstore.Blockstore,
) (*ExecutionDataStorer, error) {
	bw := &BlockWriter{
		maxBlockSize: MAX_BLOCK_SIZE,
		bstore:       bstore,
	}
	return &ExecutionDataStorer{bw, &serializer{codec, compressor}}, nil
}

func (s *ExecutionDataStorer) writeBlocks(v interface{}) ([]cid.Cid, error) {
	if err := s.serializer.serialize(s.blockWriter, v); err != nil {
		return nil, err
	}
	if err := s.blockWriter.Flush(); err != nil {
		return nil, err
	}
	cids := s.blockWriter.GetWrittenCids()
	s.blockWriter.Reset()
	return cids, nil
}

// Store stores the given ExecutionData into the blockstore and returns the root CID.
// Since blocks are limited to MAX_BLOCK_SIZE bytes, it's possible that the data may
// be stored in multiple blocks, and hence the returned root CID may point to a block
// for which the data itself represents a recursive list of CIDs.
func (s *ExecutionDataStorer) Store(sd *ExecutionData) (cid.Cid, error) {
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

func (s *ExecutionDataStorer) readBlocks(cids []cid.Cid) (interface{}, error) {
	buf := &bytes.Buffer{}
	for _, c := range cids {
		block, err := s.blockWriter.bstore.Get(c)
		if err != nil {
			return nil, fmt.Errorf("failed to get block %v from blockstore: %w", c, err)
		}
		_, _ = buf.Write(block.RawData()) // never returns error
	}
	return s.serializer.deserialize(buf)
}

// Load loads the ExecutionData represented by the given CID from the blockstore.
// Since blocks are limited to MAX_BLOCK_SIZE bytes, it's possible that the data was
// stored in multiple blocks, and hence the root CID may point to a block for which
// the data itself represents a recursive list of CIDs.
func (s *ExecutionDataStorer) Load(c cid.Cid) (*ExecutionData, error) {
	cids := []cid.Cid{c}
	for {
		v, err := s.readBlocks(cids)
		if err != nil {
			return nil, fmt.Errorf("could not read blocks: %w", err)
		}
		switch v := v.(type) {
		case *ExecutionData:
			return v, nil
		case *[]cid.Cid:
			cids = *v
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

func (bw *BlockWriter) WriteByte(c byte) error {
	bw.buf = append(bw.buf, c)
	if len(bw.buf) >= bw.maxBlockSize {
		if err := bw.writeBlock(bw.buf); err != nil {
			return err
		}
		bw.buf = nil
	}
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
		bw.buf = make([]byte, 0, len(p))
		bw.buf = append(bw.buf, p...)
		n += len(bw.buf)
	}
	return
}
