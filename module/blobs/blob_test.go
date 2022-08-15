package blobs_test

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/module/blobs"
)

// TestBlobCIDLength tests that the CID length of a blob is equal to blobs.CidLength bytes.
// If this test fails, it means that the default CID length of a blob has changed, probably
// due to a change in the CID format used by our underlying dependencies.
func TestBlobCIDLength(t *testing.T) {
	data := make([]byte, 100)
	_, _ = rand.Read(data)

	assert.Equal(t, blobs.NewBlob(data).Cid().ByteLen(), blobs.CidLength)
}
