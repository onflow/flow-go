package emulator_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/sdk/emulator"
)

func TestRuntimeLogger(t *testing.T) {
	loggedMessages := make([]string, 0)

	b := emulator.NewEmulatedBlockchain(emulator.WithRuntimeLogger(
		func(msg string) {
			loggedMessages = append(loggedMessages, msg)
		},
	))

	script := []byte(`
		fun main() {
			log("elephant ears")
		}
	`)

	_, err := b.ExecuteScript(script)
	assert.Nil(t, err)
	assert.Equal(t, []string{`"elephant ears"`}, loggedMessages)
}
