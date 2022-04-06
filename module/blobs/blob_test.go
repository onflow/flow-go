package blobs_test

import (
	"crypto/rand"
	"testing"

	"github.com/onflow/flow-go/module/blobs"
	"github.com/stretchr/testify/assert"
)

func TestBlobCIDLength(t *testing.T) {
	data := make([]byte, 100)
	rand.Read(data)

	assert.Equal(t, blobs.NewBlob(data).Cid().ByteLen(), blobs.CidLength)
}
