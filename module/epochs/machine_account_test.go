package epochs

import (
	"testing"

	"github.com/onflow/cadence"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/crypto"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestMachineAccountChecking tests that CheckMachineAccount captures critical
// misconfigurations of the machine account correctly.
//
// In these tests, local refers to the machine account from the local file,
// remote refers to the machine account from on-chain.
func TestMachineAccountChecking(t *testing.T) {
	conf := DefaultMachineAccountValidatorConfig()

	t.Run("consistent machine account", func(t *testing.T) {
		local, remote := unittest.MachineAccountFixture(t)
		err := CheckMachineAccountInfo(zerolog.Nop(), conf, flow.RoleConsensus, local, remote)
		require.NoError(t, err)
	})
	t.Run("inconsistent address", func(t *testing.T) {
		local, remote := unittest.MachineAccountFixture(t)
		remote.Address = unittest.RandomSDKAddressFixture()
		err := CheckMachineAccountInfo(zerolog.Nop(), conf, flow.RoleConsensus, local, remote)
		require.Error(t, err)
	})
	t.Run("inconsistent key", func(t *testing.T) {
		local, remote := unittest.MachineAccountFixture(t)
		randomKey := unittest.PrivateKeyFixture(crypto.ECDSAP256, unittest.DefaultSeedFixtureLength)
		remote.Keys[0].PublicKey = randomKey.PublicKey()
		err := CheckMachineAccountInfo(zerolog.Nop(), conf, flow.RoleConsensus, local, remote)
		require.Error(t, err)
	})
	t.Run("inconsistent hash algo", func(t *testing.T) {
		local, remote := unittest.MachineAccountFixture(t)
		remote.Keys[0].HashAlgo = sdkcrypto.SHA2_384
		err := CheckMachineAccountInfo(zerolog.Nop(), conf, flow.RoleConsensus, local, remote)
		require.Error(t, err)
	})
	t.Run("inconsistent sig algo", func(t *testing.T) {
		local, remote := unittest.MachineAccountFixture(t)
		remote.Keys[0].SigAlgo = sdkcrypto.ECDSA_secp256k1
		err := CheckMachineAccountInfo(zerolog.Nop(), conf, flow.RoleConsensus, local, remote)
		require.Error(t, err)
	})
	t.Run("account without keys", func(t *testing.T) {
		local, remote := unittest.MachineAccountFixture(t)
		remote.Keys = nil
		err := CheckMachineAccountInfo(zerolog.Nop(), conf, flow.RoleConsensus, local, remote)
		require.Error(t, err)
	})
	t.Run("account with insufficient keys", func(t *testing.T) {
		local, remote := unittest.MachineAccountFixture(t)
		// increment key index so it doesn't match remote account
		local.KeyIndex = local.KeyIndex + 1
		err := CheckMachineAccountInfo(zerolog.Nop(), conf, flow.RoleConsensus, local, remote)
		require.Error(t, err)
	})
	t.Run("invalid role", func(t *testing.T) {
		local, remote := unittest.MachineAccountFixture(t)
		for _, role := range flow.Roles() {
			// skip valid roles
			if role == flow.RoleCollection || role == flow.RoleConsensus {
				continue
			}

			err := CheckMachineAccountInfo(zerolog.Nop(), conf, role, local, remote)
			require.Error(t, err)
		}
	})

	t.Run("account with < hard minimum balance", func(t *testing.T) {
		t.Run("collection", func(t *testing.T) {
			local, remote := unittest.MachineAccountFixture(t)
			remote.Balance = uint64(defaultHardMinBalanceLN) - 1
			err := CheckMachineAccountInfo(zerolog.Nop(), conf, flow.RoleCollection, local, remote)
			require.Error(t, err)
		})
		t.Run("consensus", func(t *testing.T) {
			local, remote := unittest.MachineAccountFixture(t)
			remote.Balance = uint64(defaultHardMinBalanceSN) - 1
			err := CheckMachineAccountInfo(zerolog.Nop(), conf, flow.RoleConsensus, local, remote)
			require.Error(t, err)
		})
	})

	t.Run("disable balance checking", func(t *testing.T) {
		minBalance, err := cadence.NewUFix64("0.001")
		require.NoError(t, err)

		balanceDisabledConfig := DefaultMachineAccountValidatorConfig()
		WithoutBalanceChecks(&balanceDisabledConfig)

		t.Run("collection", func(t *testing.T) {
			local, remote := unittest.MachineAccountFixture(t)
			remote.Balance = uint64(minBalance)
			err := CheckMachineAccountInfo(zerolog.Nop(), balanceDisabledConfig, flow.RoleCollection, local, remote)
			require.NoError(t, err)
		})
		t.Run("consensus", func(t *testing.T) {
			local, remote := unittest.MachineAccountFixture(t)
			remote.Balance = uint64(minBalance)
			err := CheckMachineAccountInfo(zerolog.Nop(), balanceDisabledConfig, flow.RoleConsensus, local, remote)
			require.NoError(t, err)
		})
	})

	// should log a warning when balance below soft minimum balance (but not
	// below hard minimum balance)
	t.Run("account with < soft minimum balance", func(t *testing.T) {
		t.Run("collection", func(t *testing.T) {
			local, remote := unittest.MachineAccountFixture(t)
			remote.Balance = uint64(defaultSoftMinBalanceLN) - 1
			log, hook := unittest.HookedLogger()

			err := CheckMachineAccountInfo(log, conf, flow.RoleCollection, local, remote)
			assert.NoError(t, err)
			assert.Regexp(t, "machine account balance is below recommended balance", hook.Logs())
		})
		t.Run("consensus", func(t *testing.T) {
			local, remote := unittest.MachineAccountFixture(t)
			remote.Balance = uint64(defaultSoftMinBalanceSN) - 1
			log, hook := unittest.HookedLogger()

			err := CheckMachineAccountInfo(log, conf, flow.RoleConsensus, local, remote)
			assert.NoError(t, err)
			assert.Regexp(t, "machine account balance is below recommended balance", hook.Logs())
		})
	})

	// should log a warning when the local file deviates from defaults
	t.Run("local file deviates from defaults", func(t *testing.T) {
		t.Run("hash algo", func(t *testing.T) {
			local, remote := unittest.MachineAccountFixture(t)
			local.HashAlgorithm = sdkcrypto.SHA3_384     // non-standard hash algo
			remote.Keys[0].HashAlgo = sdkcrypto.SHA3_384 // consistent between local/remote
			log, hook := unittest.HookedLogger()

			err := CheckMachineAccountInfo(log, conf, flow.RoleConsensus, local, remote)
			assert.NoError(t, err)
			assert.Regexp(t, "non-standard hash algo", hook.Logs())
		})
		t.Run("sig algo", func(t *testing.T) {
			local, remote := unittest.MachineAccountFixture(t)

			// non-standard sig algo
			sk := unittest.PrivateKeyFixture(crypto.ECDSASecp256k1, unittest.DefaultSeedFixtureLength)
			local.EncodedPrivateKey = sk.Encode()
			local.SigningAlgorithm = sdkcrypto.ECDSA_secp256k1
			// consistent between local/remote
			remote.Keys[0].PublicKey = sk.PublicKey()
			remote.Keys[0].SigAlgo = sdkcrypto.ECDSA_secp256k1
			log, hook := unittest.HookedLogger()

			err := CheckMachineAccountInfo(log, conf, flow.RoleConsensus, local, remote)
			assert.NoError(t, err)
			assert.Regexp(t, "non-standard signing algo", hook.Logs())
		})
		t.Run("key index", func(t *testing.T) {
			local, remote := unittest.MachineAccountFixture(t)
			local.KeyIndex = 1                                // non-standard key index
			remote.Keys = append(remote.Keys, remote.Keys[0]) // key with index exists on remote
			remote.Keys[1].Index = 1
			log, hook := unittest.HookedLogger()

			err := CheckMachineAccountInfo(log, conf, flow.RoleConsensus, local, remote)
			assert.NoError(t, err)
			assert.Regexp(t, "non-standard key index", hook.Logs())
		})
	})
}

// TestBackoff tests the backoff config behaves as expected. In particular, once
// we reach the cap duration, all future backoffs should be equal to the cap duration.
func TestMachineAccountValidatorBackoff_Overflow(t *testing.T) {

	backoff := checkMachineAccountRetryBackoff()

	// once the backoff reaches the maximum, it should remain in [(1-jitter)*max,(1+jitter*max)]
	max := checkMachineAccountRetryMax + checkMachineAccountRetryMax*(checkMachineAccountRetryJitterPct+1)/100
	min := checkMachineAccountRetryMax - checkMachineAccountRetryMax*(checkMachineAccountRetryJitterPct+1)/100

	lastWait, stop := backoff.Next()
	assert.False(t, stop)
	for i := 0; i < 100; i++ {
		wait, stop := backoff.Next()
		assert.False(t, stop)
		// the backoff value should either:
		// * strictly increase, or
		// * be within range of max duration + jitter
		if wait < lastWait {
			assert.Less(t, min, wait)
			assert.Less(t, wait, max)
		}
		lastWait = wait
	}
}
