package flow_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestDeduplicate(t *testing.T) {
	require.Nil(t, flow.Deduplicate[*flow.Collection](nil))

	cols := unittest.CollectionListFixture(5)
	require.Equal(t, cols, flow.Deduplicate(cols))

	// create duplicates, and validate
	require.Equal(t, cols, flow.Deduplicate[*flow.Collection](append(cols, cols...)))

	// verify the original order should be preserved
	require.Equal(t, cols, flow.Deduplicate[*flow.Collection](
		append(cols, cols[3], cols[1], cols[4], cols[2], cols[0])))
}
