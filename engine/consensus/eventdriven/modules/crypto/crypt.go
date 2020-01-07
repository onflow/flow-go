package crypto

import (
	"crypto/rand"
	"github.com/dapperlabs/flow-go/engine/consensus/HotStuff/types"
	"time"
)

const timeToSignSingle uint = 700
const timeToVerifySingle uint = 3600
const timeToAggregate10000Sigs uint = 15000
const timeToAggregate10000PubKeys uint = 45000
const sigLength uint = 48
const PubKeyLength uint = 96

type AggregatedPubKey struct {
	RawPubKey []byte
}

func SignMsg(msg interface{}, signerIdx uint32) *types.Signature {
	rawSig := [32]byte{}
	// Fill rawSig with random data
	rand.Read(rawSig[:])
	time.Sleep(time.Duration(timeToSignSingle) * time.Microsecond)

	return &types.Signature{
		RawSignature: rawSig,
		SignerIdx:    signerIdx,
	}
}

func VerifySig(rawData interface{}, sig *types.Signature) bool {
	time.Sleep(time.Duration(timeToVerifySingle) * time.Microsecond)

	return true
}

func AggregatePubKeys(pubKeys [][]byte) *AggregatedPubKey {
	rawPubKey := make([]byte, PubKeyLength)
	// Fill rawPubKey with random data
	rand.Read(rawPubKey)

	timeToAggregatePubKeys := (timeToAggregate10000PubKeys * uint(len(pubKeys))) / 10000
	time.Sleep(time.Duration(timeToAggregatePubKeys) * time.Microsecond)

	return &AggregatedPubKey{
		RawPubKey: rawPubKey,
	}
}

func AggregateSigs(sigs []*types.Signature, signersBitfieldLength uint32) *types.AggregatedSignature {
	rawSig := make([]byte, sigLength)
	signers := make([]bool, signersBitfieldLength)
	// Fill rawSig with random data
	rand.Read(rawSig)

	for _, sig := range sigs {
		signers[sig.SignerIdx] = true
	}

	// Aggregating sig time is linear with the number of sigs
	// Dividing last minimises rounding error
	timeToAggregateSigs := (timeToAggregate10000Sigs * uint(len(sigs))) / 10000
	time.Sleep(time.Duration(timeToAggregateSigs) * time.Microsecond)

	return &types.AggregatedSignature{
		RawSignature: rawSig,
		Signers:      signers,
	}
}

func VerifyAggregatedSig(rawData interface{}, aggregatedSig *types.AggregatedSignature, aggregatedPubKey *AggregatedPubKey) bool {
	time.Sleep(time.Duration(timeToVerifySingle) * time.Microsecond)

	return true
}
