package flow

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/encoding/rlp"
)

func TestCachedID_RLP(t *testing.T) {
	type withoutEmbeddedCachedID struct {
		A string
		B uint64
	}
	type withEmbeddedCachedID struct {
		A  string
		B  uint64
		id cachedID
	}

	a := withoutEmbeddedCachedID{
		A: "a",
		B: 1,
	}
	b := withEmbeddedCachedID{
		A: "a",
		B: 1,
	}

	assert.Equal(t, rlp.NewMarshaler().MustMarshal(a), rlp.NewMarshaler().MustMarshal(b))

	b.id = cachedID(MakeID(b))
	assert.Equal(t, rlp.NewMarshaler().MustMarshal(a), rlp.NewMarshaler().MustMarshal(b))
}
