package state_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/evm/emulator/state"
)

func TestCodeContainer(t *testing.T) {
	code := []byte("some code")

	// test construction
	cc := state.NewCodeContainer(code)
	require.Equal(t, uint64(1), cc.RefCount())
	require.Equal(t, code, cc.Code())

	// test increment
	cc.IncRefCount()
	require.Equal(t, uint64(2), cc.RefCount())

	// test encoding
	encoded := cc.Encoded()
	cc, err := state.CodeContainerFromEncoded(encoded)
	require.NoError(t, err)
	require.Equal(t, uint64(2), cc.RefCount())
	require.Equal(t, code, cc.Code())

	// test decrement
	require.Equal(t, false, cc.DecRefCount())
	require.Equal(t, uint64(1), cc.RefCount())
	require.Equal(t, true, cc.DecRefCount())
	require.Equal(t, uint64(0), cc.RefCount())
}
