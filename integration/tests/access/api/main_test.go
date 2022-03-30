package api

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/integration/tests/access"
)

func TestAccessSuite(t *testing.T) {
	suite.Run(t, new(access.AccessSuite))
}
