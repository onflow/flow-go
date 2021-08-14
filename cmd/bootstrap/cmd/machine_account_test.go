package cmd

import (
	"fmt"

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

func TestMachineAccountHappyPath(t *testing.T) {

	unittest.RunWithTempDir(t, func(bootDir string) {
		var machineAccountHappyPath = `^read machine account private key json` +
			`encoded public machine account key` +
			`wrote file %s/private-root-information/private-node-info_\S+/node-machine-account-info.priv.json`
		regex := regexp.MustCompile(fmt.Sprintf(machineAccountHappyPath, bootDir))

		// command flags
		flagOutdir = bootDir
		flagRole = "consensus"
		flagAddress = "189.123.123.42:3869"

		hook := zeroLoggerHook{logs: &strings.Builder{}}
		log = log.Hook(hook)

		// run keys command to generate all keys and bootstrap files
		keyCmdRun(nil, nil)
		hook.logs.Reset()

		// require log regex to match
		require.DirExists(t, filepath.Join(flagOutdir, bootstrap.DirnamePublicBootstrap))
		require.DirExists(t, filepath.Join(flagOutdir, bootstrap.DirPrivateRoot))

		// read in nodeID
		nodeIDPath := filepath.Join(flagOutdir, bootstrap.PathNodeID)
		require.FileExists(t, nodeIDPath)
		b, err := ioutils.ReadFile(nodeIDPath)
		require.NoError(t, err)
		nodeID := strings.TrimSpace(string(b))

		// make sure key file exists (sanity check)
		machineKeyFilePath := filepath.Join(flagOutdir, fmt.Sprintf(model.PathNodeMachineAccountPrivateKey, nodeID))
		require.FileExists(t, machineKeyFilePath)

		// sanity check if machine account info file exists
		machineInfoFilePath := filepath.Join(flagOutdir, fmt.Sprintf(model.PathNodeMachineAccountInfoPriv, nodeID))
		require.NoFileExists(t, machineInfoFilePath)

		// make sure regex matches and file was created
		machineAccountRun(nil, nil)
		require.Regexp(t, regex, hook.logs.String())
		require.FileExists(t, machineInfoFilePath)
	})
}

func TestMachineAccountInfoFileExists(t *testing.T) {

	unittest.RunWithTempDir(t, func(bootDir string) {
		var machineAccountInfoFileExistsRegex = `^node matching account info file already exists`
		regex := regexp.MustCompile(machineAccountInfoFileExistsRegex)

		// command flags
		flagOutdir = bootDir
		flagRole = "consensus"
		flagAddress = "189.123.123.42:3869"

		hook := zeroLoggerHook{logs: &strings.Builder{}}
		log = log.Hook(hook)

		// run keys command to generate all keys and bootstrap files
		keyCmdRun(nil, nil)
		hook.logs.Reset()

		// require log regex to match
		require.DirExists(t, filepath.Join(flagOutdir, bootstrap.DirnamePublicBootstrap))
		require.DirExists(t, filepath.Join(flagOutdir, bootstrap.DirPrivateRoot))

		// read in nodeID
		nodeIDPath := filepath.Join(flagOutdir, bootstrap.PathNodeID)
		require.FileExists(t, nodeIDPath)
		b, err := ioutils.ReadFile(nodeIDPath)
		require.NoError(t, err)
		nodeID := strings.TrimSpace(string(b))

		// make sure key file exists (sanity check)
		machineKeyFilePath := filepath.Join(flagOutdir, fmt.Sprintf(model.PathNodeMachineAccountPrivateKey, nodeID))
		require.FileExists(t, machineKeyFilePath)

		// sanity check if machine account info file exists
		machineInfoFilePath := filepath.Join(flagOutdir, fmt.Sprintf(model.PathNodeMachineAccountInfoPriv, nodeID))
		require.NoFileExists(t, machineInfoFilePath)

		// run machine account to create info file
		machineAccountRun(nil, nil)
		require.FileExists(t, machineInfoFilePath)
		hook.logs.Reset()

		// read in info file
		var machineAccountInfoBefore model.NodeMachineAccountInfo
		readJSON(machineInfoFilePath, &machineAccountInfoBefore)

		// run again and make sure info file was not changed
		machineAccountRun(nil, nil)
		require.Regexp(t, regex, hook.logs.String())

		var machineAccountInfoAfter model.NodeMachineAccountInfo
		readJSON(machineInfoFilePath, &machineAccountInfoAfter)

		assert.Equal(t, machineAccountInfoBefore, machineAccountInfoAfter)
	})
}
