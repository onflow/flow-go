package storage

import (
	"encoding/binary"
	"fmt"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/complete/mtrie/flattener"
)

// ImportLeafNodesFromCheckpoint takes a checkpoint file specified by the dir and fileName,
// reads all the leaf nodes from the checkpoint file, and store them into the given
// storage store.
// func ImportLeafNodesFromCheckpoint(dir string, fileName string, logger *zerolog.Logger, store ledger.Storage) error {
// 	tries, err := wal.OpenAndReadCheckpointV6(dir, fileName, logger)
// 	if err != nil {
// 		return fmt.Errorf("could not read tries: %w", err)
// 	}
//
// 	if len(tries) == 0 {
// 		return fmt.Errorf("could not find any trie root")
// 	}
//
// 	trie := tries[len(tries)-1]
// 	leafNodes := trie.AllLeafNodes()
//
// 	err = importLeafNodesConcurrently(leafNodes, logger, store)
// 	if err != nil {
// 		return fmt.Errorf("fail to store the leafNode to store: %w", err)
// 	}
//
// 	return nil
// }
//
// func importLeafNodesConcurrently(leafNodes []*node.Node, logger *zerolog.Logger, store ledger.Storage) error {
// 	jobs := make(chan *node.Node, len(leafNodes))
// 	results := make(chan error, len(leafNodes))
//
// 	nWorker := 10
// 	if len(leafNodes) < nWorker {
// 		nWorker = len(leafNodes)
// 	}
//
// 	ctx, cancel := context.WithCancel(context.Background())
// 	defer cancel()
//
// 	// create nWorker number of workers
// 	for w := 0; w < nWorker; w++ {
// 		go func() {
// 			scratch := make([]byte, 1024*4)
// 			for leafNode := range jobs {
// 				err := storeLeafNode(store, leafNode, scratch)
// 				results <- err
//
// 				if err != nil {
// 					cancel()
// 					return
// 				}
//
// 				select {
// 				case <-ctx.Done():
// 					return
// 				default:
// 				}
// 			}
// 		}()
// 	}
//
// 	// buffer all jobs
// 	for _, leafNode := range leafNodes {
// 		jobs <- leafNode
// 	}
// 	close(jobs)
//
// 	logProgress := util.LogProgress("importing leaf nodes to storage", len(leafNodes), logger)
// 	// waiting for results
// 	for i := 0; i < len(leafNodes); i++ {
// 		logProgress(i)
// 		err := <-results
// 		if err != nil {
// 			return err
// 		}
// 	}
//
// 	return nil
// }
//
// func storeLeafNode(store ledger.Storage, leafNode *node.Node, scratch []byte) error {
// 	hash, encoded, err := EncodeLeafNode(leafNode, scratch)
// 	if err != nil {
// 		return fmt.Errorf("could not encode leaf node: %w", err)
// 	}
//
// 	err = store.Set(hash, encoded)
// 	if err != nil {
// 		return fmt.Errorf("could not store encoded leaf node: %w", err)
// 	}
// 	return nil
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
		buf = make([]byte, encodedNodeSize)
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
