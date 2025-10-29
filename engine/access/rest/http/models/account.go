package models

import (
	"github.com/onflow/flow-go/engine/access/rest/common/models"
	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
	"github.com/onflow/flow-go/engine/access/rest/util"
	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
)

const expandableKeys = "keys"
const expandableContracts = "contracts"

func NewAccount(
	flowAccount *flow.Account,
	link models.LinkGenerator,
	expand map[string]bool,
	metadata *access.ExecutorMetadata,
	shouldIncludeMetadata bool,
) (*Account, error) {
	var meta *commonmodels.Metadata
	if shouldIncludeMetadata {
		meta = commonmodels.NewMetadata(metadata)
	}

	account := &Account{
		Address:    flowAccount.Address.String(),
		Balance:    util.FromUint(flowAccount.Balance),
		Expandable: &AccountExpandable{},
		Metadata:   meta,
	}

	if expand[expandableKeys] {
		account.Keys = NewAccountKeys(flowAccount.Keys, metadata, shouldIncludeMetadata)
	} else {
		account.Expandable.Keys = expandableKeys
	}

	if expand[expandableContracts] {
		contracts := make(map[string]string, len(flowAccount.Contracts))
		for name, code := range flowAccount.Contracts {
			contracts[name] = util.ToBase64(code)
		}
		account.Contracts = contracts
	} else {
		account.Expandable.Contracts = expandableContracts
	}

	var self models.Links
	err := self.Build(link.AccountLink(account.Address))
	if err != nil {
		return nil, err
	}
	account.Links = &self

	return account, nil
}

// TODO(Uliana): maybe use pointer to return
func NewAccountPublicKey(
	k flow.AccountPublicKey,
	metadata *access.ExecutorMetadata,
	shouldIncludeMetadata bool,
) AccountPublicKey {
	sigAlgo := SigningAlgorithm(k.SignAlgo.String())
	hashAlgo := HashingAlgorithm(k.HashAlgo.String())

	var meta *commonmodels.Metadata
	if shouldIncludeMetadata {
		meta = commonmodels.NewMetadata(metadata)
	}

	return AccountPublicKey{
		Index:            util.FromUint(k.Index),
		PublicKey:        k.PublicKey.String(),
		SigningAlgorithm: &sigAlgo,
		HashingAlgorithm: &hashAlgo,
		SequenceNumber:   util.FromUint(k.SeqNumber),
		Weight:           util.FromUint(uint64(k.Weight)),
		Revoked:          k.Revoked,
		Metadata:         meta,
	}
}

type AccountKeys []AccountPublicKey

func NewAccountKeys(
	accountKeys []flow.AccountPublicKey,
	metadata *access.ExecutorMetadata,
	shouldIncludeMetadata bool,
) AccountKeys {
	keys := make([]AccountPublicKey, 0, len(accountKeys))
	for i, k := range accountKeys {
		keys[i] = NewAccountPublicKey(k, metadata, shouldIncludeMetadata)
	}

	return keys
}

// NewAccountPublicKeys function use model AccountPublicKeys type for GetAccountKeys call
// AccountPublicKeys is an auto-generated type from the openapi spec
func NewAccountPublicKeys(
	accountKeys []flow.AccountPublicKey,
	metadata *access.ExecutorMetadata,
	shouldIncludeMetadata bool,
) AccountPublicKeys {
	keys := make([]AccountPublicKey, 0, len(accountKeys))
	for i, k := range accountKeys {
		keys[i] = NewAccountPublicKey(k, metadata, shouldIncludeMetadata)
	}

	return AccountPublicKeys{
		Keys: keys,
	}
}

func NewAccountBalance(
	balance uint64,
	metadata *access.ExecutorMetadata,
	shouldIncludeMetadata bool,
) AccountBalance {
	var meta *commonmodels.Metadata
	if shouldIncludeMetadata {
		meta = commonmodels.NewMetadata(metadata)
	}

	return AccountBalance{
		Balance:  util.FromUint(balance),
		Metadata: meta,
	}
}
