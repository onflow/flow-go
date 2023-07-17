package testutils

import (
	crand "crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/rand"

	l "github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/common/utils"
)

// TrieUpdateFixture returns a trie update fixture
func TrieUpdateFixture(n int, minPayloadByteSize int, maxPayloadByteSize int) *l.TrieUpdate {
	return &l.TrieUpdate{
		RootHash: RootHashFixture(),
		Paths:    RandomPaths(n),
		Payloads: RandomPayloads(n, minPayloadByteSize, maxPayloadByteSize),
	}
}

// QueryFixture returns a query fixture
func QueryFixture() *l.Query {
	scBytes, _ := hex.DecodeString("6a7a565add94fb36069d79e8725c221cd1e5740742501ef014ea6db999fd98ad")
	var sc l.State
	copy(sc[:], scBytes)
	k1p1 := l.NewKeyPart(uint16(1), []byte("1"))
	k1p2 := l.NewKeyPart(uint16(22), []byte("2"))
	k1 := l.NewKey([]l.KeyPart{k1p1, k1p2})

	k2p1 := l.NewKeyPart(uint16(1), []byte("3"))
	k2p2 := l.NewKeyPart(uint16(22), []byte("4"))
	k2 := l.NewKey([]l.KeyPart{k2p1, k2p2})

	u, _ := l.NewQuery(sc, []l.Key{k1, k2})
	return u
}

// LightPayload returns a payload with 2 byte key and 2 byte value
func LightPayload(key uint16, value uint16) *l.Payload {
	k := l.Key{KeyParts: []l.KeyPart{{Type: 0, Value: utils.Uint16ToBinary(key)}}}
	v := l.Value(utils.Uint16ToBinary(value))
	return l.NewPayload(k, v)
}

// LightPayload8 returns a payload with 1 byte key and 1 byte value
func LightPayload8(key uint8, value uint8) *l.Payload {
	k := l.Key{KeyParts: []l.KeyPart{{Type: 0, Value: []byte{key}}}}
	v := l.Value([]byte{value})
	return l.NewPayload(k, v)
}

// PathByUint8 returns a path (32 bytes) given a uint8
func PathByUint8(inp uint8) l.Path {
	var b l.Path
	b[0] = inp
	return b
}

// PathByUint16 returns a path (32 bytes) given a uint16 (big endian)
func PathByUint16(inp uint16) l.Path {
	var b l.Path
	binary.BigEndian.PutUint16(b[:], inp)
	return b
}

// PathByUint16LeftPadded returns a path (32 bytes) given a uint16 (left padded big endian)
func PathByUint16LeftPadded(inp uint16) l.Path {
	var b l.Path
	binary.BigEndian.PutUint16(b[30:], inp)
	return b
}

// KeyPartFixture returns a key part fixture
func KeyPartFixture(typ uint16, val string) l.KeyPart {
	kp1t := typ
	kp1v := []byte(val)
	return l.NewKeyPart(kp1t, kp1v)
}

// UpdateFixture returns an update fixture
func UpdateFixture() *l.Update {
	scBytes, _ := hex.DecodeString("6a7a565add94fb36069d79e8725c221cd1e5740742501ef014ea6db999fd98ad")
	var sc l.State
	copy(sc[:], scBytes)
	k1p1 := l.NewKeyPart(uint16(1), []byte("1"))
	k1p2 := l.NewKeyPart(uint16(22), []byte("2"))
	k1 := l.NewKey([]l.KeyPart{k1p1, k1p2})
	v1 := l.Value([]byte{'A'})

	k2p1 := l.NewKeyPart(uint16(1), []byte("3"))
	k2p2 := l.NewKeyPart(uint16(22), []byte("4"))
	k2 := l.NewKey([]l.KeyPart{k2p1, k2p2})
	v2 := l.Value([]byte{'B'})

	u, _ := l.NewUpdate(sc, []l.Key{k1, k2}, []l.Value{v1, v2})
	return u
}

// RootHashFixture returns a root hash fixture
func RootHashFixture() l.RootHash {
	rootBytes, _ := hex.DecodeString("6a7a565add94fb36069d79e8725c221cd1e5740742501ef014ea6db999fd98ad")
	var root l.RootHash
	copy(root[:], rootBytes)
	return root
}

// TrieProofFixture returns a trie proof fixture
func TrieProofFixture() (*l.TrieProof, l.State) {
	p := l.NewTrieProof()
	p.Path = PathByUint16(330)
	p.Payload = LightPayload8('A', 'A')
	p.Inclusion = true
	p.Flags = []byte{byte(130), byte(0)}
	p.Interims = make([]hash.Hash, 0)
	interim1Bytes, _ := hex.DecodeString("accb0399dd2b3a7a48618b2376f5e61d822e0c7736b044c364a05c2904a2f315")
	interim2Bytes, _ := hex.DecodeString("f3fba426a2f01c342304e3ca7796c3980c62c625f7fd43105ad5afd92b165542")
	var interim1, interim2 hash.Hash
	copy(interim1[:], interim1Bytes)
	copy(interim2[:], interim2Bytes)
	p.Interims = append(p.Interims, interim1)
	p.Interims = append(p.Interims, interim2)
	p.Steps = uint8(7)
	scBytes, _ := hex.DecodeString("4a9f3a15d7257b624b645955576f62edcceff5e125f49585cdf077d9f37c7ac0")
	var sc l.State
	copy(sc[:], scBytes)
	return p, sc
}

// TrieBatchProofFixture returns a trie batch proof fixture
func TrieBatchProofFixture() (*l.TrieBatchProof, l.State) {
	p, s := TrieProofFixture()
	bp := l.NewTrieBatchProof()
	bp.Proofs = append(bp.Proofs, p)
	bp.Proofs = append(bp.Proofs, p)
	return bp, s
}

// RandomPathsRandLen generate m random paths.
// the number of paths, m, is also randomly selected from the range [1, maxN]
func RandomPathsRandLen(maxN int) []l.Path {
	numberOfPaths := rand.Intn(maxN) + 1
	return RandomPaths(numberOfPaths)
}

// RandomPaths generates n random (no repetition)
func RandomPaths(n int) []l.Path {
	paths := make([]l.Path, 0, n)
	alreadySelectPaths := make(map[l.Path]bool)
	i := 0
	for i < n {
		var path l.Path
		_, err := crand.Read(path[:])
		if err != nil {
			panic("randomness failed")
		}
		// deduplicate
		if _, found := alreadySelectPaths[path]; !found {
			paths = append(paths, path)
			alreadySelectPaths[path] = true
			i++
		}
	}
	return paths
}

// RandomPayload returns a random payload
func RandomPayload(minByteSize int, maxByteSize int) *l.Payload {
	keyByteSize := minByteSize + rand.Intn(maxByteSize-minByteSize)
	keydata := make([]byte, keyByteSize)
	_, err := crand.Read(keydata)
	if err != nil {
		panic("randomness failed")
	}
	key := l.Key{KeyParts: []l.KeyPart{{Type: 0, Value: keydata}}}
	valueByteSize := minByteSize + rand.Intn(maxByteSize-minByteSize)
	valuedata := make([]byte, valueByteSize)
	_, err = crand.Read(valuedata)
	if err != nil {
		panic("random generation failed")
	}
	value := l.Value(valuedata)
	return l.NewPayload(key, value)
}

// RandomPayloads returns n random payloads
func RandomPayloads(n int, minByteSize int, maxByteSize int) []*l.Payload {
	res := make([]*l.Payload, 0)
	for i := 0; i < n; i++ {
		res = append(res, RandomPayload(minByteSize, maxByteSize))
	}
	return res
}

// RandomValues returns n random values with variable sizes (minByteSize <= size < maxByteSize)
func RandomValues(n int, minByteSize, maxByteSize int) []l.Value {
	if minByteSize > maxByteSize {
		panic("minByteSize cannot be smaller then maxByteSize")
	}
	values := make([]l.Value, 0)
	for i := 0; i < n; i++ {
		var byteSize = maxByteSize
		if minByteSize < maxByteSize {
			byteSize = minByteSize + rand.Intn(maxByteSize-minByteSize)
		}
		value := make([]byte, byteSize)
		_, err := crand.Read(value)
		if err != nil {
			panic("random generation failed")
		}
		values = append(values, value)
	}
	return values
}

// RandomUniqueKeys generates n random keys (each with m random key parts)
func RandomUniqueKeys(n, m, minByteSize, maxByteSize int) []l.Key {
	if minByteSize > maxByteSize {
		panic("minByteSize cannot be smaller then maxByteSize")
	}
	keys := make([]l.Key, 0)
	alreadySelectKeys := make(map[string]bool)
	i := 0
	for i < n {
		keyParts := make([]l.KeyPart, 0)
		for j := 0; j < m; j++ {
			byteSize := maxByteSize
			if minByteSize < maxByteSize {
				byteSize = minByteSize + rand.Intn(maxByteSize-minByteSize)
			}
			keyPartData := make([]byte, byteSize)
			_, err := crand.Read(keyPartData)
			if err != nil {
				panic("random generation failed")
			}
			keyParts = append(keyParts, l.NewKeyPart(uint16(j), keyPartData))
		}
		key := l.NewKey(keyParts)

		// deduplicate
		if _, found := alreadySelectKeys[key.String()]; !found {
			keys = append(keys, key)
			alreadySelectKeys[key.String()] = true
			i++
		} else {
			fmt.Println("Already existing")
		}
	}
	return keys
}
