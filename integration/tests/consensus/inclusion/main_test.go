package inclusion

import (
	"testing"

	"github.com/onflow/flow-go/integration/tests/consensus"
	"github.com/stretchr/testify/suite"
)

func TestCollectionGuaranteeInclusion(t *testing.T) {
	suite.Run(t, new(consensus.InclusionSuite))
}
