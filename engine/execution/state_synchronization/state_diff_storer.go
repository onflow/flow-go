package state_synchronization

import (
	"bytes"
	"context"
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

const (
	CodeIntermediateCIDs = iota
	CodeExecutionStateDiff
)

func getCode(v interface{}) byte {
	switch v.(type) {
	case *ExecutionStateDiff:
		return CodeExecutionStateDiff
	case []cid.Cid:
		return CodeIntermediateCIDs
	default:
		panic(fmt.Sprintf("invalid type for interface: %T", v))
	}
}

func getPrototype(code byte) interface{} {
	switch code {
	case CodeExecutionStateDiff:
		return &ExecutionStateDiff{}
	case CodeIntermediateCIDs:
		return &[]cid.Cid{}
	default:
		panic(fmt.Sprintf("invalid code: %v", code))
	}
}

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
	if _, err := s.blockWriter.Write([]byte{getCode(v)}); err != nil {
		return nil, err
	}
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

func (s *StateDiffStorer) readBlocks(cids []cid.Cid) (interface{}, error) {
	buf := &bytes.Buffer{}
	comp, err := s.compressor.NewReader(buf)
	if err != nil {
		return nil, err
	}
	dec := s.codec.NewDecoder(comp)
	var code byte
	for i, c := range cids {
		// TODO: cheating on this context since this code is unused and about to be completely refactored
		block, err := s.blockWriter.bstore.Get(context.Background(), c)
		if err != nil {
			return nil, fmt.Errorf("failed to get block %v from blockstore: %w", c, err)
		}
		data := block.RawData()
		if i == 0 {
			code = data[0]
			data = data[1:]
		}
		_, _ = buf.Write(data) // never returns error
	}
	v := getPrototype(code)
	err = dec.Decode(v)
	return v, err
}

// Load loads the ExecutionStateDiff represented by the given CID from the blockstore.
// Since blocks are limited to MAX_BLOCK_SIZE bytes, it's possible that the data was
// stored in multiple chunks, and hence the root CID may point to a block for which
// the data itself is a list of concatenated CIDs.
func (s *StateDiffStorer) Load(c cid.Cid) (*ExecutionStateDiff, error) {
	cids := []cid.Cid{c}
	for {
		v, err := s.readBlocks(cids)
		if err != nil {
			return nil, fmt.Errorf("could not read blocks: %w", err)
		}
		switch v := v.(type) {
		case *ExecutionStateDiff:
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
	// TODO: cheating on this context since this code is unused and about to be completely refactored
	if err := bw.bstore.Put(context.Background(), block); err != nil {
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
