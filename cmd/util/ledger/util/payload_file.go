package util

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/fxamacker/cbor/v2"
	"github.com/rs/zerolog"

	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/ledger"
)

const (
	defaultBufioWriteSize = 1024 * 32
	defaultBufioReadSize  = 1024 * 32

	payloadEncodingVersion = 1
)

func CreatePayloadFile(
	logger zerolog.Logger,
	payloadFile string,
	payloads []*ledger.Payload,
	addresses []common.Address,
) (int, error) {

	f, err := os.Create(payloadFile)
	if err != nil {
		return 0, fmt.Errorf("can't create %s: %w", payloadFile, err)
	}
	defer f.Close()

	writer := bufio.NewWriterSize(f, defaultBufioWriteSize)
	if err != nil {
		return 0, fmt.Errorf("can't create bufio writer for %s: %w", payloadFile, err)
	}
	defer writer.Flush()

	includeAllPayloads := len(addresses) == 0

	if includeAllPayloads {
		return writeAllPayloads(logger, writer, payloads)
	}

	return writeSelectedPayloads(logger, writer, payloads, addresses)
}

func writeAllPayloads(logger zerolog.Logger, w io.Writer, payloads []*ledger.Payload) (int, error) {
	logger.Info().Msgf("writing %d payloads to file", len(payloads))

	enc := cbor.NewEncoder(w)

	// Encode number of payloads
	err := enc.Encode(len(payloads))
	if err != nil {
		return 0, fmt.Errorf("failed to encode number of payloads %d in CBOR: %w", len(payloads), err)
	}

	var payloadScratchBuffer [1024 * 2]byte
	for _, p := range payloads {

		buf := ledger.EncodeAndAppendPayloadWithoutPrefix(payloadScratchBuffer[:0], p, payloadEncodingVersion)

		// Encode payload
		err = enc.Encode(buf)
		if err != nil {
			return 0, err
		}
	}

	return len(payloads), nil
}

func writeSelectedPayloads(logger zerolog.Logger, w io.Writer, payloads []*ledger.Payload, addresses []common.Address) (int, error) {
	var includedPayloadCount int

	includedFlags := make([]bool, len(payloads))
	for i, p := range payloads {
		include, err := includePayloadByAddresses(p, addresses)
		if err != nil {
			return 0, err
		}

		includedFlags[i] = include

		if include {
			includedPayloadCount++
		}
	}

	logger.Info().Msgf("writing %d payloads to file", includedPayloadCount)

	enc := cbor.NewEncoder(w)

	// Encode number of payloads
	err := enc.Encode(includedPayloadCount)
	if err != nil {
		return 0, fmt.Errorf("failed to encode number of payloads %d in CBOR: %w", includedPayloadCount, err)
	}

	var payloadScratchBuffer [1024 * 2]byte
	for i, included := range includedFlags {
		if !included {
			continue
		}

		p := payloads[i]

		buf := ledger.EncodeAndAppendPayloadWithoutPrefix(payloadScratchBuffer[:0], p, payloadEncodingVersion)

		// Encode payload
		err = enc.Encode(buf)
		if err != nil {
			return 0, err
		}
	}

	return includedPayloadCount, nil
}

func includePayloadByAddresses(payload *ledger.Payload, addresses []common.Address) (bool, error) {
	if len(addresses) == 0 {
		// Include all payloads
		return true, nil
	}

	for _, address := range addresses {
		k, err := payload.Key()
		if err != nil {
			return false, fmt.Errorf("failed to get key from payload: %w", err)
		}

		owner := k.KeyParts[0].Value
		if bytes.Equal(owner, address[:]) {
			return true, nil
		}
	}

	return false, nil
}

func ReadPayloadFile(logger zerolog.Logger, payloadFile string) ([]*ledger.Payload, error) {

	if _, err := os.Stat(payloadFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("%s doesn't exist", payloadFile)
	}

	f, err := os.Open(payloadFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open %s: %w", payloadFile, err)
	}
	defer f.Close()

	r := bufio.NewReaderSize(f, defaultBufioReadSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create bufio reader for %s: %w", payloadFile, err)
	}

	dec := cbor.NewDecoder(r)

	// Decode number of payloads
	var payloadCount int
	err = dec.Decode(&payloadCount)
	if err != nil {
		return nil, fmt.Errorf("failed to decode number of payload in CBOR: %w", err)
	}

	logger.Info().Msgf("reading %d payloads from file", payloadCount)

	payloads := make([]*ledger.Payload, 0, payloadCount)

	for {
		var rawPayload []byte
		err := dec.Decode(&rawPayload)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to decode payload in CBOR: %w", err)
		}

		payload, err := ledger.DecodePayloadWithoutPrefix(rawPayload, false, payloadEncodingVersion)
		if err != nil {
			return nil, fmt.Errorf("failed to decode payload 0x%x: %w", rawPayload, err)
		}

		payloads = append(payloads, payload)
	}

	if payloadCount != len(payloads) {
		return nil, fmt.Errorf("failed to decode %s: expect %d payloads, got %d payloads", payloadFile, payloadCount, len(payloads))
	}

	return payloads, nil
}
