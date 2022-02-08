package systemchunk

import (
	"testing"

	"github.com/onflow/flow-go/integration/tests/verification"
	"github.com/stretchr/testify/suite"
)

func TestVerifySystemChunk(t *testing.T) {
	suite.Run(t, new(verification.VerifySystemChunkSuite))
}
