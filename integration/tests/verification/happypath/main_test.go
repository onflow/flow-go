package happypath

import (
	"testing"

	"github.com/onflow/flow-go/integration/tests/verification"
	"github.com/stretchr/testify/suite"
)

func TestHappyPath(t *testing.T) {
	suite.Run(t, new(verification.VerificationTestSuite))
}
