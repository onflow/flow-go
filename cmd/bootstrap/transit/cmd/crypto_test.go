package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/bootstrap"
)

const nodeID string = "0000000000000000000000000000000000000000000000000000000000000001"

func TestEndToEnd(t *testing.T) {

	// Create a temp directory to work as "bootstrap"
	bootdir, err := ioutil.TempDir("", "bootstrap.*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(bootdir)

	t.Logf("Created dir %s", bootdir)

	// Create test files
	//bootcmd.WriteText(filepath.Join(bootdir, bootstrap.PathNodeId), []byte(nodeID)
	randomBeaconPath := filepath.Join(bootdir, fmt.Sprintf(bootstrap.PathRandomBeaconPriv, nodeID))
	err = os.MkdirAll(filepath.Dir(randomBeaconPath), 0755)
	if err != nil {
		t.Fatalf("Failed to write dir for random beacon file: %s", err)
	}
	err = ioutil.WriteFile(randomBeaconPath, []byte("test data"), 0644)
	if err != nil {
		t.Fatalf("Failed to write random beacon file: %s", err)
	}

	// Client:
	// create the transit keys
	err = generateKeys(bootdir, nodeID)
	if err != nil {
		t.Fatalf("Error generating keys: %s", err)
	}

	// Dapper:
	// Wrap a file with transit keys
	err = wrapFile(bootdir, nodeID)
	if err != nil {
		t.Fatalf("Error wrapping files: %s", err)
	}

	// Client:
	// Unwrap files
	err = unWrapFile(bootdir, nodeID)
	if err != nil {
		t.Fatalf("Error unwrapping response: %s", err)
	}
}

func TestSha256(t *testing.T) {
	tmpFile, err := ioutil.TempFile(os.TempDir(), "prefix-")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	text := []byte("write some text to file")
	_, err = tmpFile.Write(text)
	assert.NoError(t, err)

	assert.NoError(t, tmpFile.Close())

	hash, err := getFileSHA256(tmpFile.Name())
	assert.NoError(t, err)
	assert.Equal(t, "876a3eab5fe740cb864a3d62869b0eefd6fbc34ec331c3064a6ffac0f9485a88", hash)
}
