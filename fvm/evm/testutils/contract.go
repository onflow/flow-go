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
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

type TestContract struct {
	Code       string
	ABI        string
	ByteCode   []byte
	DeployedAt types.Address
}

func (tc *TestContract) MakeCallData(t testing.TB, name string, args ...interface{}) []byte {
	abi, err := gethABI.JSON(strings.NewReader(tc.ABI))
	require.NoError(t, err)
	call, err := abi.Pack(name, args...)
	require.NoError(t, err)
	return call
}

func (tc *TestContract) SetDeployedAt(deployedAt types.Address) {
	tc.DeployedAt = deployedAt
}

func GetStorageTestContract(tb testing.TB) *TestContract {
	byteCodes, err := hex.DecodeString("6080604052610249806100115f395ff3fe608060405234801561000f575f80fd5b5060043610610060575f3560e01c80632e64cec11461006457806348b151661461008257806357e871e7146100a05780635ec01e4d146100be5780636057361d146100dc57806385df51fd146100f8575b5f80fd5b61006c610128565b6040516100799190610170565b60405180910390f35b61008a610130565b6040516100979190610170565b60405180910390f35b6100a8610137565b6040516100b59190610170565b60405180910390f35b6100c661013e565b6040516100d39190610170565b60405180910390f35b6100f660048036038101906100f191906101b7565b610145565b005b610112600480360381019061010d91906101b7565b61014e565b60405161011f91906101fa565b60405180910390f35b5f8054905090565b5f42905090565b5f43905090565b5f44905090565b805f8190555050565b5f81409050919050565b5f819050919050565b61016a81610158565b82525050565b5f6020820190506101835f830184610161565b92915050565b5f80fd5b61019681610158565b81146101a0575f80fd5b50565b5f813590506101b18161018d565b92915050565b5f602082840312156101cc576101cb610189565b5b5f6101d9848285016101a3565b91505092915050565b5f819050919050565b6101f4816101e2565b82525050565b5f60208201905061020d5f8301846101eb565b9291505056fea26469706673582212204e444dbbee71334344ae4d9fe1b45944b0aff9ffd6b8ac8a33cc0f31c6e21d6664736f6c63430008170033")
	require.NoError(tb, err)
	return &TestContract{
		Code: `
		pragma solidity >=0.7.0 <0.9.0;

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
					return block.timestamp;
				}
				function blockHash(uint num)  public view returns (bytes32) {
					return blockhash(num);
				}
			
				function random() public view returns (uint256) {
					return block.prevrandao;
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
				"name": "random",
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

func GetDummyKittyTestContract(t testing.TB) *TestContract {
	byteCodes, err := hex.DecodeString("608060405234801561001057600080fd5b506107dd806100206000396000f3fe608060405234801561001057600080fd5b50600436106100415760003560e01c8063a45f4bfc14610046578063d0b169d114610076578063ddf252ad146100a6575b600080fd5b610060600480360381019061005b91906104e4565b6100c2565b60405161006d9190610552565b60405180910390f35b610090600480360381019061008b919061056d565b6100f5565b60405161009d91906105e3565b60405180910390f35b6100c060048036038101906100bb919061062a565b610338565b005b60026020528060005260406000206000915054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b60008463ffffffff16851461010957600080fd5b8363ffffffff16841461011b57600080fd5b8261ffff16831461012b57600080fd5b60006040518060a001604052808481526020014267ffffffffffffffff1681526020018763ffffffff1681526020018663ffffffff1681526020018561ffff16815250905060018190806001815401808255809150506001900390600052602060002090600202016000909190919091506000820151816000015560208201518160010160006101000a81548167ffffffffffffffff021916908367ffffffffffffffff16021790555060408201518160010160086101000a81548163ffffffff021916908363ffffffff160217905550606082015181600101600c6101000a81548163ffffffff021916908363ffffffff16021790555060808201518160010160106101000a81548161ffff021916908361ffff16021790555050507fc1e409485f45287e73ab1623a8f2ef17af5eac1b4c792ee9ec466e8795e7c09133600054836040015163ffffffff16846060015163ffffffff16856000015160405161029995949392919061067d565b60405180910390a13073ffffffffffffffffffffffffffffffffffffffff1663ddf252ad6000336000546040518463ffffffff1660e01b81526004016102e1939291906106d0565b600060405180830381600087803b1580156102fb57600080fd5b505af115801561030f573d6000803e3d6000fd5b5050505060008081548092919061032590610736565b9190505550600054915050949350505050565b600360008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600081548092919061038890610736565b9190505550816002600083815260200190815260200160002060006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff160217905550600073ffffffffffffffffffffffffffffffffffffffff168373ffffffffffffffffffffffffffffffffffffffff161461046957600360008473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008154809291906104639061077e565b91905055505b7feaf1c4b3ce0f4f62a2bae7eb3e68225c75f7e6ff4422073b7437b9a78d25f17083838360405161049c939291906106d0565b60405180910390a1505050565b600080fd5b6000819050919050565b6104c1816104ae565b81146104cc57600080fd5b50565b6000813590506104de816104b8565b92915050565b6000602082840312156104fa576104f96104a9565b5b6000610508848285016104cf565b91505092915050565b600073ffffffffffffffffffffffffffffffffffffffff82169050919050565b600061053c82610511565b9050919050565b61054c81610531565b82525050565b60006020820190506105676000830184610543565b92915050565b60008060008060808587031215610587576105866104a9565b5b6000610595878288016104cf565b94505060206105a6878288016104cf565b93505060406105b7878288016104cf565b92505060606105c8878288016104cf565b91505092959194509250565b6105dd816104ae565b82525050565b60006020820190506105f860008301846105d4565b92915050565b61060781610531565b811461061257600080fd5b50565b600081359050610624816105fe565b92915050565b600080600060608486031215610643576106426104a9565b5b600061065186828701610615565b935050602061066286828701610615565b9250506040610673868287016104cf565b9150509250925092565b600060a0820190506106926000830188610543565b61069f60208301876105d4565b6106ac60408301866105d4565b6106b960608301856105d4565b6106c660808301846105d4565b9695505050505050565b60006060820190506106e56000830186610543565b6106f26020830185610543565b6106ff60408301846105d4565b949350505050565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052601160045260246000fd5b6000610741826104ae565b91507fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff820361077357610772610707565b5b600182019050919050565b6000610789826104ae565b91506000820361079c5761079b610707565b5b60018203905091905056fea2646970667358221220ab35c07ec72cc064a663de06ec7f5f919b1a499a25cf6ef0c63a45fdd4a1e91e64736f6c63430008120033")
	require.NoError(t, err)
	return &TestContract{
		Code: `
			contract DummyKitty {
				
				event BirthEvent(address owner, uint256 kittyId, uint256 matronId, uint256 sireId, uint256 genes);
				event TransferEvent(address from, address to, uint256 tokenId);
			
				struct Kitty {
					uint256 genes;
					uint64 birthTime;
					uint32 matronId;
					uint32 sireId;
					uint16 generation;
				}
			
				uint256 idCounter;
			
				// @dev all kitties 
				Kitty[] kitties;
			
				/// @dev a mapping from cat IDs to the address that owns them. 
				mapping (uint256 => address) public kittyIndexToOwner;
			
				// @dev a mapping from owner address to count of tokens that address owns.
				mapping (address => uint256) ownershipTokenCount;
			
				/// @dev a method to transfer kitty
				function Transfer(address _from, address _to, uint256 _tokenId) external {
					// Since the number of kittens is capped to 2^32 we can't overflow this
					ownershipTokenCount[_to]++;
					// transfer ownership
					kittyIndexToOwner[_tokenId] = _to;
					// When creating new kittens _from is 0x0, but we can't account that address.
					if (_from != address(0)) {
						ownershipTokenCount[_from]--;
					}
					// Emit the transfer event.
					emit TransferEvent(_from, _to, _tokenId);
				}
			
				/// @dev a method callable by anyone to create a kitty
				function CreateKitty(
					uint256 _matronId,
					uint256 _sireId,
					uint256 _generation,
					uint256 _genes
				)
					external
					returns (uint)
				{
			
					require(_matronId == uint256(uint32(_matronId)));
					require(_sireId == uint256(uint32(_sireId)));
					require(_generation == uint256(uint16(_generation)));
			
					Kitty memory _kitty = Kitty({
						genes: _genes,
						birthTime: uint64(block.timestamp),
						matronId: uint32(_matronId),
						sireId: uint32(_sireId),
						generation: uint16(_generation)
					});
			
					kitties.push(_kitty);
			
					emit BirthEvent(
						msg.sender,
						idCounter,
						uint256(_kitty.matronId),
						uint256(_kitty.sireId),
						_kitty.genes
					);
			
					this.Transfer(address(0), msg.sender, idCounter);
			
					idCounter++;
			
					return idCounter;
				}
			}
		`,

		ABI: `
		[
			{
				"anonymous": false,
				"inputs": [
					{
						"indexed": false,
						"internalType": "address",
						"name": "owner",
						"type": "address"
					},
					{
						"indexed": false,
						"internalType": "uint256",
						"name": "kittyId",
						"type": "uint256"
					},
					{
						"indexed": false,
						"internalType": "uint256",
						"name": "matronId",
						"type": "uint256"
					},
					{
						"indexed": false,
						"internalType": "uint256",
						"name": "sireId",
						"type": "uint256"
					},
					{
						"indexed": false,
						"internalType": "uint256",
						"name": "genes",
						"type": "uint256"
					}
				],
				"name": "BirthEvent",
				"type": "event"
			},
			{
				"anonymous": false,
				"inputs": [
					{
						"indexed": false,
						"internalType": "address",
						"name": "from",
						"type": "address"
					},
					{
						"indexed": false,
						"internalType": "address",
						"name": "to",
						"type": "address"
					},
					{
						"indexed": false,
						"internalType": "uint256",
						"name": "tokenId",
						"type": "uint256"
					}
				],
				"name": "TransferEvent",
				"type": "event"
			},
			{
				"inputs": [
					{
						"internalType": "uint256",
						"name": "_matronId",
						"type": "uint256"
					},
					{
						"internalType": "uint256",
						"name": "_sireId",
						"type": "uint256"
					},
					{
						"internalType": "uint256",
						"name": "_generation",
						"type": "uint256"
					},
					{
						"internalType": "uint256",
						"name": "_genes",
						"type": "uint256"
					}
				],
				"name": "CreateKitty",
				"outputs": [
					{
						"internalType": "uint256",
						"name": "",
						"type": "uint256"
					}
				],
				"stateMutability": "nonpayable",
				"type": "function"
			},
			{
				"inputs": [
					{
						"internalType": "address",
						"name": "_from",
						"type": "address"
					},
					{
						"internalType": "address",
						"name": "_to",
						"type": "address"
					},
					{
						"internalType": "uint256",
						"name": "_tokenId",
						"type": "uint256"
					}
				],
				"name": "Transfer",
				"outputs": [],
				"stateMutability": "nonpayable",
				"type": "function"
			},
			{
				"inputs": [
					{
						"internalType": "uint256",
						"name": "",
						"type": "uint256"
					}
				],
				"name": "kittyIndexToOwner",
				"outputs": [
					{
						"internalType": "address",
						"name": "",
						"type": "address"
					}
				],
				"stateMutability": "view",
				"type": "function"
			}
		]
		`,
		ByteCode: byteCodes,
	}
}

func RunWithDeployedContract(t testing.TB, tc *TestContract, led atree.Ledger, flowEVMRootAddress flow.Address, f func(*TestContract)) {
	DeployContract(t, tc, led, flowEVMRootAddress)
	f(tc)
}

func DeployContract(t testing.TB, tc *TestContract, led atree.Ledger, flowEVMRootAddress flow.Address) {
	// deploy contract
	e := emulator.NewEmulator(led, flowEVMRootAddress)

	blk, err := e.NewBlockView(types.NewDefaultBlockContext(2))
	require.NoError(t, err)

	caller := types.NewAddress(gethCommon.Address{})
	_, err = blk.DirectCall(
		types.NewDepositCall(
			caller,
			new(big.Int).Mul(big.NewInt(1e18), big.NewInt(1000)),
		),
	)
	require.NoError(t, err)

	blk2, err := e.NewBlockView(types.NewDefaultBlockContext(3))
	require.NoError(t, err)

	res, err := blk2.DirectCall(
		types.NewDeployCall(
			caller,
			tc.ByteCode,
			math.MaxUint64,
			big.NewInt(0),
		),
	)
	require.NoError(t, err)

	tc.SetDeployedAt(res.DeployedContractAddress)
}
