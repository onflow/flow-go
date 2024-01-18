package handler

var COAContractBytes = []byte("608060405234801561000f575f80fd5b50610e1e8061001d5f395ff3fe608060405260043610610072575f3560e01c80631626ba7e1161004d5780631626ba7e1461016b578063bc197c81146101a7578063d0d250bd146101e3578063f23a6e611461020d576100c7565b806223de29146100cb57806301ffc9a7146100f3578063150b7a021461012f576100c7565b366100c7573373ffffffffffffffffffffffffffffffffffffffff167fb496ce2863a72c02f90deb616c638c76948f3aff803140881c602775c05fdf3a346040516100bd91906105a5565b60405180910390a2005b5f80fd5b3480156100d6575f80fd5b506100f160048036038101906100ec91906106b4565b610249565b005b3480156100fe575f80fd5b50610119600480360381019061011491906107d3565b610253565b6040516101269190610818565b60405180910390f35b34801561013a575f80fd5b5061015560048036038101906101509190610831565b6103f3565b60405161016291906108c4565b60405180910390f35b348015610176575f80fd5b50610191600480360381019061018c9190610a48565b610407565b60405161019e91906108c4565b60405180910390f35b3480156101b2575f80fd5b506101cd60048036038101906101c89190610af7565b610554565b6040516101da91906108c4565b60405180910390f35b3480156101ee575f80fd5b506101f761056b565b6040516102049190610bdd565b60405180910390f35b348015610218575f80fd5b50610233600480360381019061022e9190610bf6565b610578565b60405161024091906108c4565b60405180910390f35b5050505050505050565b5f7f4e2312e0000000000000000000000000000000000000000000000000000000007bffffffffffffffffffffffffffffffffffffffffffffffffffffffff1916827bffffffffffffffffffffffffffffffffffffffffffffffffffffffff1916148061031d57507f150b7a02000000000000000000000000000000000000000000000000000000007bffffffffffffffffffffffffffffffffffffffffffffffffffffffff1916827bffffffffffffffffffffffffffffffffffffffffffffffffffffffff1916145b8061038457507e23de29000000000000000000000000000000000000000000000000000000007bffffffffffffffffffffffffffffffffffffffffffffffffffffffff1916827bffffffffffffffffffffffffffffffffffffffffffffffffffffffff1916145b806103ec57507f01ffc9a7000000000000000000000000000000000000000000000000000000007bffffffffffffffffffffffffffffffffffffffffffffffffffffffff1916827bffffffffffffffffffffffffffffffffffffffffffffffffffffffff1916145b9050919050565b5f63150b7a0260e01b905095945050505050565b5f805f6801000000000000000173ffffffffffffffffffffffffffffffffffffffff16858560405160240161043d929190610d15565b6040516020818303038152906040527eb997bd000000000000000000000000000000000000000000000000000000007bffffffffffffffffffffffffffffffffffffffffffffffffffffffff19166020820180517bffffffffffffffffffffffffffffffffffffffffffffffffffffffff83818316178352505050506040516104c69190610d7d565b5f60405180830381855afa9150503d805f81146104fe576040519150601f19603f3d011682016040523d82523d5f602084013e610503565b606091505b509150915081610511575f80fd5b5f818060200190518101906105269190610dbd565b9050801561054057631626ba7e60e01b935050505061054e565b63ffffffff60e01b93505050505b92915050565b5f63bc197c8160e01b905098975050505050505050565b6801000000000000000181565b5f63f23a6e6160e01b90509695505050505050565b5f819050919050565b61059f8161058d565b82525050565b5f6020820190506105b85f830184610596565b92915050565b5f604051905090565b5f80fd5b5f80fd5b5f73ffffffffffffffffffffffffffffffffffffffff82169050919050565b5f6105f8826105cf565b9050919050565b610608816105ee565b8114610612575f80fd5b50565b5f81359050610623816105ff565b92915050565b6106328161058d565b811461063c575f80fd5b50565b5f8135905061064d81610629565b92915050565b5f80fd5b5f80fd5b5f80fd5b5f8083601f84011261067457610673610653565b5b8235905067ffffffffffffffff81111561069157610690610657565b5b6020830191508360018202830111156106ad576106ac61065b565b5b9250929050565b5f805f805f805f8060c0898b0312156106d0576106cf6105c7565b5b5f6106dd8b828c01610615565b98505060206106ee8b828c01610615565b97505060406106ff8b828c01610615565b96505060606107108b828c0161063f565b955050608089013567ffffffffffffffff811115610731576107306105cb565b5b61073d8b828c0161065f565b945094505060a089013567ffffffffffffffff8111156107605761075f6105cb565b5b61076c8b828c0161065f565b92509250509295985092959890939650565b5f7fffffffff0000000000000000000000000000000000000000000000000000000082169050919050565b6107b28161077e565b81146107bc575f80fd5b50565b5f813590506107cd816107a9565b92915050565b5f602082840312156107e8576107e76105c7565b5b5f6107f5848285016107bf565b91505092915050565b5f8115159050919050565b610812816107fe565b82525050565b5f60208201905061082b5f830184610809565b92915050565b5f805f805f6080868803121561084a576108496105c7565b5b5f61085788828901610615565b955050602061086888828901610615565b94505060406108798882890161063f565b935050606086013567ffffffffffffffff81111561089a576108996105cb565b5b6108a68882890161065f565b92509250509295509295909350565b6108be8161077e565b82525050565b5f6020820190506108d75f8301846108b5565b92915050565b5f819050919050565b6108ef816108dd565b81146108f9575f80fd5b50565b5f8135905061090a816108e6565b92915050565b5f80fd5b5f601f19601f8301169050919050565b7f4e487b71000000000000000000000000000000000000000000000000000000005f52604160045260245ffd5b61095a82610914565b810181811067ffffffffffffffff8211171561097957610978610924565b5b80604052505050565b5f61098b6105be565b90506109978282610951565b919050565b5f67ffffffffffffffff8211156109b6576109b5610924565b5b6109bf82610914565b9050602081019050919050565b828183375f83830152505050565b5f6109ec6109e78461099c565b610982565b905082815260208101848484011115610a0857610a07610910565b5b610a138482856109cc565b509392505050565b5f82601f830112610a2f57610a2e610653565b5b8135610a3f8482602086016109da565b91505092915050565b5f8060408385031215610a5e57610a5d6105c7565b5b5f610a6b858286016108fc565b925050602083013567ffffffffffffffff811115610a8c57610a8b6105cb565b5b610a9885828601610a1b565b9150509250929050565b5f8083601f840112610ab757610ab6610653565b5b8235905067ffffffffffffffff811115610ad457610ad3610657565b5b602083019150836020820283011115610af057610aef61065b565b5b9250929050565b5f805f805f805f8060a0898b031215610b1357610b126105c7565b5b5f610b208b828c01610615565b9850506020610b318b828c01610615565b975050604089013567ffffffffffffffff811115610b5257610b516105cb565b5b610b5e8b828c01610aa2565b9650965050606089013567ffffffffffffffff811115610b8157610b806105cb565b5b610b8d8b828c01610aa2565b9450945050608089013567ffffffffffffffff811115610bb057610baf6105cb565b5b610bbc8b828c0161065f565b92509250509295985092959890939650565b610bd7816105ee565b82525050565b5f602082019050610bf05f830184610bce565b92915050565b5f805f805f8060a08789031215610c1057610c0f6105c7565b5b5f610c1d89828a01610615565b9650506020610c2e89828a01610615565b9550506040610c3f89828a0161063f565b9450506060610c5089828a0161063f565b935050608087013567ffffffffffffffff811115610c7157610c706105cb565b5b610c7d89828a0161065f565b92509250509295509295509295565b610c95816108dd565b82525050565b5f81519050919050565b5f82825260208201905092915050565b5f5b83811015610cd2578082015181840152602081019050610cb7565b5f8484015250505050565b5f610ce782610c9b565b610cf18185610ca5565b9350610d01818560208601610cb5565b610d0a81610914565b840191505092915050565b5f604082019050610d285f830185610c8c565b8181036020830152610d3a8184610cdd565b90509392505050565b5f81905092915050565b5f610d5782610c9b565b610d618185610d43565b9350610d71818560208601610cb5565b80840191505092915050565b5f610d888284610d4d565b915081905092915050565b610d9c816107fe565b8114610da6575f80fd5b50565b5f81519050610db781610d93565b92915050565b5f60208284031215610dd257610dd16105c7565b5b5f610ddf84828501610da9565b9150509291505056fea2646970667358221220c5a50c64c66e3bfb048a9c796b1fd390d35974be1136aed5e4c00cf2fc62c55164736f6c63430008160033")

var COAContractCode = `
	// SPDX-License-Identifier: UNLICENSED
	pragma solidity >=0.7.0 <0.9.0;
	interface IERC165 {
		function supportsInterface(bytes4 interfaceId) external view returns (bool);
	}
	interface ERC721TokenReceiver {
		function onERC721Received(
		address _operator,
		address _from,
		uint256 _tokenId,
		bytes calldata _data
		) external returns (bytes4);
	}
	interface ERC777TokensRecipient {
	function tokensReceived(
			address operator,
			address from,
			address to,
			uint256 amount,
			bytes calldata data,
			bytes calldata operatorData
		) external;
	}
	interface ERC1155TokenReceiver {
	function onERC1155Received(
			address _operator,
			address _from,
			uint256 _id,
			uint256 _value,
			bytes calldata _data
		) external returns (bytes4);
	function onERC1155BatchReceived(
			address _operator,
			address _from,
			uint256[] calldata _ids,
			uint256[] calldata _values,
			bytes calldata _data
		) external returns (bytes4);
	}
	contract COA is ERC1155TokenReceiver, ERC777TokensRecipient, ERC721TokenReceiver, IERC165 {
		address constant public cadenceArch = 0x0000000000000000000000010000000000000001;
		event FlowReceived(address indexed sender, uint256 value);
		receive() external payable  {
			emit FlowReceived(msg.sender, msg.value);
		}
		function supportsInterface(bytes4 id) external view virtual override returns (bool) {
			return
				id == type(ERC1155TokenReceiver).interfaceId ||
				id == type(ERC721TokenReceiver).interfaceId ||
				id == type(ERC777TokensRecipient).interfaceId ||
				id == type(IERC165).interfaceId;
		}

		function onERC1155Received(
			address,
			address,
			uint256,
			uint256,
			bytes calldata
		) external pure override returns (bytes4) {
			return 0xf23a6e61;
		}

		function onERC1155BatchReceived(
			address,
			address,
			uint256[] calldata,
			uint256[] calldata,
			bytes calldata
		) external pure override returns (bytes4) {
			return 0xbc197c81;
		}

		function onERC721Received(
			address,
			address,
			uint256,
			bytes calldata
		) external pure override returns (bytes4) {
			return 0x150b7a02;
		}

		function tokensReceived(
			address,
			address,
			address,
			uint256,
			bytes calldata,
			bytes calldata
		) external pure override {}

		function isValidSignature(
		bytes32 _hash,
		bytes memory _sig
	) external view virtual returns (bytes4){
		(bool ok, bytes memory data) = cadenceArch.staticcall(abi.encodeWithSignature("verifyCOAOwnershipProof(bytes32, bytes)", _hash, _sig));
		require(ok);
		bool output = abi.decode(data, (bool));
		if (output) {
			return 0x1626ba7e;
		}
		return 0xffffffff;
	}
	}
`

var COAContractABI = `
[
	{
		"anonymous": false,
		"inputs": [
			{
				"indexed": true,
				"internalType": "address",
				"name": "sender",
				"type": "address"
			},
			{
				"indexed": false,
				"internalType": "uint256",
				"name": "value",
				"type": "uint256"
			}
		],
		"name": "FlowReceived",
		"type": "event"
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
		"inputs": [
			{
				"internalType": "bytes32",
				"name": "_hash",
				"type": "bytes32"
			},
			{
				"internalType": "bytes",
				"name": "_sig",
				"type": "bytes"
			}
		],
		"name": "isValidSignature",
		"outputs": [
			{
				"internalType": "bytes4",
				"name": "",
				"type": "bytes4"
			}
		],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [
			{
				"internalType": "address",
				"name": "",
				"type": "address"
			},
			{
				"internalType": "address",
				"name": "",
				"type": "address"
			},
			{
				"internalType": "uint256[]",
				"name": "",
				"type": "uint256[]"
			},
			{
				"internalType": "uint256[]",
				"name": "",
				"type": "uint256[]"
			},
			{
				"internalType": "bytes",
				"name": "",
				"type": "bytes"
			}
		],
		"name": "onERC1155BatchReceived",
		"outputs": [
			{
				"internalType": "bytes4",
				"name": "",
				"type": "bytes4"
			}
		],
		"stateMutability": "pure",
		"type": "function"
	},
	{
		"inputs": [
			{
				"internalType": "address",
				"name": "",
				"type": "address"
			},
			{
				"internalType": "address",
				"name": "",
				"type": "address"
			},
			{
				"internalType": "uint256",
				"name": "",
				"type": "uint256"
			},
			{
				"internalType": "uint256",
				"name": "",
				"type": "uint256"
			},
			{
				"internalType": "bytes",
				"name": "",
				"type": "bytes"
			}
		],
		"name": "onERC1155Received",
		"outputs": [
			{
				"internalType": "bytes4",
				"name": "",
				"type": "bytes4"
			}
		],
		"stateMutability": "pure",
		"type": "function"
	},
	{
		"inputs": [
			{
				"internalType": "address",
				"name": "",
				"type": "address"
			},
			{
				"internalType": "address",
				"name": "",
				"type": "address"
			},
			{
				"internalType": "uint256",
				"name": "",
				"type": "uint256"
			},
			{
				"internalType": "bytes",
				"name": "",
				"type": "bytes"
			}
		],
		"name": "onERC721Received",
		"outputs": [
			{
				"internalType": "bytes4",
				"name": "",
				"type": "bytes4"
			}
		],
		"stateMutability": "pure",
		"type": "function"
	},
	{
		"inputs": [
			{
				"internalType": "bytes4",
				"name": "id",
				"type": "bytes4"
			}
		],
		"name": "supportsInterface",
		"outputs": [
			{
				"internalType": "bool",
				"name": "",
				"type": "bool"
			}
		],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [
			{
				"internalType": "address",
				"name": "",
				"type": "address"
			},
			{
				"internalType": "address",
				"name": "",
				"type": "address"
			},
			{
				"internalType": "address",
				"name": "",
				"type": "address"
			},
			{
				"internalType": "uint256",
				"name": "",
				"type": "uint256"
			},
			{
				"internalType": "bytes",
				"name": "",
				"type": "bytes"
			},
			{
				"internalType": "bytes",
				"name": "",
				"type": "bytes"
			}
		],
		"name": "tokensReceived",
		"outputs": [],
		"stateMutability": "pure",
		"type": "function"
	},
	{
		"stateMutability": "payable",
		"type": "receive"
	}
]

`
