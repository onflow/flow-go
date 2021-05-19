package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateKeys(t *testing.T) {

	t.Run("should generate all keys", func(t *testing.T) {
		net, stake, machine, err := generateKeys(true, true, true)
		require.NoError(t, err)
		assert.NotNil(t, net)
		assert.NotNil(t, stake)
		assert.NotNil(t, machine)
	})

	t.Run("should generate network key only", func(t *testing.T) {
		net, stake, machine, err := generateKeys(true, false, false)
		require.NoError(t, err)
		assert.NotNil(t, net)
		assert.Nil(t, stake)
		assert.Nil(t, machine)
	})

	t.Run("should generate staking key only", func(t *testing.T) {
		net, stake, machine, err := generateKeys(false, true, false)
		require.NoError(t, err)
		assert.Nil(t, net)
		assert.NotNil(t, stake)
		assert.Nil(t, machine)
	})

	t.Run("should generate machine key only", func(t *testing.T) {
		net, stake, machine, err := generateKeys(false, false, true)
		require.NoError(t, err)
		assert.Nil(t, net)
		assert.Nil(t, stake)
		assert.NotNil(t, machine)
	})
}

var happyPathRegex = `^will generate networking key` +
	`generated networking key` +
	`will generate staking key` +
	`generated staking key` +
	`assembling node information` +
	`encoded public staking and network keys` +
	`wrote file /tmp/%s/public-root-information/node-id` +
	`wrote file /tmp/%s/private-root-information/private-node-info_\S+/node-info.priv.json` +
	`wrote file /tmp/%s/public-root-information/node-info.pub.\S+.json`

func TestHappyPath(t *testing.T) {
	dirName := strconv.FormatInt(time.Now().UnixNano(), 10)
	regex := regexp.MustCompile(fmt.Sprintf(happyPathRegex, dirName, dirName, dirName))
	flagOutdir = "/tmp/" + dirName
	flagRole = "consensus"
	flagAddress = "189.123.123.42:3869"
	hook := zeroLoggerHook{
		logs: &strings.Builder{},
	}
	log = log.Hook(hook)
	keyCmdRun(nil, nil)
	require.Regexp(t, regex, hook.logs.String())
	require.DirExists(t, flagOutdir+"/public-root-information")
	require.FileExists(t, flagOutdir+"/public-root-information/node-id", "node-id file not created")
	require.DirExists(t, flagOutdir+"/private-root-information")
}

func TestInvalidAddress(t *testing.T) {

	invalidAddresses := []string{
		// address with http
		"http://123.34.2.42:3469",
		// address with no port
		"123.34.2.42",
		// address with http and no port
		"http://123.34.2.42",
	}

	for _, a := range invalidAddresses {

		// Run the test in a subprocess since it does as os.Exit(1) when the args are invalid
		cmd := exec.Command(os.Args[0], "-test.run=TestInvalidAddressSubprocess")
		cmd.Env = append(os.Environ(), "FLAG=1")

		// capture the subprocess stdout and stderr
		errMsg, err := cmd.CombinedOutput()

		// check that the process indeed failed
		assert.Error(t, err)

		// check the error message
		assert.Truef(t, strings.Contains(string(errMsg), "invalid address format"),
			"validation failed for address %s", a)
	}
}

// TestInvalidAddressSubprocess is called from a new process when checking for invalid args. It causes a system.Exit
func TestInvalidAddressSubprocess(t *testing.T) {
	// Run the crashing code when FLAG is set
	if os.Getenv("FLAG") == "1" {
		flagOutdir = "/tmp"
		flagRole = "consensus"
		flagAddress = os.Getenv("address")
		keyCmdRun(nil, nil)
		return
	}
}

type zeroLoggerHook struct {
	logs *strings.Builder
}

func (h zeroLoggerHook) Run(_ *zerolog.Event, _ zerolog.Level, msg string) {
	h.logs.WriteString(msg)
}
