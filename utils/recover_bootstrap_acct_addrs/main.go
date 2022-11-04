package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/flow/order"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/utils/io"
)

const ROOT_SNAPSHOT_PATH = "/Users/dapperlabs/sandbox2-root-snapshot.json"
const OUT_DIR = "/Users/dapperlabs/dev/src/github.com/onflow/flow-mgmt/json/sandbox"

func loadSnapshot(path string) *inmem.Snapshot {
	data, err := io.ReadFile(path)
	if err != nil {
		panic(err)
	}

	var snapshot inmem.EncodableSnapshot
	err = json.Unmarshal(data, &snapshot)
	if err != nil {
		panic(err)
	}

	return inmem.SnapshotFromEncodable(snapshot)
}

func writeJSON(path string, data interface{}) {
	bz, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		panic(err)
	}

	writeText(path, bz)
}

func writeText(path string, data []byte) {
	err := os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		panic(err)
	}

	err = os.WriteFile(path, data, 0644)
	if err != nil {
		panic(err)
	}
}

func main() {

	snapshot := loadSnapshot(ROOT_SNAPSHOT_PATH)
	identities, err := snapshot.Identities(filter.Any)
	if err != nil {
		panic(err)
	}
	identities = identities.Sort(order.Canonical)

	// STEP 1: construct mapping nodeID->acctAddr
	//  -> can ignore stake
	idToStakingAddress := make(map[flow.Identifier]flow.Address)
	addressIndex := uint64(4) // points to the first address BEFORE any machine account or staking account
	for _, identity := range identities {

		// increment address index to point to staking account for identity
		addressIndex += 1

		stakingAddress, err := flow.Sandboxnet.Chain().AddressAtIndex(addressIndex)
		if err != nil {
			panic(err)
		}
		idToStakingAddress[identity.NodeID] = stakingAddress

		if identity.Role == flow.RoleCollection || identity.Role == flow.RoleConsensus {
			addressIndex += 1 // skip the machine account
		}
	}

	// STEP 2: generate files

	type nodeConf struct {
		Role           string
		Address        string // netw addr
		AccountAddress string // staking acct
	}
	type nodeInfoPub struct {
		Role          string
		Address       string // netw addr
		NodeID        string
		Stake         uint64
		NetworkPubKey string
		StakingPubKey string
	}

	nodeConfigWithAddressJSON := []nodeConf{}
	nodeInfosPubJSON := []nodeInfoPub{}

	for _, identity := range identities {
		nc := nodeConf{
			Role:           identity.Role.String(),
			Address:        identity.Address,
			AccountAddress: idToStakingAddress[identity.NodeID].Hex(),
		}
		ni := nodeInfoPub{
			Role:          identity.Role.String(),
			Address:       identity.Address,
			NodeID:        identity.NodeID.String(),
			Stake:         1000,
			NetworkPubKey: identity.NetworkPubKey.String(),
			StakingPubKey: identity.StakingPubKey.String(),
		}
		nodeConfigWithAddressJSON = append(nodeConfigWithAddressJSON, nc)
		nodeInfosPubJSON = append(nodeInfosPubJSON, ni)
	}

	writeJSON(filepath.Join(OUT_DIR, "node-config-with-address.json"), nodeConfigWithAddressJSON)
	writeJSON(filepath.Join(OUT_DIR, "node-infos-pub.json"), nodeInfosPubJSON)
}

/*
FILE 1: node-config-with-address.json
{
    "Role": "verification",
    "Address": "verification-009.canary6.nodes.onflow.org:3569",
    "AccountAddress": "dfda40267c1257ce",
    "Stake": "135000.0"
  },
FILE 2: node-infos-pub.json
{
    "Role": "consensus",
    "Address": "consensus-002.canary6.nodes.onflow.org:3569",
    "NodeID": "56697368616c204368616e6772616e6900e9862a29da0d55be0a6f2ff90cf31f",
    "Stake": 1000,
    "NetworkPubKey": "fa7e894e478bd4146b4c62345d03052cfef2adc635cbaf835ef5a2575cb43760b075bf40c2b992abcccb8698b89f930fd487dd0999b333750a339abf544214c3",
    "StakingPubKey": "94c6fb3ee8b34bad128656c484114a7e924c9d9e2b75352a8a8646a404d46e5baa70a642175196a492d6700ccaf6fb50034c86bd195ddf55b3cd31486b63529b115947936be4e071b2e8312db51fd1cd78d712207d1e203dede65087485d73b6"
  },
*/
