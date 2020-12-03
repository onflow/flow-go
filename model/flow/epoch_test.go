package flow_test

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/utils/unittest"
)

// test that the FirstView field does not impact the ID computation.
func TestEpochSetup_ID(t *testing.T) {
	setup := unittest.EpochSetupFixture()
	id := setup.ID()
	setup.FirstView = rand.Uint64()
	assert.Equal(t, id, setup.ID())
}
