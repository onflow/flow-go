package testnet

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"os/user"
	"path/filepath"
	"regexp"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/integration/client"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
)

// healthcheckAccessGRPC returns a Docker healthcheck function that pings the Access node GRPC
// service exposed at the given port.
func healthcheckAccessGRPC(apiPort string) func() error {
	return func() error {
		fmt.Println("healthchecking...")
		c, err := client.NewAccessClient(fmt.Sprintf(":%s", apiPort))
		if err != nil {
			return err
		}

		return c.Ping(context.Background())
	}
}

// healthcheckExecutionGRPC returns a Docker healthcheck function that pings the Execution node GRPC
// service exposed at the given port.
func healthcheckExecutionGRPC(apiPort string) func() error {
	return func() error {
		fmt.Println("healthchecking...")
		c, err := client.NewExecutionClient(fmt.Sprintf(":%s", apiPort))
		if err != nil {
			return err
		}

		return c.Ping(context.Background())
	}
}

// currentUser returns a uid:gid Unix user identifier string for the current
// user. This is used to run node containers under the same user to avoid
// permission conflicts on files mounted from the host.
func currentUser() string {
	cur, _ := user.Current()
	return fmt.Sprintf("%s:%s", cur.Uid, cur.Gid)
}

func toParticipants(confs []ContainerConfig) flow.IdentityList {
	il := make(flow.IdentityList, 0, len(confs))
	for _, conf := range confs {
		il = append(il, conf.Identity())
	}
	return il
}

func toNodeInfos(confs []ContainerConfig) []bootstrap.NodeInfo {
	infos := make([]bootstrap.NodeInfo, 0, len(confs))
	for _, conf := range confs {
		infos = append(infos, conf.NodeInfo)
	}
	return infos
}

// filterContainerConfigs filters a list of container configs.
func filterContainerConfigs(confs []ContainerConfig, shouldInclude func(ContainerConfig) bool) []ContainerConfig {
	filtered := make([]ContainerConfig, 0, len(confs))
	for _, conf := range confs {
		if !shouldInclude(conf) {
			continue
		}
		filtered = append(filtered, conf)
	}
	return filtered
}

func getSeed() ([]byte, error) {
	seedLen := int(math.Max(crypto.SeedMinLenDKG, crypto.KeyGenSeedMinLenBLSBLS12381))
	seed := make([]byte, seedLen)
	n, err := rand.Read(seed)
	if err != nil || n != seedLen {
		return nil, err
	}
	return seed, nil
}

func WriteJSON(path string, data interface{}) error {
	err := os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		return err
	}

	marshaled, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(path, marshaled, 0644)
	return err
}

func StripAddressesFromRootProtocolJson(srcfile string, dstFile string) error {

	fileData, err := ioutil.ReadFile(srcfile)
	if err != nil {
		return err
	}
	fileString := string(fileData)
	m1 := regexp.MustCompile(`.*"Address".*:.*".*".*\n`)
	newString := m1.ReplaceAllString(fileString, "")
	newFileData := []byte(newString)

	err = ioutil.WriteFile(dstFile, newFileData, 0o600)
	if err != nil {
		return err
	}
	fmt.Println(newString)

	return nil
}
