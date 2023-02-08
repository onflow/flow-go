package importer

import (
	"encoding/binary"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/complete/mtrie/flattener"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/module/util"
)

// ImportLeafNodesFromCheckpoint takes a checkpoint file specified by the dir and fileName,
// reads all the leaf nodes from the checkpoint file, and store them into the given
// storage store.
func ImportLeafNodesFromCheckpoint(dir string, fileName string, logger *zerolog.Logger, store ledger.PayloadStorage) error {
	leafNodes, err := wal.ReadLeafNodesFromCheckpointV6(dir, fileName, logger)
	if err != nil {
		return fmt.Errorf("could not read tries: %w", err)
	}

	logProgress := util.LogProgress("importing leaf nodes to storage", len(leafNodes), logger)
	batchSize := 10
	batch := make([]ledger.LeafNode, 0, batchSize)
	for i, leafNode := range leafNodes {
		logProgress(i)
		batch = append(batch, *leafNode)
		if len(batch) >= batchSize {
			err := store.Add(batch)
			if err != nil {
				return fmt.Errorf("could not store leaf nodes: %w", err)
			}
		}
	}

	if len(batch) > 0 {
		err := store.Add(batch)
		if err != nil {
			return fmt.Errorf("could not store leaf nodes: %w", err)
		}
	}

	return nil
}

// func importLeafNodesConcurrently(jobs <-chan *ledger.LeafNode, logger *zerolog.Logger, store ledger.Storage) error {
//  results := make(chan error)
//
//  nWorker := 10
//
//  ctx, cancel := context.WithCancel(context.Background())
//  defer cancel()
//
//  // create nWorker number of workers
//  for w := 0; w < nWorker; w++ {
//      go func() {
//          scratch := make([]byte, 1024*4)
//          for leafNode := range jobs {
//              err := storeLeafNode(store, leafNode, scratch)
//              results <- err
//
//              if err != nil {
//                  cancel()
//                  return
//              }
//
//              select {
//              case <-ctx.Done():
//                  return
//              default:
//              }
//          }
//      }()
//  }
//
//  logProgress := util.LogProgress("importing leaf nodes to storage", len(leafNodes), logger)
//  // waiting for results
//  for i := 0; i < len(leafNodes); i++ {
//      logProgress(i)
//      err := <-results
//      if err != nil {
//          return err
//      }
//  }
//
//  return nil
// }

const (
	encHeightSize        = 2
	encHashSize          = hash.HashLen
	encPathSize          = ledger.PathLen
	encPayloadLengthSize = 4
)

func EncodePayload(path ledger.Path, payload *ledger.Payload, scratch []byte) ([]byte, error) {
	encPayloadSize := ledger.EncodedPayloadLengthWithoutPrefix(payload, flattener.PayloadEncodingVersion)

	encodedNodeSize := encPathSize +
		encPayloadLengthSize +
		encPayloadSize

	buf := scratch
	if len(scratch) < encodedNodeSize {
		buf = make([]byte, encPathSize+encPayloadLengthSize, encodedNodeSize)
	}

	pos := 0

	// encode path (32 bytes path)
	copy(buf[pos:], path[:])
	pos += encPathSize

	// encode payload (4 bytes Big Endian for encoded payload length and n bytes encoded payload)
	binary.BigEndian.PutUint32(buf[pos:], uint32(encPayloadSize))
	pos += encPayloadLengthSize

	// EncodeAndAppendPayloadWithoutPrefix appends encoded payload to the resliced buf.
	// Returned buf is resliced to include appended payload.
	buf = ledger.EncodeAndAppendPayloadWithoutPrefix(buf[:pos], payload, flattener.PayloadEncodingVersion)
	return buf, nil
}

func DecodePayload(encoded []byte) (ledger.Path, *ledger.Payload, error) {
	if len(encoded) < encPathSize+encPayloadLengthSize {
		return ledger.DummyPath, nil, fmt.Errorf("could not decode leaf node, not enough bytes: %v", len(encoded))
	}

	pos := 0

	// decode path
	path, err := ledger.ToPath(encoded[pos : pos+encPathSize])
	if err != nil {
		return ledger.DummyPath, nil, fmt.Errorf("could node decode path: %w", err)
	}
	pos += encPathSize

	// decode payload size
	expectedSize := binary.BigEndian.Uint32(encoded[pos : pos+encPayloadLengthSize])
	pos += encPayloadLengthSize

	// decode payload
	actualSize := uint32(len(encoded) - pos)
	if expectedSize != actualSize {
		return ledger.DummyPath, nil, fmt.Errorf("incorrect payload size, expect %v, actual %v", expectedSize, actualSize)
	}

	payload, err := ledger.DecodePayloadWithoutPrefix(encoded[pos:], false, flattener.PayloadEncodingVersion)
	if err != nil {
		return ledger.DummyPath, nil, fmt.Errorf("could not decode payload: %w", err)
	}

	return path, payload, nil
}
