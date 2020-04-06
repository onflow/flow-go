package testnet

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os/user"

	"github.com/dapperlabs/flow-go/integration/client"
	"github.com/dapperlabs/flow-go/model/bootstrap"
	"github.com/dapperlabs/flow-go/model/flow"
)

// healthcheckGRPC returns a Docker healthcheck function that pings the GRPC
// service exposed at the given port.
func healthcheckGRPC(apiPort string) func() error {
	return func() error {
		fmt.Println("healthchecking...")
		c, err := client.New(fmt.Sprintf(":%s", apiPort))
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

func toIdentityList(confs []ContainerConfig) flow.IdentityList {
	il := make(flow.IdentityList, 0, len(confs))
	for _, conf := range confs {
		il = append(il, conf.Identity())
	}
	return il
}

func toNodeInfoList(confs []ContainerConfig) []bootstrap.NodeInfo {
	infos := make([]bootstrap.NodeInfo, 0, len(confs))
	for _, conf := range confs {
		infos = append(infos, conf.NodeInfo)
	}
	return infos
}

// getSeeds returns a list of n random seeds of 48 bytes each. This is used in
// conjunction with bootstrap key generation functions.
func getSeeds(n int) ([][]byte, error) {
	seeds := make([][]byte, n)
	for i := 0; i < n; i++ {
		seed := make([]byte, 48)
		_, err := rand.Read(seed)
		if err != nil {
			return nil, err
		}

		seeds[i] = seed
	}
	return seeds, nil
}

func writeJSON(path string, data interface{}) error {
	marshaled, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(path, marshaled, 0644)
	return err
}
