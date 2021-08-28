package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go-sdk/crypto"

	"github.com/onflow/flow-go/cmd"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/epochs"
	"github.com/onflow/flow-go/module/local"
	"github.com/onflow/flow-go/utils/io"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestCreateQCContractClient(t *testing.T) {
	cmd := &cmd.NodeConfig{}

	// collection local
	identity := unittest.IdentityFixture()
	stakingKey, err := unittest.StakingKey()
	require.NoError(t, err)
	local, err := local.New(identity, stakingKey)
	require.NoError(t, err)

	// set required attributes
	cmd.Logger = zerolog.Nop()
	cmd.Me = local
	cmd.RootChainID = flow.Testnet

	machineAccountFileName := fmt.Sprintf(bootstrap.PathNodeMachineAccountInfoPriv, local.NodeID())

	t.Run("should return valid QC contract client", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(bootDir string) {

			accessAddress := "17.123.255.123:2353"
			nk, err := unittest.NetworkingKey()
			require.NoError(t, err)
			accessApiNodePubKey := hex.EncodeToString(nk.PublicKey().Encode())
			insecureAccessAPI := false
			// set BootstrapDir to temporary dir
			cmd.BaseConfig.BootstrapDir = bootDir

			// write machine account info
			infoPath := filepath.Join(bootDir, machineAccountFileName)
			writeNodeMachineAccountInfo(t, infoPath)
			require.FileExists(t, infoPath)

			client, err := createQCContractClient(cmd, accessAddress, accessApiNodePubKey, insecureAccessAPI)
			require.NoError(t, err)

			assert.IsType(t, &epochs.QCContractClient{}, client)

		})
	})

	t.Run("should return err if node machine account info is missing", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(bootDir string) {

			accessAddress := "17.123.255.123:2353"
			nk, err := unittest.NetworkingKey()
			require.NoError(t, err)
			accessApiNodePubKey := hex.EncodeToString(nk.PublicKey().Encode())
			insecureAccessAPI := false
			// set BootstrapDir to temporary dir
			cmd.BaseConfig.BootstrapDir = bootDir

			// make sure NodeMachineAccount file does not exist (sanity-check)
			require.NoFileExists(t, filepath.Join(bootDir, machineAccountFileName))

			_, err = createQCContractClient(cmd, accessAddress, accessApiNodePubKey, insecureAccessAPI)
			require.Error(t, err)
		})
	})

}

func writeNodeMachineAccountInfo(t *testing.T, path string) {
	nk, err := unittest.NetworkingKey()
	require.NoError(t, err)
	info := bootstrap.NodeMachineAccountInfo{
		Address:           "",
		EncodedPrivateKey: nk.Encode(),
		KeyIndex:          0,
		SigningAlgorithm:  nk.Algorithm(),
		HashAlgorithm:     crypto.SHA3_256,
	}
	bz, err := json.MarshalIndent(info, "", "  ")
	require.NoError(t, err)
	err = io.WriteFile(path, bz)
	require.NoError(t, err)
}
