package pathfinder_test

import (
	"crypto/sha256"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/crypto/hash"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/common/testutils"
)

// Test_KeyToPathV0 tests key to path for V0
func Test_KeyToPathV0(t *testing.T) {

	kp1 := testutils.KeyPartFixture(1, "key part 1")
	kp2 := testutils.KeyPartFixture(22, "key part 2")
	k := ledger.NewKey([]ledger.KeyPart{kp1, kp2})

	path, err := pathfinder.KeyToPath(k, 0)
	require.NoError(t, err)

	// compute expected value
	h := sha256.New()
	_, err = h.Write([]byte("key part 1"))
	require.NoError(t, err)
	_, err = h.Write([]byte("key part 2"))
	require.NoError(t, err)
	var expected ledger.Path
	copy(expected[:], h.Sum(nil))
	require.Equal(t, path, expected)
}

func Test_KeyToPathV1(t *testing.T) {

	kp1 := testutils.KeyPartFixture(1, "key part 1")
	kp2 := testutils.KeyPartFixture(22, "key part 2")
	k := ledger.NewKey([]ledger.KeyPart{kp1, kp2})

	path, err := pathfinder.KeyToPath(k, 1)
	require.NoError(t, err)

	// compute expected value
	hasher := hash.NewSHA3_256()
	_, err = hasher.Write([]byte("/1/key part 1/22/key part 2"))
	require.NoError(t, err)

	var expected ledger.Path
	copy(expected[:], hasher.SumHash())
	require.Equal(t, path, expected)
}
