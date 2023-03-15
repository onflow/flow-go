package crypto

import (
	"encoding/hex"
	bn256 "github.com/onflow/flow-go/crypto/bn254/cloudflare"
	"math/big"
)

func GetBN256SignaturePoint(s Signature) (*bn256.G1, error) {
	point := new(bn256.G1)
	_, err := point.Unmarshal(s)
	if err != nil {
		return nil, err
	}
	return point, nil
}

func Point2BigInt(p *bn256.G1) []*big.Int {
	m := p.Marshal()
	x := new(big.Int).SetBytes(m[0:32])
	y := new(big.Int).SetBytes(m[32:])
	return []*big.Int{x, y}
}

func BigToHex32(bi *big.Int) string {
	return "0x" + hex.EncodeToString(BigToBytes(bi, 32))
}
