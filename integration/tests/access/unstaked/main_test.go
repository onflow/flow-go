package unstaked

import (
	"testing"

	"github.com/onflow/flow-go/integration/tests/access"
	"github.com/stretchr/testify/suite"
)

func TestUnstakedAccessSuite(t *testing.T) {
	suite.Run(t, new(access.UnstakedAccessSuite))
}
