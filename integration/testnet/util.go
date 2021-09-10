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
	"reflect"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/integration/client"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/utils/io"
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

// rootProtocolJsonWithoutAddresses strips out all node addresses from the root protocol json file specified as srcFile
// and creates the dstFile with the modified contents
func rootProtocolJsonWithoutAddresses(srcfile string, dstFile string) error {

	data, err := io.ReadFile(filepath.Join(srcfile))
	if err != nil {
		return err
	}

	var rootSnapshot inmem.EncodableSnapshot
	err = json.Unmarshal(data, &rootSnapshot)
	if err != nil {
		return err
	}

	removeAddress := func(ids flow.IdentityList) {

		for _, identity := range ids {
			identity.Address = ""
			//TODO: following doesn't work and Address is still retained
			idType := reflect.TypeOf(*identity)
			if addressField, found := idType.FieldByName("Address"); found {
				addressField.Tag = `json:"-"` // suppress json
			}
		}
	}
	removeAddress(rootSnapshot.Identities)
	if rootSnapshot.Epochs.Previous != nil {
		removeAddress(rootSnapshot.Epochs.Previous.InitialIdentities)
	}

	removeAddress(rootSnapshot.Epochs.Current.InitialIdentities)

	if rootSnapshot.Epochs.Next != nil {
		removeAddress(rootSnapshot.Epochs.Next.InitialIdentities)
	}

	for _, event := range rootSnapshot.LatestResult.ServiceEvents {
		switch event.Type {
		case flow.ServiceEventSetup:
			removeAddress(event.Event.(*flow.EpochSetup).Participants)
		}
	}

	return WriteJSON(dstFile, rootSnapshot)
}
