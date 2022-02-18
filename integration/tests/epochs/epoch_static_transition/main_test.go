package epoch_static_transition

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/integration/tests/epochs"
)

func TestStaticTransition(t *testing.T) {
	suite.Run(t, new(epochs.StaticEpochTransitionSuite))
}
