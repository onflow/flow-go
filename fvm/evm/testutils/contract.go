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

func MakeCallData(t testing.TB, abiString string, name string, args ...interface{}) []byte {
	abi, err := gethABI.JSON(strings.NewReader(abiString))
	require.NoError(t, err)
	call, err := abi.Pack(name, args...)
	require.NoError(t, err)
	return call
}

func (tc *TestContract) MakeCallData(t testing.TB, name string, args ...interface{}) []byte {
	return MakeCallData(t, tc.ABI, name, args...)
}

func (tc *TestContract) SetDeployedAt(deployedAt types.Address) {
	tc.DeployedAt = deployedAt
}

func GetStorageTestContract(tb testing.TB) *TestContract {
	byteCodes, err := hex.DecodeString("6080604052610c4d806100115f395ff3fe608060405234801561000f575f80fd5b5060043610610091575f3560e01c80635ec01e4d116100645780635ec01e4d1461011f5780636057361d1461013d578063828dd0481461015957806385df51fd14610189578063d0d250bd146101b957610091565b80632e64cec11461009557806348b15166146100b357806352e24024146100d157806357e871e714610101575b5f80fd5b61009d6101d7565b6040516100aa9190610588565b60405180910390f35b6100bb6101df565b6040516100c89190610588565b60405180910390f35b6100eb60048036038101906100e691906105ef565b6101e6565b6040516100f89190610629565b60405180910390f35b610109610393565b6040516101169190610588565b60405180910390f35b61012761039a565b6040516101349190610588565b60405180910390f35b6101576004803603810190610152919061066c565b6103a1565b005b610173600480360381019061016e9190610895565b6103aa565b6040516101809190610924565b60405180910390f35b6101a3600480360381019061019e919061066c565b610559565b6040516101b0919061094c565b60405180910390f35b6101c1610563565b6040516101ce9190610974565b60405180910390f35b5f8054905090565b5f42905090565b5f805f6801000000000000000173ffffffffffffffffffffffffffffffffffffffff166040516024016040516020818303038152906040527f53e87d66000000000000000000000000000000000000000000000000000000007bffffffffffffffffffffffffffffffffffffffffffffffffffffffff19166020820180517bffffffffffffffffffffffffffffffffffffffffffffffffffffffff838183161783525050505060405161029991906109f9565b5f60405180830381855afa9150503d805f81146102d1576040519150601f19603f3d011682016040523d82523d5f602084013e6102d6565b606091505b50915091508161031b576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040161031290610a69565b60405180910390fd5b5f818060200190518101906103309190610a9b565b90508067ffffffffffffffff168567ffffffffffffffff1614610388576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040161037f90610b36565b60405180910390fd5b809350505050919050565b5f43905090565b5f44905090565b805f8190555050565b5f805f6801000000000000000173ffffffffffffffffffffffffffffffffffffffff168686866040516024016103e293929190610b9c565b6040516020818303038152906040527f5ee837e7000000000000000000000000000000000000000000000000000000007bffffffffffffffffffffffffffffffffffffffffffffffffffffffff19166020820180517bffffffffffffffffffffffffffffffffffffffffffffffffffffffff838183161783525050505060405161046c91906109f9565b5f60405180830381855afa9150503d805f81146104a4576040519150601f19603f3d011682016040523d82523d5f602084013e6104a9565b606091505b5091509150816104ee576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004016104e590610a69565b60405180910390fd5b5f818060200190518101906105039190610bec565b90508015158815151461054b576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040161054290610b36565b60405180910390fd5b809350505050949350505050565b5f81409050919050565b6801000000000000000181565b5f819050919050565b61058281610570565b82525050565b5f60208201905061059b5f830184610579565b92915050565b5f604051905090565b5f80fd5b5f80fd5b5f67ffffffffffffffff82169050919050565b6105ce816105b2565b81146105d8575f80fd5b50565b5f813590506105e9816105c5565b92915050565b5f60208284031215610604576106036105aa565b5b5f610611848285016105db565b91505092915050565b610623816105b2565b82525050565b5f60208201905061063c5f83018461061a565b92915050565b61064b81610570565b8114610655575f80fd5b50565b5f8135905061066681610642565b92915050565b5f60208284031215610681576106806105aa565b5b5f61068e84828501610658565b91505092915050565b5f8115159050919050565b6106ab81610697565b81146106b5575f80fd5b50565b5f813590506106c6816106a2565b92915050565b5f73ffffffffffffffffffffffffffffffffffffffff82169050919050565b5f6106f5826106cc565b9050919050565b610705816106eb565b811461070f575f80fd5b50565b5f81359050610720816106fc565b92915050565b5f819050919050565b61073881610726565b8114610742575f80fd5b50565b5f813590506107538161072f565b92915050565b5f80fd5b5f80fd5b5f601f19601f8301169050919050565b7f4e487b71000000000000000000000000000000000000000000000000000000005f52604160045260245ffd5b6107a782610761565b810181811067ffffffffffffffff821117156107c6576107c5610771565b5b80604052505050565b5f6107d86105a1565b90506107e4828261079e565b919050565b5f67ffffffffffffffff82111561080357610802610771565b5b61080c82610761565b9050602081019050919050565b828183375f83830152505050565b5f610839610834846107e9565b6107cf565b9050828152602081018484840111156108555761085461075d565b5b610860848285610819565b509392505050565b5f82601f83011261087c5761087b610759565b5b813561088c848260208601610827565b91505092915050565b5f805f80608085870312156108ad576108ac6105aa565b5b5f6108ba878288016106b8565b94505060206108cb87828801610712565b93505060406108dc87828801610745565b925050606085013567ffffffffffffffff8111156108fd576108fc6105ae565b5b61090987828801610868565b91505092959194509250565b61091e81610697565b82525050565b5f6020820190506109375f830184610915565b92915050565b61094681610726565b82525050565b5f60208201905061095f5f83018461093d565b92915050565b61096e816106eb565b82525050565b5f6020820190506109875f830184610965565b92915050565b5f81519050919050565b5f81905092915050565b5f5b838110156109be5780820151818401526020810190506109a3565b5f8484015250505050565b5f6109d38261098d565b6109dd8185610997565b93506109ed8185602086016109a1565b80840191505092915050565b5f610a0482846109c9565b915081905092915050565b5f82825260208201905092915050565b7f756e7375636365737366756c2063616c6c20746f2061726368000000000000005f82015250565b5f610a53601983610a0f565b9150610a5e82610a1f565b602082019050919050565b5f6020820190508181035f830152610a8081610a47565b9050919050565b5f81519050610a95816105c5565b92915050565b5f60208284031215610ab057610aaf6105aa565b5b5f610abd84828501610a87565b91505092915050565b7f6f757470757420646f65736e74206d61746368207468652065787065637465645f8201527f2076616c75650000000000000000000000000000000000000000000000000000602082015250565b5f610b20602683610a0f565b9150610b2b82610ac6565b604082019050919050565b5f6020820190508181035f830152610b4d81610b14565b9050919050565b5f82825260208201905092915050565b5f610b6e8261098d565b610b788185610b54565b9350610b888185602086016109a1565b610b9181610761565b840191505092915050565b5f606082019050610baf5f830186610965565b610bbc602083018561093d565b8181036040830152610bce8184610b64565b9050949350505050565b5f81519050610be6816106a2565b92915050565b5f60208284031215610c0157610c006105aa565b5b5f610c0e84828501610bd8565b9150509291505056fea26469706673582212200645c22bab8a46d284684ca77d210bb3c64f2ff98286830dea50f10dd4ae421c64736f6c63430008160033")
	require.NoError(tb, err)
	return &TestContract{
		Code: `
		// SPDX-License-Identifier: GPL-3.0

		pragma solidity >=0.7.0 <0.9.0;
		
			contract Storage {
		
			address constant public cadenceArch = 0x0000000000000000000000010000000000000001;
			
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
		
			function verifyArchCallToFlowBlockHeight(uint64 expected) public view returns (uint64){
				(bool ok, bytes memory data) = cadenceArch.staticcall(abi.encodeWithSignature("flowBlockHeight()"));
				require(ok, "unsuccessful call to arch ");
				uint64 output = abi.decode(data, (uint64));
				require(expected == output, "output doesnt match the expected value");
				return output;
			}
		
			function verifyArchCallToVerifyCOAOwnershipProof(bool expected, address arg0 , bytes32 arg1 , bytes memory arg2 ) public view returns (bool){
				(bool ok, bytes memory data) = cadenceArch.staticcall(abi.encodeWithSignature("verifyCOAOwnershipProof(address,bytes32,bytes)", arg0, arg1, arg2));
				require(ok, "unsuccessful call to arch");
				bool output = abi.decode(data, (bool));
				require(expected == output, "output doesnt match the expected value");
				return output;
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
				"name": "cadenceArch",
				"outputs": [
					{
						"internalType": "address",
						"name": "",
						"type": "address"
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
			},
			{
				"inputs": [
					{
						"internalType": "uint64",
						"name": "expected",
						"type": "uint64"
					}
				],
				"name": "verifyArchCallToFlowBlockHeight",
				"outputs": [
					{
						"internalType": "uint64",
						"name": "",
						"type": "uint64"
					}
				],
				"stateMutability": "view",
				"type": "function"
			},
			{
				"inputs": [
					{
						"internalType": "bool",
						"name": "expected",
						"type": "bool"
					},
					{
						"internalType": "address",
						"name": "arg0",
						"type": "address"
					},
					{
						"internalType": "bytes32",
						"name": "arg1",
						"type": "bytes32"
					},
					{
						"internalType": "bytes",
						"name": "arg2",
						"type": "bytes"
					}
				],
				"name": "verifyArchCallToVerifyCOAOwnershipProof",
				"outputs": [
					{
						"internalType": "bool",
						"name": "",
						"type": "bool"
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
	DeployContract(t, types.NewAddress(gethCommon.Address{}), tc, led, flowEVMRootAddress)
	f(tc)
}

func DeployContract(t testing.TB, caller types.Address, tc *TestContract, led atree.Ledger, flowEVMRootAddress flow.Address) {
	// deploy contract
	e := emulator.NewEmulator(led, flowEVMRootAddress)

	blk, err := e.NewBlockView(types.NewDefaultBlockContext(2))
	require.NoError(t, err)

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
