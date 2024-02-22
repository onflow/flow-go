package util

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"

	"github.com/fxamacker/cbor/v2"
	"github.com/rs/zerolog"

	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/complete/wal"
)

const (
	defaultBufioWriteSize = 1024 * 32
	defaultBufioReadSize  = 1024 * 32

	payloadEncodingVersion = 1
)

const (
	MagicBytesPayloadHeader uint16 = 0x2138

	PayloadFileVersionV1 uint16 = 0x01

	encMagicBytesIndex       = 0
	encMagicBytesSize        = 2
	encVersionIndex          = 2
	encVersionSize           = 2
	encFlagIndex             = 4
	encFlagLowByteIndex      = 5
	encPartialStateFlagIndex = encFlagLowByteIndex
	encFlagSize              = 2
	headerSize               = encMagicBytesSize + encVersionSize + encFlagSize
	encPayloadCountSize      = 8
	footerSize               = encPayloadCountSize
	crc32SumSize             = 4
)

const (
	maskPartialState byte = 0b0000_0001
)

// newPayloadFileHeader() returns payload header, consisting of:
// - magic bytes (2 bytes)
// - version (2 bytes)
// - flags (2 bytes)
func newPayloadFileHeader(version uint16, partialState bool) []byte {
	var header [headerSize]byte

	// Write magic bytes.
	binary.BigEndian.PutUint16(header[encMagicBytesIndex:], MagicBytesPayloadHeader)

	// Write version.
	binary.BigEndian.PutUint16(header[encVersionIndex:], version)

	// Write flag.
	if partialState {
		header[encPartialStateFlagIndex] |= maskPartialState
	}

	return header[:]
}

// parsePayloadFileHeader verifies magic bytes and version in payload header.
func parsePayloadFileHeader(header []byte) (partialState bool, err error) {
	if len(header) != headerSize {
		return false, fmt.Errorf("can't parse payload header: got %d bytes, expected %d bytes", len(header), headerSize)
	}

	// Read magic bytes.
	gotMagicBytes := binary.BigEndian.Uint16(header[encMagicBytesIndex:])
	if gotMagicBytes != MagicBytesPayloadHeader {
		return false, fmt.Errorf("can't parse payload header: got magic bytes %d, expected %d", gotMagicBytes, MagicBytesPayloadHeader)
	}

	// Read version.
	gotVersion := binary.BigEndian.Uint16(header[encVersionIndex:])
	if gotVersion != PayloadFileVersionV1 {
		return false, fmt.Errorf("can't parse payload header: got version %d, expected %d", gotVersion, PayloadFileVersionV1)
	}

	// Read partial state flag.
	partialState = header[encPartialStateFlagIndex]&maskPartialState != 0

	return partialState, nil
}

// newPayloadFileFooter returns payload footer.
// - payload count (8 bytes)
func newPayloadFileFooter(payloadCount int) []byte {
	var footer [footerSize]byte

	binary.BigEndian.PutUint64(footer[:], uint64(payloadCount))

	return footer[:]
}

// parsePayloadFooter returns payload count from footer.
func parsePayloadFooter(footer []byte) (payloadCount int, err error) {
	if len(footer) != footerSize {
		return 0, fmt.Errorf("can't parse payload footer: got %d bytes, expected %d bytes", len(footer), footerSize)
	}

	count := binary.BigEndian.Uint64(footer)
	if count > math.MaxInt {
		return 0, fmt.Errorf("can't parse payload footer: got %d payload count, expected payload count < %d", count, math.MaxInt)
	}

	return int(count), nil
}

func CreatePayloadFile(
	logger zerolog.Logger,
	payloadFile string,
	payloads []*ledger.Payload,
	addresses []common.Address,
	inputPayloadsFromPartialState bool,
) (int, error) {

	partialState := inputPayloadsFromPartialState || len(addresses) > 0

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

	// TODO: replace CRC-32 checksum.
	// For now, CRC-32 checksum is used because checkpoint files (~21GB input files) already uses CRC-32 checksum.
	// Additionally, the primary purpose of this intermediate payload file (since Feb 12, 2024) is to speed up
	// development, testing, and troubleshooting by allowing a small subset of payloads to be extracted.
	// However, we should replace it since it is inappropriate for large files as already suggested at:
	// - September 28, 2022: https://github.com/onflow/flow-go/issues/3302
	// - September 26, 2022 (asked if SHA2 should replace CRC32) https://github.com/onflow/flow-go/pull/3273#discussion_r980433612
	// - February 24, 2022 (TODO suggested BLAKE2, etc. to replace CRC32):  https://github.com/onflow/flow-go/pull/1944
	crc32Writer := wal.NewCRC32Writer(writer)

	// Write header with magic bytes, version, and flags.
	header := newPayloadFileHeader(PayloadFileVersionV1, partialState)

	_, err = crc32Writer.Write(header)
	if err != nil {
		return 0, fmt.Errorf("can't write payload file head for %s: %w", payloadFile, err)
	}

	includeAllPayloads := len(addresses) == 0

	// Write payloads.
	var writtenPayloadCount int
	if includeAllPayloads {
		writtenPayloadCount, err = writePayloads(logger, crc32Writer, payloads)
	} else {
		writtenPayloadCount, err = writeSelectedPayloads(logger, crc32Writer, payloads, addresses)
	}

	if err != nil {
		return 0, fmt.Errorf("can't write payload for %s: %w", payloadFile, err)
	}

	// Write footer with payload count.
	footer := newPayloadFileFooter(writtenPayloadCount)

	_, err = crc32Writer.Write(footer)
	if err != nil {
		return 0, fmt.Errorf("can't write payload footer for %s: %w", payloadFile, err)
	}

	// Write CRC32 sum for validation
	var crc32buf [crc32SumSize]byte
	binary.BigEndian.PutUint32(crc32buf[:], crc32Writer.Crc32())

	_, err = writer.Write(crc32buf[:])
	if err != nil {
		return 0, fmt.Errorf("can't write CRC32 for %s: %w", payloadFile, err)
	}

	return writtenPayloadCount, nil
}

func writePayloads(logger zerolog.Logger, w io.Writer, payloads []*ledger.Payload) (int, error) {
	logger.Info().Msgf("writing %d payloads to file", len(payloads))

	enc := cbor.NewEncoder(w)

	var payloadScratchBuffer [1024 * 2]byte
	for _, p := range payloads {

		buf := ledger.EncodeAndAppendPayloadWithoutPrefix(payloadScratchBuffer[:0], p, payloadEncodingVersion)

		// Encode payload
		err := enc.Encode(buf)
		if err != nil {
			return 0, err
		}
	}

	return len(payloads), nil
}

func writeSelectedPayloads(logger zerolog.Logger, w io.Writer, payloads []*ledger.Payload, addresses []common.Address) (int, error) {
	logger.Info().Msgf("filtering %d payloads and writing selected payloads to file", len(payloads))

	enc := cbor.NewEncoder(w)

	var includedPayloadCount int
	var payloadScratchBuffer [1024 * 2]byte
	for _, p := range payloads {
		include, err := includePayloadByAddresses(p, addresses)
		if err != nil {
			return 0, err
		}
		if !include {
			continue
		}

		buf := ledger.EncodeAndAppendPayloadWithoutPrefix(payloadScratchBuffer[:0], p, payloadEncodingVersion)

		// Encode payload
		err = enc.Encode(buf)
		if err != nil {
			return 0, err
		}

		includedPayloadCount++
	}

	return includedPayloadCount, nil
}

func includePayloadByAddresses(payload *ledger.Payload, addresses []common.Address) (bool, error) {
	if len(addresses) == 0 {
		// Include all payloads
		return true, nil
	}

	k, err := payload.Key()
	if err != nil {
		return false, fmt.Errorf("can't get key from payload: %w", err)
	}

	owner := k.KeyParts[0].Value

	for _, address := range addresses {
		if bytes.Equal(owner, address[:]) {
			return true, nil
		}
	}

	return false, nil
}

func ReadPayloadFile(logger zerolog.Logger, payloadFile string) (bool, []*ledger.Payload, error) {

	fInfo, err := os.Stat(payloadFile)
	if os.IsNotExist(err) {
		return false, nil, fmt.Errorf("%s doesn't exist", payloadFile)
	}

	fsize := fInfo.Size()

	f, err := os.Open(payloadFile)
	if err != nil {
		return false, nil, fmt.Errorf("can't open %s: %w", payloadFile, err)
	}
	defer f.Close()

	partialState, payloadCount, err := readMetaDataFromPayloadFile(f)
	if err != nil {
		return false, nil, err
	}

	bufReader := bufio.NewReaderSize(f, defaultBufioReadSize)

	crcReader := wal.NewCRC32Reader(bufReader)

	// Skip header (processed already)
	_, err = io.CopyN(io.Discard, crcReader, headerSize)
	if err != nil {
		return false, nil, fmt.Errorf("can't read and discard header: %w", err)
	}

	if partialState {
		logger.Info().Msgf("reading %d payloads (partial state) from file", payloadCount)
	} else {
		logger.Info().Msgf("reading %d payloads from file", payloadCount)
	}

	encPayloadSize := fsize - headerSize - footerSize - crc32SumSize

	// NOTE: We need to limit the amount of data CBOR codec reads
	// because CBOR codec reads chunks of data under the hood for
	// performance and we don't want crcReader to proces data
	// containing CRC-32 checksum.
	dec := cbor.NewDecoder(io.LimitReader(crcReader, encPayloadSize))

	payloads := make([]*ledger.Payload, payloadCount)

	for i := 0; i < payloadCount; i++ {
		var rawPayload []byte
		err := dec.Decode(&rawPayload)
		if err != nil {
			return false, nil, fmt.Errorf("can't decode payload in CBOR: %w", err)
		}

		payload, err := ledger.DecodePayloadWithoutPrefix(rawPayload, false, payloadEncodingVersion)
		if err != nil {
			return false, nil, fmt.Errorf("can't decode payload 0x%x: %w", rawPayload, err)
		}

		payloads[i] = payload
	}

	// Skip footer (processed already)
	_, err = io.CopyN(io.Discard, crcReader, footerSize)
	if err != nil {
		return false, nil, fmt.Errorf("can't read and discard footer: %w", err)
	}

	// Read CRC32
	var crc32buf [crc32SumSize]byte
	_, err = io.ReadFull(bufReader, crc32buf[:])
	if err != nil {
		return false, nil, fmt.Errorf("can't read CRC32: %w", err)
	}

	readCrc32 := binary.BigEndian.Uint32(crc32buf[:])

	calculatedCrc32 := crcReader.Crc32()

	if calculatedCrc32 != readCrc32 {
		return false, nil, fmt.Errorf("payload file checksum failed! File contains %x but calculated crc32 is %x", readCrc32, calculatedCrc32)
	}

	// Verify EOF is reached
	_, err = io.CopyN(io.Discard, bufReader, 1)
	if err == nil || err != io.EOF {
		return false, nil, fmt.Errorf("can't process payload file: found trailing data")
	}

	return partialState, payloads, nil
}

// readMetaDataFromPayloadFile reads metadata from header and footer.
// NOTE: readMetaDataFromPayloadFile resets file offset to start of file in exit.
func readMetaDataFromPayloadFile(f *os.File) (partialState bool, payloadCount int, err error) {
	defer func() {
		_, seekErr := f.Seek(0, io.SeekStart)
		if err == nil {
			err = seekErr
		}
	}()

	// Seek to header
	_, err = f.Seek(0, io.SeekStart)
	if err != nil {
		return false, 0, fmt.Errorf("can't seek to start of payload file: %w", err)
	}

	var header [headerSize]byte

	// Read header
	_, err = io.ReadFull(f, header[:])
	if err != nil {
		return false, 0, fmt.Errorf("can't read payload header: %w", err)
	}

	// Parse header
	partialState, err = parsePayloadFileHeader(header[:])
	if err != nil {
		return false, 0, err
	}

	const footerOffset = footerSize + crc32SumSize

	// Seek to footer
	_, err = f.Seek(-footerOffset, io.SeekEnd)
	if err != nil {
		return false, 0, fmt.Errorf("can't seek to payload footer: %w", err)
	}

	var footer [footerSize]byte

	// Read footer
	_, err = io.ReadFull(f, footer[:])
	if err != nil {
		return false, 0, fmt.Errorf("can't read payload footer: %w", err)
	}

	// Parse footer
	payloadCount, err = parsePayloadFooter(footer[:])
	if err != nil {
		return false, 0, err
	}

	return partialState, payloadCount, nil
}

func IsPayloadFilePartialState(payloadFile string) (bool, error) {
	if _, err := os.Stat(payloadFile); os.IsNotExist(err) {
		return false, fmt.Errorf("%s doesn't exist", payloadFile)
	}

	f, err := os.Open(payloadFile)
	if err != nil {
		return false, fmt.Errorf("can't open %s: %w", payloadFile, err)
	}
	defer f.Close()

	var header [headerSize]byte

	// Read header
	_, err = io.ReadFull(f, header[:])
	if err != nil {
		return false, fmt.Errorf("can't read payload header: %w", err)
	}

	return header[encPartialStateFlagIndex]&maskPartialState != 0, nil
}
