package cmd

import (
	"fmt"
	"github.com/onflow/flow-go/cmd/bootstrap/utils"
	"os"

	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/bootstrap"
	model "github.com/onflow/flow-go/model/bootstrap"
	ioutils "github.com/onflow/flow-go/utils/io"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestMachineAccountKeyFileExists(t *testing.T) {

	unittest.RunWithTempDir(t, func(bootDir string) {
		var keyFileExistsRegex = regexp.MustCompile(`^machine account private key already exists`)

		// command flags
		FlagOutdir = bootDir
		flagRole = "consensus"
		flagAddress = "189.123.123.42:3869"

		hook := zeroLoggerHook{logs: &strings.Builder{}}
		log = log.Hook(hook)

		// run keys command to generate all keys and bootstrap files
		keyCmdRun(nil, nil)
		hook.logs.Reset()

		require.DirExists(t, filepath.Join(FlagOutdir, bootstrap.DirnamePublicBootstrap))
		require.DirExists(t, filepath.Join(FlagOutdir, bootstrap.DirPrivateRoot))

		nodeIDPath := filepath.Join(FlagOutdir, bootstrap.PathNodeID)
		require.FileExists(t, nodeIDPath)
		b, err := ioutils.ReadFile(nodeIDPath)
		require.NoError(t, err)
		nodeID := strings.TrimSpace(string(b))

		// make sure file exists
		machineKeyFilePath := filepath.Join(FlagOutdir, fmt.Sprintf(model.PathNodeMachineAccountPrivateKey, nodeID))
		require.FileExists(t, machineKeyFilePath)

		// read file priv key file before command
		var machineAccountPrivBefore model.NodeMachineAccountKey
		utils.ReadJSON(machineKeyFilePath, &machineAccountPrivBefore)

		// run command with flags
		machineAccountKeyRun(nil, nil)

		// make sure regex matches
		require.Regexp(t, keyFileExistsRegex, hook.logs.String())

		// read machine account key file again
		var machineAccountPrivAfter model.NodeMachineAccountKey
		utils.ReadJSON(machineKeyFilePath, &machineAccountPrivAfter)

		// check if key was modified
		assert.Equal(t, machineAccountPrivBefore, machineAccountPrivAfter)
	})
}

func TestMachineAccountKeyFileCreated(t *testing.T) {

	unittest.RunWithTempDir(t, func(bootDir string) {
		var keyFileCreatedRegex = `^generated machine account private key` +
			`encoded public machine account key` +
			`wrote file %s/private-root-information/private-node-info_\S+/node-machine-account-key.priv.json` +
			`machine account public key: \S+`
		regex := regexp.MustCompile(fmt.Sprintf(keyFileCreatedRegex, bootDir))

		// command flags
		FlagOutdir = bootDir
		flagRole = "consensus"
		flagAddress = "189.123.123.42:3869"

		hook := zeroLoggerHook{logs: &strings.Builder{}}
		log = log.Hook(hook)

		// run keys command to generate all keys and bootstrap files
		keyCmdRun(nil, nil)
		hook.logs.Reset()

		// require log regex to match
		require.DirExists(t, filepath.Join(FlagOutdir, bootstrap.DirnamePublicBootstrap))
		require.DirExists(t, filepath.Join(FlagOutdir, bootstrap.DirPrivateRoot))

		// read in node ID
		nodeIDPath := filepath.Join(FlagOutdir, bootstrap.PathNodeID)
		require.FileExists(t, nodeIDPath)
		b, err := ioutils.ReadFile(nodeIDPath)
		require.NoError(t, err)
		nodeID := strings.TrimSpace(string(b))

		// delete machine account key file
		machineKeyFilePath := filepath.Join(FlagOutdir, fmt.Sprintf(model.PathNodeMachineAccountPrivateKey, nodeID))
		err = os.Remove(machineKeyFilePath)
		require.NoError(t, err)

		// confirm file was removed
		require.NoFileExists(t, machineKeyFilePath)

		// run command with flags
		machineAccountKeyRun(nil, nil)

		// make sure regex matches
		assert.Regexp(t, regex, hook.logs.String())

		// make sure file exists (regex checks this too)
		require.FileExists(t, machineKeyFilePath)
	})
}
