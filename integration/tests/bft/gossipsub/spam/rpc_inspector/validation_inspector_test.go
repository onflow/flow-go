package rpc_inspector

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type GossipSubSpamMitigationIntegrationTestSuite struct {
	Suite
}

func TestGossipSubSpamMitigationSuite(t *testing.T) {
	suite.Run(t, new(GossipSubSpamMitigationIntegrationTestSuite))
}

func (s *GossipSubSpamMitigationIntegrationTestSuite) TestGossipSubWhisper() {

}
