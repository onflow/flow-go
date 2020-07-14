package common

import (
	"encoding/binary"
	"encoding/hex"

	"github.com/dapperlabs/flow-go/ledger"
)

// OneBytePath returns a path (1 byte) given a uint8
func OneBytePath(inp uint8) ledger.Path {
	return ledger.Path([]byte{inp})
}

// TwoBytesPath returns a path (2 bytes) given a uint16
func TwoBytesPath(inp uint16) ledger.Path {
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, inp)
	return ledger.Path(b)
}

func LightPayload(key uint16, value uint16) *ledger.Payload {
	k := ledger.Key{KeyParts: []ledger.KeyPart{ledger.KeyPart{Type: 0, Value: Uint16ToBinary(key)}}}
	v := ledger.Value(Uint16ToBinary(value))
	return &ledger.Payload{Key: k, Value: v}
}

func LightPayload8(key uint8, value uint8) *ledger.Payload {
	k := ledger.Key{KeyParts: []ledger.KeyPart{ledger.KeyPart{Type: 0, Value: []byte{key}}}}
	v := ledger.Value([]byte{value})
	return &ledger.Payload{Key: k, Value: v}
}

func KeyPartFixture(typ uint16, val string) *ledger.KeyPart {
	kp1t := uint16(typ)
	kp1v := []byte(val)
	return ledger.NewKeyPart(kp1t, kp1v)
}

func UpdateFixture() *ledger.Update {
	sc, _ := hex.DecodeString("6a7a565add94fb36069d79e8725c221cd1e5740742501ef014ea6db999fd98ad")
	k1p1 := ledger.NewKeyPart(uint16(1), []byte("1"))
	k1p2 := ledger.NewKeyPart(uint16(22), []byte("2"))
	k1 := ledger.NewKey([]ledger.KeyPart{*k1p1, *k1p2})
	v1 := ledger.Value([]byte{'A'})

	k2p1 := ledger.NewKeyPart(uint16(1), []byte("3"))
	k2p2 := ledger.NewKeyPart(uint16(22), []byte("4"))
	k2 := ledger.NewKey([]ledger.KeyPart{*k2p1, *k2p2})
	v2 := ledger.Value([]byte{'B'})

	u, _ := ledger.NewUpdate(sc, []ledger.Key{*k1, *k2}, []ledger.Value{v1, v2})
	return u
}

func RootHashFixture() ledger.RootHash {
	sc, _ := hex.DecodeString("6a7a565add94fb36069d79e8725c221cd1e5740742501ef014ea6db999fd98ad")
	return ledger.RootHash(sc)
}

func TrieProofFixture() (*ledger.TrieProof, ledger.StateCommitment) {
	p := ledger.NewTrieProof()
	p.Path = TwoBytesPath(330)
	p.Payload = LightPayload8('A', 'A')
	p.Inclusion = true
	p.Flags = []byte{byte(130), byte(0)}
	p.Interims = make([][]byte, 0)
	interim1, _ := hex.DecodeString("accb0399dd2b3a7a48618b2376f5e61d822e0c7736b044c364a05c2904a2f315")
	interim2, _ := hex.DecodeString("f3fba426a2f01c342304e3ca7796c3980c62c625f7fd43105ad5afd92b165542")
	p.Interims = append(p.Interims, interim1)
	p.Interims = append(p.Interims, interim2)
	p.Steps = uint8(7)
	sc, _ := hex.DecodeString("6a7a565add94fb36069d79e8725c221cd1e5740742501ef014ea6db999fd98ad")
	return p, ledger.StateCommitment(sc)
}

func TrieBatchProofFixture() (*ledger.TrieBatchProof, ledger.StateCommitment) {
	p1 := ledger.NewTrieProof()
	p1.Path = TwoBytesPath(330)
	p1.Payload = LightPayload8('A', 'A')
	p1.Inclusion = true
	p1.Flags = []byte{byte(130), byte(0)}
	p1.Interims = make([][]byte, 0)
	p1interim1, _ := hex.DecodeString("accb0399dd2b3a7a48618b2376f5e61d822e0c7736b044c364a05c2904a2f315")
	p1interim2, _ := hex.DecodeString("f3fba426a2f01c342304e3ca7796c3980c62c625f7fd43105ad5afd92b165542")
	p1.Interims = append(p1.Interims, p1interim1)
	p1.Interims = append(p1.Interims, p1interim2)
	p1.Steps = uint8(7)

	p2 := ledger.NewTrieProof()
	p2.Path = TwoBytesPath(33354)
	p2.Payload = LightPayload8('C', 'C')
	p2.Inclusion = true
	p2.Flags = []byte{byte(129), byte(0)}
	p2.Interims = make([][]byte, 0)
	p2interim1, _ := hex.DecodeString("fbb89a7115c48406de7b976049223484c4a9fa1a018f36e13e2b5a6d02d29de2")
	p2interim2, _ := hex.DecodeString("71710ddc0967ef8fdc4e29444ae114165e339423799f9b7a05772cf19b71a9fd")
	p2.Interims = append(p2.Interims, p2interim1)
	p2.Interims = append(p2.Interims, p2interim2)
	p2.Steps = uint8(8)

	bp := ledger.NewTrieBatchProof()
	bp.Proofs = append(bp.Proofs, p1)
	bp.Proofs = append(bp.Proofs, p2)

	sc, _ := hex.DecodeString("6a7a565add94fb36069d79e8725c221cd1e5740742501ef014ea6db999fd98ad")
	return bp, ledger.StateCommitment(sc)
}
