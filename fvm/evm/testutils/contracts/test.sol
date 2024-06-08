// SPDX-License-Identifier: GPL-3.0

pragma solidity >=0.7.0 <0.9.0;

contract Storage {

    address constant public cadenceArch = 0x0000000000000000000000010000000000000001;
    event NewStore(address indexed caller, uint256 indexed value);

    error MyCustomError(uint value, string message);

    uint256 number;

    constructor() payable {
    }

    function store(uint256 num) public {
        number = num;
    }

    function storeWithLog(uint256 num) public {
        emit NewStore(msg.sender, num);
        number = num;
    }

    function storeButRevert(uint256 num) public {
        number = num;
        revert();
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

    function chainID() public view returns (uint256) {
        return block.chainid;
    }

    function destroy() public {
        selfdestruct(payable(msg.sender));
    }

    function assertError() public pure{
        require(false, "Assert Error Message");
    }

    function customError() public pure{
       revert MyCustomError(5, "Value is too low");
    }

    function verifyArchCallToRandomSource(uint64 height) public view returns (uint64) {
        (bool ok, bytes memory data) = cadenceArch.staticcall(abi.encodeWithSignature("getRandomSource(uint64)", height));
        require(ok, "unsuccessful call to arch ");
        uint64 output = abi.decode(data, (uint64));
        return output;
    }

    function verifyArchCallToRevertibleRandom() public view returns (uint64) {
        (bool ok, bytes memory data) = cadenceArch.staticcall(abi.encodeWithSignature("revertibleRandom()"));
        require(ok, "unsuccessful call to arch");
        uint64 output = abi.decode(data, (uint64));
        return output;
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
