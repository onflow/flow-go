package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/dapperlabs/flow-go/model/bootstrap"
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
	//bootcmd.WriteText(filepath.Join(bootdir, fmt.Sprintf(bootstrap.PathNodeId, nodeID)), []byte(nodeID)
	randomBeaconPath := filepath.Join(bootdir, fmt.Sprintf(bootstrap.PathRandomBeaconPriv, nodeID))
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
	err = unwrapFile(bootdir, nodeID)
	if err != nil {
		t.Fatalf("Error unwrapping response: %s", err)
	}
}
