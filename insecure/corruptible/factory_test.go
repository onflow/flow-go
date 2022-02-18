package corruptible_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/insecure/corruptible"
	"github.com/onflow/flow-go/network/codec/cbor"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestRegisterAdapter(t *testing.T) {
	f := corruptible.NewCorruptibleConduitFactory(unittest.IdentifierFixture(), cbor.NewCodec())

	adapter := &mocknetwork.Adapter{}

	// registering adapter must go successful
	require.NoError(t, f.RegisterAdapter(adapter))

	// second attempt on registering adapter must fail
	require.Error(t, f.RegisterAdapter(adapter))
}
