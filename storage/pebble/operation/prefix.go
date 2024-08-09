package operation

import (
	"encoding/binary"
	"fmt"

	"github.com/ipfs/go-cid"

	"github.com/onflow/flow-go/model/flow"
)

const (
	//lint:ignore U1000 Ignore unused variable warning
	// job queue consumers and producers
	codeJobConsumerProcessed = 70

	// legacy codes (should be cleaned up)
	codeChunkDataPack = 100
)

func makePrefix(code byte, keys ...interface{}) []byte {
	prefix := make([]byte, 1)
	prefix[0] = code
	for _, key := range keys {
		prefix = append(prefix, b(key)...)
	}
	return prefix
}

func b(v interface{}) []byte {
	switch i := v.(type) {
	case uint8:
		return []byte{i}
	case uint32:
		b := make([]byte, 4)
		binary.BigEndian.PutUint32(b, i)
		return b
	case uint64:
		b := make([]byte, 8)
		binary.BigEndian.PutUint64(b, i)
		return b
	case string:
		return []byte(i)
	case flow.Role:
		return []byte{byte(i)}
	case flow.Identifier:
		return i[:]
	case flow.ChainID:
		return []byte(i)
	case cid.Cid:
		return i.Bytes()
	default:
		panic(fmt.Sprintf("unsupported type to convert (%T)", v))
	}
}
