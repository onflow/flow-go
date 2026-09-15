package cmd

import (
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
)

// defaultExecutionConfig returns an [ExecutionConfig] populated with the flag defaults, which is a
// valid configuration as far as `ValidateFlags` is concerned.
func defaultExecutionConfig(t *testing.T) *ExecutionConfig {
	conf := &ExecutionConfig{}
	conf.SetupFlags(pflag.NewFlagSet("test", pflag.ContinueOnError))
	require.NoError(t, conf.ValidateFlags(), "sanity check: flag defaults must be a valid config")
	return conf
}

// TestValidateFlags_Payloadless verifies that the payloadless ledger can only be enabled together
// with the storehouse. The payloadless ledger does not retain register values, so without the
// storehouse there would be no register value source at execution and snapshot-read time.
func TestValidateFlags_Payloadless(t *testing.T) {
	t.Run("payloadless without storehouse is rejected", func(t *testing.T) {
		conf := defaultExecutionConfig(t)
		conf.payloadless = true
		conf.enableStorehouse = false

		err := conf.ValidateFlags()
		require.Error(t, err)
		require.Contains(t, err.Error(), "--payloadless requires --enable-storehouse")
	})

	t.Run("payloadless with storehouse is accepted", func(t *testing.T) {
		conf := defaultExecutionConfig(t)
		conf.payloadless = true
		conf.enableStorehouse = true

		require.NoError(t, conf.ValidateFlags())
	})
}
