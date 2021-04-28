package main

import (
	"encoding/json"
	"fmt"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func handle(err error) {
	if err != nil {
		panic(err)
	}
}

// demonstrates how, without custom RLP encoding defined, an identity with the
// same key but wrapped with different crypto.PublicKey implementations can
// have inconsistent RLP encodings (and consequently different IDs / hashes).
func main() {
	key, err := crypto.GeneratePrivateKey(crypto.ECDSAP256, unittest.SeedFixture(128))
	handle(err)

	id := unittest.IdentifierFixture()
	role := flow.RoleConsensus
	addr := "node@flow.com"
	stake := uint64(100)

	// create an identity with the public key specified directly as the crypto
	// library type
	identity1 := &flow.Identity{
		NodeID:        id,
		Role:          role,
		Address:       addr,
		Stake:         stake,
		NetworkPubKey: key.PublicKey(),
	}

	// create an identity with the public key wrapped by the encodable type,
	// which provides encoding methods
	//
	// this is how (only) partner nodes are instantiated in the bootstrapping
	// because partner nodes are represented as bootstrap.NodeInfoPub and
	// internal nodes are represented as bootstrap.NodeInfoPriv.
	identity2 := &flow.Identity{
		NodeID:        id,
		Role:          role,
		Address:       addr,
		Stake:         stake,
		NetworkPubKey: encodable.NetworkPubKey{PublicKey: key.PublicKey()},
	}

	// 1 - check the IDs of each identity
	//
	// they are inconsistent because:
	// * no custom RLP encoding is specified for the keys
	// * the default RLP encoding is structure-dependent - since one
	//   key type has a wrapper and one does not, the encoding differs
	fmt.Println("id1: ", flow.MakeID(identity1))
	fmt.Println("id2: ", flow.MakeID(identity2))

	// 2 - encode and decode
	//
	// this mimics the process of initially writing the root snapshot file
	// during the finalize command, then reading that file again within the
	// node software.
	identity1 = encodeDecode(identity1)
	identity2 = encodeDecode(identity2)

	// 3 - check the IDs of each identity
	//
	// they are now consistent because:
	// * custom JSON encoding is defined, so both keys were encoded the same
	//   regardless of the wrapper
	// * after the initial encode (which is consistent regardless of whether
	//   the wrapper is used), the keys are always decoded the same way
	//   (without the wrapper)
	fmt.Println("after encode/decode cycle...")
	fmt.Println("id1: ", flow.MakeID(identity1))
	fmt.Println("id2: ", flow.MakeID(identity2))
}

func encodeDecode(identity *flow.Identity) *flow.Identity {
	bz, err := json.Marshal(identity)
	handle(err)
	var decoded flow.Identity
	err = json.Unmarshal(bz, &decoded)
	handle(err)
	return &decoded
}
