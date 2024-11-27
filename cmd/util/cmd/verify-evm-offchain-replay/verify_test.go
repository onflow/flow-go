package verify

import (
	"testing"

	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
)

func TestTestnetFromRootToLatest(t *testing.T) {
	// verify Testnet51
	require.NoError(t, Verify(log.Logger, 211176670, 218215348,
		flow.Testnet, "/var/flow/data/protocol", "/var/flow/data/execution_data",
		"/var/flow/nov26_testnet_evm_state_gob", 100_000),
	)

	// verify Testnet52
	require.NoError(t, Verify(log.Logger, 218215349, 228901661,
		flow.Testnet, "/var/flow52/data/protocol", "/var/flow52/data/execution_data",
		"/var/flow/nov26_testnet_evm_state_gob", 100_000),
	)
}

func TestMainnetFromRootToLatest(t *testing.T) {
	// verify Mainnet25
	require.NoError(t, Verify(log.Logger, 85981135, 88226266,
		flow.Testnet, "/var/flow/data/protocol", "/var/flow/data/execution_data",
		"/var/flow/nov26_mainnet_evm_state_gob", 100_000),
	)

	// verify Mainnet26
	require.NoError(t, Verify(log.Logger, 88226267, 94800210,
		flow.Testnet, "/var/flow52/data/protocol", "/var/flow52/data/execution_data",
		"/var/flow/nov26_mainnet_evm_state_gob", 100_000),
	)
}
