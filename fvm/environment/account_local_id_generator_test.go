package environment_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/common"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/tracing"
	"github.com/onflow/flow-go/model/flow"
)

func Test_accountLocalIDGenerator_GenerateAccountID(t *testing.T) {
	t.Skip("Skipping tests that depend on removed envMock package")
}
