/*
 * Flow Emulator
 *
 * Copyright Flow Foundation
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package emulator

import (
	"fmt"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"

	"github.com/onflow/crypto"
	"github.com/onflow/crypto/hash"
	"github.com/onflow/flow-go/access"
	flowgo "github.com/onflow/flow-go/model/flow"
)

// SignatureAlgorithm is an identifier for a signature algorithm (and parameters if applicable).
type SignatureAlgorithm = crypto.SigningAlgorithm

const (
	UnknownSignatureAlgorithm SignatureAlgorithm = crypto.UnknownSigningAlgorithm
	// ECDSA_P256 is ECDSA on NIST P-256 curve
	ECDSA_P256 = crypto.ECDSAP256
	// ECDSA_secp256k1 is ECDSA on secp256k1 curve
	ECDSA_secp256k1 = crypto.ECDSASecp256k1
	// BLS_BLS12_381 is BLS on BLS12-381 curve
	BLS_BLS12_381 = crypto.BLSBLS12381
)

// StringToSignatureAlgorithm converts a string to a SignatureAlgorithm.
func StringToSignatureAlgorithm(s string) SignatureAlgorithm {
	switch s {
	case ECDSA_P256.String():
		return ECDSA_P256
	case ECDSA_secp256k1.String():
		return ECDSA_secp256k1
	case BLS_BLS12_381.String():
		return BLS_BLS12_381
	default:
		return UnknownSignatureAlgorithm
	}
}

type ServiceKey struct {
	Index          uint32
	Address        flowgo.Address
	SequenceNumber uint64
	PrivateKey     crypto.PrivateKey
	PublicKey      crypto.PublicKey
	HashAlgo       hash.HashingAlgorithm
	SigAlgo        SignatureAlgorithm
	Weight         int
}

const defaultServiceKeyPrivateKeySeed = "elephant ears space cowboy octopus rodeo potato cannon pineapple"
const DefaultServiceKeySigAlgo = sdkcrypto.ECDSA_P256
const DefaultServiceKeyHashAlgo = sdkcrypto.SHA3_256

func DefaultServiceKey() *ServiceKey {
	return GenerateDefaultServiceKey(DefaultServiceKeySigAlgo, DefaultServiceKeyHashAlgo)
}

func GenerateDefaultServiceKey(
	sigAlgo crypto.SigningAlgorithm,
	hashAlgo hash.HashingAlgorithm,
) *ServiceKey {
	privateKey, err := crypto.GeneratePrivateKey(
		sigAlgo,
		[]byte(defaultServiceKeyPrivateKeySeed),
	)
	if err != nil {
		panic(fmt.Sprintf("Failed to generate default service key: %s", err.Error()))
	}

	return &ServiceKey{
		PrivateKey: privateKey,
		PublicKey:  privateKey.PublicKey(),
		SigAlgo:    sigAlgo,
		HashAlgo:   hashAlgo,
	}
}

func (s ServiceKey) Signer() (sdkcrypto.Signer, error) {
	return sdkcrypto.NewInMemorySigner(s.PrivateKey, s.HashAlgo)
}

func (s ServiceKey) AccountKey() (crypto.PublicKey, crypto.PrivateKey) {

	var publicKey crypto.PublicKey
	if s.PublicKey != nil {
		publicKey = s.PublicKey
	}

	if s.PrivateKey != nil {
		publicKey = s.PrivateKey.PublicKey()
	}

	return publicKey, s.PrivateKey

}

type AccessProvider interface {
	Ping() error
	GetNetworkParameters() access.NetworkParameters

	GetLatestBlock() (*flowgo.Block, error)
	GetBlockByID(id flowgo.Identifier) (*flowgo.Block, error)
	GetBlockByHeight(height uint64) (*flowgo.Block, error)

	GetCollectionByID(colID flowgo.Identifier) (*flowgo.LightCollection, error)
	GetFullCollectionByID(colID flowgo.Identifier) (*flowgo.Collection, error)

	GetTransaction(txID flowgo.Identifier) (*flowgo.TransactionBody, error)
	GetTransactionResult(txID flowgo.Identifier) (*access.TransactionResult, error)
	GetTransactionsByBlockID(blockID flowgo.Identifier) ([]*flowgo.TransactionBody, error)
	GetTransactionResultsByBlockID(blockID flowgo.Identifier) ([]*access.TransactionResult, error)

	GetAccount(address flowgo.Address) (*flowgo.Account, error)
	GetAccountAtBlockHeight(address flowgo.Address, blockHeight uint64) (*flowgo.Account, error)
	GetAccountByIndex(uint) (*flowgo.Account, error)

	GetEventsByHeight(blockHeight uint64, eventType string) ([]flowgo.Event, error)
	GetEventsForBlockIDs(eventType string, blockIDs []flowgo.Identifier) ([]flowgo.BlockEvents, error)
	GetEventsForHeightRange(eventType string, startHeight, endHeight uint64) ([]flowgo.BlockEvents, error)

	ExecuteScript(script []byte, arguments [][]byte) (*ScriptResult, error)
	ExecuteScriptAtBlockHeight(script []byte, arguments [][]byte, blockHeight uint64) (*ScriptResult, error)
	ExecuteScriptAtBlockID(script []byte, arguments [][]byte, id flowgo.Identifier) (*ScriptResult, error)

	SendTransaction(tx *flowgo.TransactionBody) error
	AddTransaction(tx flowgo.TransactionBody) error
}

type AutoMineCapable interface {
	EnableAutoMine()
	DisableAutoMine()
}

type ExecutionCapable interface {
	ExecuteAndCommitBlock() (*flowgo.Block, []*TransactionResult, error)
	ExecuteNextTransaction() (*TransactionResult, error)
	ExecuteBlock() ([]*TransactionResult, error)
	CommitBlock() (*flowgo.Block, error)
}

type Contract struct {
	Name   string
	Source string
}

// Emulator defines the method set of an emulated emulator.
type Emulator interface {
	ServiceKey() ServiceKey
	AccessProvider
	AutoMineCapable
	ExecutionCapable
}
