package checkpoint

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFindCheckpointsToRemove(t *testing.T) {
	require.Equal(t, []int{1}, findCheckpointsToRemove([]int{1, 2, 3, 4, 5, 6}, 5))
	require.Equal(t, []int(nil), findCheckpointsToRemove([]int{1, 2, 3, 4, 5}, 5))
	require.Equal(t, []int(nil), findCheckpointsToRemove([]int{1, 2, 3, 4}, 5))
	require.Equal(t, []int(nil), findCheckpointsToRemove([]int{}, 5))
}
