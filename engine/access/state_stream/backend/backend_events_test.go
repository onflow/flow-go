package backend

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type BackendEventsSuite struct {
	suite.Suite
}

func TestBackendEventsSuite(t *testing.T) {
	suite.Run(t, new(BackendEventsSuite))
}
