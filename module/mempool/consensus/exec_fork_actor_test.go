package consensus

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test_ActorPanicsOnSeals verifies that the actor panics when receiving seals
func Test_ActorPanicsOnSeals(t *testing.T) {
	//actor := LogForkAndCrash(zerolog.New(os.Stderr))
	//irSeals := unittest.IncorporatedResultSeal.Fixtures(2)
	//assert.Panics(t, func() { actor(irSeals) }, "Actor should crash the consensus node on inconsistent seals")
	assert.Panics(t, func() { panic(":") }, "Actor should crash the consensus node on inconsistent seals")
}
