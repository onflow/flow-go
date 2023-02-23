package importer

import (
	"context"
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
	logger.Info().Msgf("start reading checkpoint file at %v/%v", dir, fileName)

	leafNodes, err := wal.ReadLeafNodesFromCheckpointV6(dir, fileName, logger)
	if err != nil {
		return fmt.Errorf("could not read tries: %w", err)
	}

	logger.Info().Msgf("start importing %v payloads to storage", len(leafNodes))

	// creating jobs for batch importing
	logCreatingJobs := util.LogProgress("creating jobs to store leaf nodes to storage", len(leafNodes), logger)
	batchSize := 1000

	jobs := make(chan []ledger.LeafNode, len(leafNodes)/batchSize+1)

	batch := make([]ledger.LeafNode, 0, batchSize)
	nBatch := 0

	for i, leafNode := range leafNodes {
		logCreatingJobs(i)
		batch = append(batch, *leafNode)
		if len(batch) >= batchSize {
			jobs <- batch
			nBatch++
			batch = make([]ledger.LeafNode, 0, batchSize)
		}
	}

	if len(batch) > 0 {
		jobs <- batch
		nBatch++
	}

	nWorker := 10
	results := make(chan error)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// create nWorker number of workers to import
	for w := 0; w < nWorker; w++ {
		go func() {
			for batch := range jobs {
				err := store.Add(batch)
				results <- err
				if err != nil {
					cancel()
					return
				}

				select {
				case <-ctx.Done():
					return
				default:
				}
			}
		}()
	}

	logImporting := util.LogProgressWithThreshold(1, "importing leaf nodes to storage", nBatch, logger)
	// waiting for the results
	for i := 0; i < nBatch; i++ {
		logImporting(i)
		err := <-results
		if err != nil {
			return err
		}
	}
	return nil
}

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
