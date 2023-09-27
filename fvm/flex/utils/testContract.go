package utils

import (
	"encoding/hex"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

type TestContract struct {
	Code       string
	ABI        string
	ByteCode   []byte
	DeployedAt common.Address
}

func (tc *TestContract) MakeStoreCallData(t *testing.T, num *big.Int) []byte {
	abi, err := abi.JSON(strings.NewReader(tc.ABI))
	require.NoError(t, err)
	store, err := abi.Pack("store", num)
	require.NoError(t, err)
	return store
}

func (tc *TestContract) MakeRetrieveCallData(t *testing.T) []byte {
	abi, err := abi.JSON(strings.NewReader(tc.ABI))
	require.NoError(t, err)
	retrieve, err := abi.Pack("retrieve")
	require.NoError(t, err)
	return retrieve
}

func (tc *TestContract) SetDeployedAt(deployedAt common.Address) {
	tc.DeployedAt = deployedAt
}

func GetTestContract(t *testing.T) *TestContract {
	byteCodes, err := hex.DecodeString("6080604052610150806100136000396000f3fe608060405234801561001057600080fd5b50600436106100365760003560e01c80632e64cec11461003b5780636057361d14610059575b600080fd5b610043610075565b60405161005091906100a1565b60405180910390f35b610073600480360381019061006e91906100ed565b61007e565b005b60008054905090565b8060008190555050565b6000819050919050565b61009b81610088565b82525050565b60006020820190506100b66000830184610092565b92915050565b600080fd5b6100ca81610088565b81146100d557600080fd5b50565b6000813590506100e7816100c1565b92915050565b600060208284031215610103576101026100bc565b5b6000610111848285016100d8565b9150509291505056fea2646970667358221220029e22143e146846aff5dd684a6d627d0bec77c78e5b7ce77674d91c25d7e22264736f6c63430008120033")
	require.NoError(t, err)
	return &TestContract{
		Code: `
			contract Storage {
				uint256 number;
				constructor() payable {
				}
				function store(uint256 num) public {
					number = num;
				}
				function retrieve() public view returns (uint256){
					return number;
				}
			}
		`,

		ABI: `
			[
				{
					"inputs": [],
					"stateMutability": "payable",
					"type": "constructor"
				},
				{
					"inputs": [],
					"name": "retrieve",
					"outputs": [
						{
							"internalType": "uint256",
							"name": "",
							"type": "uint256"
						}
					],
					"stateMutability": "view",
					"type": "function"
				},
				{
					"inputs": [
						{
							"internalType": "uint256",
							"name": "num",
							"type": "uint256"
						}
					],
					"name": "store",
					"outputs": [],
					"stateMutability": "nonpayable",
					"type": "function"
				}
			]
		`,
		ByteCode: byteCodes,
	}
}
