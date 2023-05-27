package spam

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type GossipSubSpamMitigationIntegrationTestSuite struct {
	Suite
}

func TestGossipSubSpamMitigationSuite(t *testing.T) {
	suite.Run(t, new(GossipSubSpamMitigationIntegrationTestSuite))
}

func (s *GossipSubSpamMitigationIntegrationTestSuite) TestGossipSubWhisper() {
	require.Equal(s.T(), 2, 2)
}
