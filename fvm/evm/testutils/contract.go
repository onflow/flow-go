package testutils

import (
	"encoding/hex"
	"math"
	"math/big"
	"strings"
	"testing"

	gethABI "github.com/ethereum/go-ethereum/accounts/abi"
	gethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	"github.com/onflow/atree"

	"github.com/onflow/flow-go/fvm/evm/emulator"
	"github.com/onflow/flow-go/fvm/evm/emulator/database"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

type TestContract struct {
	Code       string
	ABI        string
	ByteCode   []byte
	DeployedAt types.Address
}

func (tc *TestContract) MakeStoreCallData(t *testing.T, num *big.Int) []byte {
	abi, err := gethABI.JSON(strings.NewReader(tc.ABI))
	require.NoError(t, err)
	store, err := abi.Pack("store", num)
	require.NoError(t, err)
	return store
}

func (tc *TestContract) MakeRetrieveCallData(t *testing.T) []byte {
	abi, err := gethABI.JSON(strings.NewReader(tc.ABI))
	require.NoError(t, err)
	retrieve, err := abi.Pack("retrieve")
	require.NoError(t, err)
	return retrieve
}

func (tc *TestContract) MakeBlockNumberCallData(t *testing.T) []byte {
	abi, err := gethABI.JSON(strings.NewReader(tc.ABI))
	require.NoError(t, err)
	blockNum, err := abi.Pack("blockNumber")
	require.NoError(t, err)
	return blockNum
}

func (tc *TestContract) SetDeployedAt(deployedAt types.Address) {
	tc.DeployedAt = deployedAt
}

func GetTestContract(t *testing.T) *TestContract {
	byteCodes, err := hex.DecodeString("608060405261022c806100136000396000f3fe608060405234801561001057600080fd5b50600436106100575760003560e01c80632e64cec11461005c57806348b151661461007a57806357e871e7146100985780636057361d146100b657806385df51fd146100d2575b600080fd5b610064610102565b6040516100719190610149565b60405180910390f35b61008261010b565b60405161008f9190610149565b60405180910390f35b6100a0610113565b6040516100ad9190610149565b60405180910390f35b6100d060048036038101906100cb9190610195565b61011b565b005b6100ec60048036038101906100e79190610195565b610125565b6040516100f991906101db565b60405180910390f35b60008054905090565b600042905090565b600043905090565b8060008190555050565b600081409050919050565b6000819050919050565b61014381610130565b82525050565b600060208201905061015e600083018461013a565b92915050565b600080fd5b61017281610130565b811461017d57600080fd5b50565b60008135905061018f81610169565b92915050565b6000602082840312156101ab576101aa610164565b5b60006101b984828501610180565b91505092915050565b6000819050919050565b6101d5816101c2565b82525050565b60006020820190506101f060008301846101cc565b9291505056fea26469706673582212203ee61567a25f0b1848386ae6b8fdbd7733c8a502c83b5ed305b921b7933f4e8164736f6c63430008120033")
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
				function blockNumber() public view returns (uint256) {
					return block.number;
				}
				function blockTime() public view returns (uint) {
					return  block.timestamp;
				}
				function blockHash(uint num)  public view returns (bytes32) {
					return blockhash(num);
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
				"inputs": [
					{
						"internalType": "uint256",
						"name": "num",
						"type": "uint256"
					}
				],
				"name": "blockHash",
				"outputs": [
					{
						"internalType": "bytes32",
						"name": "",
						"type": "bytes32"
					}
				],
				"stateMutability": "view",
				"type": "function"
			},
			{
				"inputs": [],
				"name": "blockNumber",
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
				"inputs": [],
				"name": "blockTime",
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

func RunWithDeployedContract(t *testing.T, led atree.Ledger, flowEVMRootAddress flow.Address, f func(*TestContract)) {
	tc := GetTestContract(t)
	// deploy contract
	db, err := database.NewDatabase(led, flowEVMRootAddress)
	require.NoError(t, err)

	e := emulator.NewEmulator(db)

	blk, err := e.NewBlockView(types.NewDefaultBlockContext(2))
	require.NoError(t, err)

	caller := types.NewAddress(gethCommon.Address{})
	_, err = blk.MintTo(caller, new(big.Int).Mul(big.NewInt(1e18), big.NewInt(1000)))
	require.NoError(t, err)

	res, err := blk.Deploy(caller, tc.ByteCode, math.MaxUint64, big.NewInt(0))
	require.NoError(t, err)

	tc.SetDeployedAt(res.DeployedContractAddress)
	f(tc)
}
