package extractpayloads

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fxamacker/cbor/v2"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/ledger"
)

const (
	defaultBufioWriteSize = 1024 * 32
	defaultBufioReadSize  = 1024 * 32

	payloadEncodingVersion = 1
)

var (
	flagInputPayloadFileName  string
	flagOutputPayloadFileName string
	flagAddresses             string
)

var Cmd = &cobra.Command{
	Use:   "extract-payload-by-address",
	Short: "Read payload file and generate payload file containing payloads with specified addresses",
	Run:   run,
}

func init() {
	Cmd.Flags().StringVar(
		&flagInputPayloadFileName,
		"input-filename",
		"",
		"Input payload file name")
	_ = Cmd.MarkFlagRequired("input-filename")

	Cmd.Flags().StringVar(
		&flagOutputPayloadFileName,
		"output-filename",
		"",
		"Output payload file name")
	_ = Cmd.MarkFlagRequired("output-filename")

	Cmd.Flags().StringVar(
		&flagAddresses,
		"addresses",
		"",
		"extract payloads of addresses (comma separated hex-encoded addresses) to file specified by output-payload-filename",
	)
	_ = Cmd.MarkFlagRequired("addresses")
}

func run(*cobra.Command, []string) {

	if _, err := os.Stat(flagInputPayloadFileName); os.IsNotExist(err) {
		log.Fatal().Msgf("Input file %s doesn't exist", flagInputPayloadFileName)
	}

	if _, err := os.Stat(flagOutputPayloadFileName); os.IsExist(err) {
		log.Fatal().Msgf("Output file %s exists", flagOutputPayloadFileName)
	}

	addresses, err := parseAddresses(strings.Split(flagAddresses, ","))
	if err != nil {
		log.Fatal().Err(err)
	}

	log.Info().Msgf(
		"extracting payloads with address %v from %s to %s",
		addresses,
		flagInputPayloadFileName,
		flagOutputPayloadFileName,
	)

	numOfPayloadWritten, err := extractPayloads(log.Logger, flagInputPayloadFileName, flagOutputPayloadFileName, addresses)
	if err != nil {
		log.Fatal().Err(err)
	}

	err = overwritePayloadCountInFile(flagOutputPayloadFileName, numOfPayloadWritten)
	if err != nil {
		log.Fatal().Err(err)
	}
}

func overwritePayloadCountInFile(output string, numOfPayloadWritten int) error {
	in, err := os.OpenFile(output, os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("failed to open %s to write payload count: %w", output, err)
	}
	defer in.Close()

	var data [9]byte
	data[0] = 0x1b
	binary.BigEndian.PutUint64(data[1:], uint64(numOfPayloadWritten))

	n, err := in.WriteAt(data[:], 0)
	if err != nil {
		return fmt.Errorf("failed to overwrite number of payloads in %s: %w", output, err)
	}
	if n != len(data) {
		return fmt.Errorf("failed to overwrite number of payloads in %s: wrote %d bytes, expect %d bytes", output, n, len(data))
	}

	return nil
}

func extractPayloads(log zerolog.Logger, input, output string, addresses []common.Address) (int, error) {
	in, err := os.Open(input)
	if err != nil {
		return 0, fmt.Errorf("failed to open %s: %w", input, err)
	}
	defer in.Close()

	reader := bufio.NewReaderSize(in, defaultBufioReadSize)
	if err != nil {
		return 0, fmt.Errorf("failed to create bufio reader for %s: %w", input, err)
	}

	out, err := os.Create(output)
	if err != nil {
		return 0, fmt.Errorf("failed to open %s: %w", output, err)
	}
	defer out.Close()

	writer := bufio.NewWriterSize(out, defaultBufioWriteSize)
	if err != nil {
		return 0, fmt.Errorf("failed to create bufio writer for %s: %w", output, err)
	}
	defer writer.Flush()

	// Preserve 9-bytes header for number of payloads.
	var head [9]byte
	_, err = writer.Write(head[:])
	if err != nil {
		return 0, fmt.Errorf("failed to write header for %s: %w", output, err)
	}

	// Need to flush buffer before encoding payloads.
	writer.Flush()

	enc := cbor.NewEncoder(writer)

	const logIntervalForPayloads = 1_000_000
	count := 0
	err = readPayloadFile(log, reader, func(rawPayload []byte) error {

		payload, err := ledger.DecodePayloadWithoutPrefix(rawPayload, false, payloadEncodingVersion)
		if err != nil {
			return fmt.Errorf("failed to decode payload 0x%x: %w", rawPayload, err)
		}

		k, err := payload.Key()
		if err != nil {
			return err
		}

		owner := k.KeyParts[0].Value

		include := false
		for _, address := range addresses {
			if bytes.Equal(owner, address[:]) {
				include = true
				break
			}
		}

		if include {
			err = enc.Encode(rawPayload)
			if err != nil {
				return fmt.Errorf("failed to encode payload: %w", err)
			}

			count++
			if count%logIntervalForPayloads == 0 {
				log.Info().Msgf("wrote %d payloads", count)
			}
		}

		return nil
	})
	if err != nil {
		return 0, err
	}

	log.Info().Msgf("wrote %d payloads", count)
	return count, nil
}

func parseAddresses(hexAddresses []string) ([]common.Address, error) {
	if len(hexAddresses) == 0 {
		return nil, fmt.Errorf("at least one address must be provided")
	}

	addresses := make([]common.Address, len(hexAddresses))
	for i, hexAddr := range hexAddresses {
		b, err := hex.DecodeString(strings.TrimSpace(hexAddr))
		if err != nil {
			return nil, fmt.Errorf("address is not hex encoded %s: %w", strings.TrimSpace(hexAddr), err)
		}

		addr, err := common.BytesToAddress(b)
		if err != nil {
			return nil, fmt.Errorf("cannot decode address %x", b)
		}

		addresses[i] = addr
	}

	return addresses, nil
}

func readPayloadFile(log zerolog.Logger, r io.Reader, processPayload func([]byte) error) error {
	dec := cbor.NewDecoder(r)

	var payloadCount int
	err := dec.Decode(&payloadCount)
	if err != nil {
		return err
	}

	log.Info().Msgf("Processing input file with %d payloads", payloadCount)

	const logIntervalForPayloads = 1_000_000
	count := 0
	for {
		var rawPayload []byte
		err = dec.Decode(&rawPayload)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		err = processPayload(rawPayload)
		if err != nil {
			return err
		}

		count++
		if count%logIntervalForPayloads == 0 {
			log.Info().Msgf("processed %d payloads", count)
		}
	}

	log.Info().Msgf("processed %d payloads", count)
	return nil
}
