// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package cbor

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/fxamacker/cbor/v2"

	cborcodec "github.com/onflow/flow-go/model/encoding/cbor"
	"github.com/onflow/flow-go/network"
	_ "github.com/onflow/flow-go/utils/binstat"
)

// Codec represents a CBOR codec for our network.
type Codec struct {
}

// NewCodec creates a new CBOR codec.
func NewCodec() *Codec {
	c := &Codec{}
	return c
}

// NewEncoder creates a new CBOR encoder with the given underlying writer.
func (c *Codec) NewEncoder(w io.Writer) network.Encoder {
	enc := cborcodec.EncMode.NewEncoder(w)
	return &Encoder{enc: enc}
}

// NewDecoder creates a new CBOR decoder with the given underlying reader.
func (c *Codec) NewDecoder(r io.Reader) network.Decoder {
	dec := cbor.NewDecoder(r)
	return &Decoder{dec: dec}
}

const IS_COMPRESSED uint8 = 1
const IS_NOT_COMPRESSED uint8 = 0
const MINIMUM_BYTES_TO_COMPRESS = 8192

// Given a Golang interface 'v', return a []byte 'envelope'.
// Return an error if packing the envelope fails.
// NOTE: 'v' is the network message payload in unserialized form.
// NOTE: 'code' is the message type.
// NOTE: 'what' is the 'code' name for debugging / instrumentation.
// NOTE: 'envelope' contains 'code' & serialized / encoded 'v'.
// i.e.  1st byte is 'code' and remaining bytes are CBOR encoded 'v'.
func (c *Codec) Encode(v interface{}) ([]byte, error) {

	// encode the value
	what, code, err := v2envelopeCode(v)
	if err != nil {
		return nil, fmt.Errorf("could not determine envelope code: %w", err)
	}

	// NOTE: benchmarking shows that prepending the code and then using
	//       .NewEncoder() to .Encode() is the fastest.

	// encode / append the envelope code
	//bs1 := binstat.EnterTime(binstat.BinNet + ":wire<1(cbor)envelope2payload")
	var data bytes.Buffer
	data.WriteByte(IS_NOT_COMPRESSED)
	data.WriteByte(code)
	//binstat.LeaveVal(bs1, int64(data.Len()))

	// encode the payload
	//bs2 := binstat.EnterTime(fmt.Sprintf("%s%s%s:%d", binstat.BinNet, ":wire<2(cbor)", what, code)) // e.g. ~3net::wire<1(cbor)CodeEntityRequest:23
	encoder := cborcodec.EncMode.NewEncoder(&data)
	err = encoder.Encode(v)
	//binstat.LeaveVal(bs2, int64(data.Len()))
	if err != nil {
		return nil, fmt.Errorf("could not encode CBOR payload with envelope code %d AKA %s: %w", code, what, err) // e.g. 2, "CodeBlockProposal", <CBOR error>
	}

	if data.Len() < MINIMUM_BYTES_TO_COMPRESS {
		// come here if encoded message is considered too small to compress
		return data.Bytes(), nil
	}

	// compress the encoded payload
	//bs3 := binstat.EnterTime(fmt.Sprintf("%s%s%s:%d(size=saved)", binstat.BinNet, ":wire<2.5(cbor)compress:", what, code)) // e.g. ~3net::wire<1(cbor)CodeEntityRequest:23
	var data2gzip bytes.Buffer
	data2gzip.WriteByte(IS_COMPRESSED)
	zw, err := gzip.NewWriterLevel(&data2gzip, gzip.BestSpeed)
	if err != nil {
		return nil, fmt.Errorf("gzip.NewWriterLevel() failed: %w", err)
	}
	_, err = zw.Write(data.Bytes()[1:]) // all but first byte
	if err != nil {
		return nil, fmt.Errorf("gzip.NewWriterLevel().Write() failed: %w", err)
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("gzip.NewWriterLevel().Close() failed: %w", err)
	}
	//binstat.LeaveVal(bs3, int64(data.Len()-data2gzip.Len()))

	return data2gzip.Bytes(), nil
}

// Given a []byte 'envelope', eturn a Golang interface 'v'.
// Return an error if unpacking the envelope fails.
// NOTE: 'v' is the network message payload in unserialized form.
// NOTE: 'code' is the message type.
// NOTE: 'what' is the 'code' name for debugging / instrumentation.
// NOTE: 'envelope' contains 'code' & serialized / encoded 'v'.
// i.e.  1st byte is 'code' and remaining bytes are CBOR encoded 'v'.
func (c *Codec) Decode(maybeGzipBytes []byte) (interface{}, error) {

	var data []byte
	var err error

	if IS_COMPRESSED == maybeGzipBytes[0] {
		// uncompress the encoded payload
		//bs0 := binstat.EnterTime(binstat.BinNet + ":wire>2.5(cbor)uncompress")
		gzip2data := bytes.NewBuffer(maybeGzipBytes[1:])
		zr, err := gzip.NewReader(gzip2data)
		if err != nil {
			return nil, fmt.Errorf("gzip.NewReader() failed: %w", err)
		}
		data, err = ioutil.ReadAll(zr)
		if err != nil {
			return nil, fmt.Errorf("ioutil.ReadAll() failed: %w", err)
		}
		if err := zr.Close(); err != nil {
			return nil, fmt.Errorf("gzip.NewReader().Close() failed: %w", err)
		}
		//binstat.Leave(bs0)
	} else {
		data = maybeGzipBytes[1:]
	}

	// decode the envelope
	//bs1 := binstat.EnterTime(binstat.BinNet + ":wire>3(cbor)payload2envelope")
	code := data[0] // only first byte
	//binstat.LeaveVal(bs1, int64(len(data)))

	what, v, err := envelopeCode2v(code)
	if err != nil {
		return nil, fmt.Errorf("could not determine interface from code: %w", err)
	}

	// unmarshal the payload
	//bs2 := binstat.EnterTimeVal(fmt.Sprintf("%s%s%s:%d", binstat.BinNet, ":wire>4(cbor)", what, code), int64(len(data))) // e.g. ~3net:wire>4(cbor)CodeEntityRequest:23
	err = cbor.Unmarshal(data[1:], v) // all but first byte
	//binstat.Leave(bs2)
	if err != nil {
		return nil, fmt.Errorf("could not decode CBOR payload with envelope code %d AKA %s: %w", code, what, err) // e.g. 2, "CodeBlockProposal", <CBOR error>
	}

	return v, nil
}
