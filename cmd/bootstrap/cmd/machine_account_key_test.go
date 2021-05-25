package cmd

import (
	"fmt"
	"os"

	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/bootstrap"
	model "github.com/onflow/flow-go/model/bootstrap"
	ioutils "github.com/onflow/flow-go/utils/io"
)

func TestMachineAccountKeyFileExists(t *testing.T) {
	var keyFileExistsRegex = `^machine account private key already exists`

	dirName := strconv.FormatInt(time.Now().UnixNano(), 10)
	regex := regexp.MustCompile(keyFileExistsRegex)

	// command flags
	flagOutdir = fmt.Sprintf("/tmp/%s", dirName)
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

	nodeIDPath := filepath.Join(flagOutdir, bootstrap.PathNodeID)
	require.FileExists(t, nodeIDPath)
	b, err := ioutils.ReadFile(nodeIDPath)
	require.NoError(t, err)
	nodeID := strings.TrimSpace(string(b))

	// make sure file exists
	machineKeyFilePath := filepath.Join(flagOutdir, fmt.Sprintf(model.PathNodeMachineAccountPrivateKey, nodeID))
	require.FileExists(t, machineKeyFilePath)

	// read file priv key file before command
	var machineAccountPrivBefore model.NodeMachineAccountPriv
	readJSON(machineKeyFilePath, &machineAccountPrivBefore)

	// run command with flags
	machineAccountKeyRun(nil, nil)

	// make sure regex matches
	require.Regexp(t, regex, hook.logs.String())

	// read machine account key file again
	var machineAccountPrivAfter model.NodeMachineAccountPriv
	readJSON(machineKeyFilePath, &machineAccountPrivAfter)

	// check if key was modified
	assert.Equal(t, machineAccountPrivBefore, machineAccountPrivAfter)
}

func TestMachineAccountKeyFileCreated(t *testing.T) {
	var keyFileCreatedRegex = `^generated machine account private key` +
		`encoded public machine account key` +
		`wrote file /tmp/%s/private-root-information/private-node-info_\S+/node-machine-account-key.priv.json` +
		`wrote machine account private key`

	dirName := strconv.FormatInt(time.Now().UnixNano(), 10)
	regex := regexp.MustCompile(fmt.Sprintf(keyFileCreatedRegex, dirName))

	// command flags
	flagOutdir = fmt.Sprintf("/tmp/%s", dirName)
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

	// read in node ID
	nodeIDPath := filepath.Join(flagOutdir, bootstrap.PathNodeID)
	require.FileExists(t, nodeIDPath)
	b, err := ioutils.ReadFile(nodeIDPath)
	require.NoError(t, err)
	nodeID := strings.TrimSpace(string(b))

	// delete machine account key file
	machineKeyFilePath := filepath.Join(flagOutdir, fmt.Sprintf(model.PathNodeMachineAccountPrivateKey, nodeID))
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
}
