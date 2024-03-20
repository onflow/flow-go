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

    // bytes4(keccak256("onERC721Received(address,address,uint256,bytes)"))
    bytes4 constant internal ERC721ReceivedIsSupported = 0x150b7a02;

    // bytes4(keccak256("onERC1155Received(address,address,uint256,uint256,bytes)"))
    bytes4 constant internal ERC1155ReceivedIsSupported = 0xf23a6e61;

    // bytes4(keccak256("onERC1155BatchReceived(address,address,uint256[],uint256[],bytes)"))
    bytes4 constant internal ERC1155BatchReceivedIsSupported = 0xbc197c81;

    // bytes4(keccak256("isValidSignature(bytes32,bytes)")
    bytes4 constant internal ValidERC1271Signature = 0x1626ba7e;
    bytes4 constant internal InvalidERC1271Signature = 0xffffffff;

    receive() external payable  {
    }
    function supportsInterface(bytes4 id) external view virtual override returns (bool) {
        return
            id == type(ERC1155TokenReceiver).interfaceId ||
            id == type(ERC721TokenReceiver).interfaceId ||
            id == type(ERC777TokensRecipient).interfaceId ||
            id == type(IERC165).interfaceId;
    }

    function tokensReceived(
        address,
        address,
        address,
        uint256,
        bytes calldata,
        bytes calldata
    ) external pure override {}

    function onERC721Received(
        address,
        address,
        uint256,
        bytes calldata
    ) external pure override returns (bytes4) {
        return ERC721ReceivedIsSupported;
    }

    function onERC1155Received(
        address,
        address,
        uint256,
        uint256,
        bytes calldata
    ) external pure override returns (bytes4) {
        return ERC1155ReceivedIsSupported;
    }

    function onERC1155BatchReceived(
        address,
        address,
        uint256[] calldata,
        uint256[] calldata,
        bytes calldata
    ) external pure override returns (bytes4) {
        return ERC1155BatchReceivedIsSupported;
    }

    // ERC1271 requirement 
    function isValidSignature(
        bytes32 _hash,
        bytes memory _sig
    ) external view virtual returns (bytes4){
        (bool ok, bytes memory data) = cadenceArch.staticcall(abi.encodeWithSignature("verifyCOAOwnershipProof(address,bytes32,bytes)", address(this), _hash, _sig));
        require(ok);
        bool output = abi.decode(data, (bool));
        if (output) {
            return ValidERC1271Signature;
        }
        return InvalidERC1271Signature;
    }
}