package emulator_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/sdk/emulator"
)

func TestRuntimeLogs(t *testing.T) {
	b, err := emulator.NewEmulatedBlockchain()
	require.NoError(t, err)

	script := []byte(`
		pub fun main() {
			log("elephant ears")
		}
	`)

	result, err := b.ExecuteScript(script)
	assert.NoError(t, err)
	assert.Equal(t, []string{`"elephant ears"`}, result.Logs)
}
