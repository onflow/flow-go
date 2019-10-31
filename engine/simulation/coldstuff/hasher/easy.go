// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package hasher

import (
	"crypto/sha256"
	"encoding/binary"

	"github.com/dapperlabs/flow-go/model/coldstuff"
)

// Easy implements simple hashing of the coldstuff header model.
type Easy struct {
}

// NewEasy creates a new easy hasher.
func NewEasy() *Easy {
	return &Easy{}
}

// BlockHash returns the hash of a block by header.
func (e *Easy) BlockHash(header *coldstuff.BlockHeader) []byte {
	data := make([]byte, 24, 88)
	binary.LittleEndian.PutUint64(data[0:8], header.Height)
	binary.LittleEndian.PutUint64(data[8:16], header.Nonce)
	binary.LittleEndian.PutUint64(data[16:24], uint64(header.Timestamp.UnixNano()))
	data = append(data, header.Parent...)
	data = append(data, header.Payload...)
	hash := sha256.Sum256(data)
	return hash[:]
}
