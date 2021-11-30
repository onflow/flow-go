package rest

import (
	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/generated"
	"github.com/onflow/flow-go/model/flow"
)

const blockHeightQueryParam = "block_height"
const expandableFieldKeys = "keys"
const expandableFieldContracts = "contracts"

func getAccount(r *requestDecorator, backend access.API, link LinkGenerator) (interface{}, error) {
	address, err := toAddress(r.getVar("address"))
	if err != nil {
		return nil, NewBadRequestError(err)
	}

	var account *flow.Account
	height := r.getQueryParam(blockHeightQueryParam)
	switch height {
	case sealedHeightQueryParam:
		account, err = backend.GetAccountAtLatestBlock(r.Context(), address)
		if err != nil {
			return nil, err
		}
	case finalHeightQueryParam:
		// GetAccountAtLatestBlock only lookup account at the latest sealed block, hence lookup finalized block using
		// the GetLatestBlockHeader call first to find latest finalized height
		finalizedBlkHeader, err := backend.GetLatestBlockHeader(r.Context(), false)
		if err != nil {
			return nil, err
		}
		account, err = backend.GetAccountAtBlockHeight(r.Context(), address, finalizedBlkHeader.Height)
		if err != nil {
			return nil, err
		}
	default:
		h, err := toHeight(height)
		if err != nil {
			return nil, NewBadRequestError(err)
		}
		account, err = backend.GetAccountAtBlockHeight(r.Context(), address, h)
		if err != nil {
			return nil, err
		}
	}

	expandContracts := r.expands(expandableFieldContracts)
	expandKeys := r.expands(expandableFieldKeys)
	return accountResponse(account, link, expandKeys, expandContracts)
}

func accountResponse(flowAccount *flow.Account, link LinkGenerator, expandKeys bool, expandContracts bool) (generated.Account, error) {

	account := generated.Account{
		Address: flowAccount.Address.String(),
		Balance: int32(flowAccount.Balance),
	}

	if expandKeys {
		account.Keys = accountKeysResponse(flowAccount.Keys)
	} else {
		account.Expandable = &generated.AccountExpandable{
			Keys: expandableFieldKeys,
		}
	}

	if expandContracts {
		contracts := make(map[string]string, len(flowAccount.Contracts))
		for name, code := range flowAccount.Contracts {
			contracts[name] = toBase64(code)
		}
		account.Contracts = contracts
	} else {
		if account.Expandable == nil {
			account.Expandable = &generated.AccountExpandable{}
		}
		account.Expandable.Contracts = expandableFieldContracts
	}

	selfLink, err := link.AccountLink(account.Address)
	if err != nil {
		return generated.Account{}, nil
	}
	account.Links = &generated.Links{
		Self: selfLink,
	}

	return account, nil
}
