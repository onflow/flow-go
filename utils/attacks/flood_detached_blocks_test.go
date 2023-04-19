package attacks

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack/v4"

	"github.com/onflow/flow-go/utils/unittest"
)

func TestBlockSize(t *testing.T) {
	attack := createAttackProposal(unittest.BlockHeaderFixture())
	bz, err := msgpack.Marshal(attack)
	require.NoError(t, err)
	fmt.Println(len(bz))
}
