package p2p

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type HierarchicalTranslatorTestSuite struct {
	suite.Suite
}

func TestHierarchicalTranslator(t *testing.T) {
	suite.Run(t, new(HierarchicalTranslatorTestSuite))
}
