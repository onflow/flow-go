package epochs

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestMachineAccountChecking tests that CheckMachineAccount captures critical
// misconfigurations of the machine account correctly.
//
// In these tests, local refers to the machine account from the local file,
// remote refers to the machine account from on-chain.
func TestMachineAccountChecking(t *testing.T) {
	t.Run("consistent machine account", func(t *testing.T) {
		local, remote := unittest.MachineAccountFixture(t)
		err := CheckMachineAccountInfo(zerolog.Nop(), flow.RoleConsensus, local, remote)
		require.NoError(t, err)
	})
	t.Run("inconsistent address", func(t *testing.T) {
		local, remote := unittest.MachineAccountFixture(t)
		remote.Address = unittest.RandomSDKAddressFixture()
		err := CheckMachineAccountInfo(zerolog.Nop(), flow.RoleConsensus, local, remote)
		require.Error(t, err)
	})
	t.Run("inconsistent key", func(t *testing.T) {
		local, remote := unittest.MachineAccountFixture(t)
		randomKey := unittest.PrivateKeyFixture(crypto.ECDSAP256)
		remote.Keys[0].PublicKey = randomKey.PublicKey()
		err := CheckMachineAccountInfo(zerolog.Nop(), flow.RoleConsensus, local, remote)
		require.Error(t, err)
	})
	t.Run("inconsistent hash algo", func(t *testing.T) {
		local, remote := unittest.MachineAccountFixture(t)
		remote.Keys[0].HashAlgo = sdkcrypto.SHA2_384
		err := CheckMachineAccountInfo(zerolog.Nop(), flow.RoleConsensus, local, remote)
		require.Error(t, err)
	})
	t.Run("inconsistent sig algo", func(t *testing.T) {
		local, remote := unittest.MachineAccountFixture(t)
		remote.Keys[0].SigAlgo = sdkcrypto.ECDSA_secp256k1
		err := CheckMachineAccountInfo(zerolog.Nop(), flow.RoleConsensus, local, remote)
		require.Error(t, err)
	})
	t.Run("account without keys", func(t *testing.T) {
		local, remote := unittest.MachineAccountFixture(t)
		remote.Keys = nil
		err := CheckMachineAccountInfo(zerolog.Nop(), flow.RoleConsensus, local, remote)
		require.Error(t, err)
	})
	t.Run("account with insufficient keys", func(t *testing.T) {
		local, remote := unittest.MachineAccountFixture(t)
		// increment key index so it doesn't match remote account
		local.KeyIndex = local.KeyIndex + 1
		err := CheckMachineAccountInfo(zerolog.Nop(), flow.RoleConsensus, local, remote)
		require.Error(t, err)
	})
	t.Run("account with < minimum balance", func(t *testing.T) {
		t.Run("collection", func(t *testing.T) {
			local, remote := unittest.MachineAccountFixture(t)
			remote.Balance = uint64(HardMinBalanceLN) - 1
			err := CheckMachineAccountInfo(zerolog.Nop(), flow.RoleCollection, local, remote)
			require.Error(t, err)
		})
		t.Run("consensus", func(t *testing.T) {
			local, remote := unittest.MachineAccountFixture(t)
			remote.Balance = uint64(HardMinBalanceSN) - 1
			err := CheckMachineAccountInfo(zerolog.Nop(), flow.RoleConsensus, local, remote)
			require.Error(t, err)
		})
	})
	t.Run("invalid role", func(t *testing.T) {
		local, remote := unittest.MachineAccountFixture(t)
		for _, role := range flow.Roles() {
			// skip valid roles
			if role == flow.RoleCollection || role == flow.RoleConsensus {
				continue
			}

			err := CheckMachineAccountInfo(zerolog.Nop(), role, local, remote)
			require.Error(t, err)
		}
	})
}
