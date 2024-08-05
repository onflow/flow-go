// SPDX-License-Identifier: UNLICENSED

pragma solidity >=0.7.0 <0.9.0;

contract Factory {
    constructor() payable {} 

    function deploy(bytes32 salt) public returns (address) {
        new Deployable{salt: salt}();
        return _getCreate2Address(salt, keccak256(abi.encodePacked(type(Deployable).creationCode)));
    }

    function deployAndDestroy(bytes32 salt) public {
        Deployable dep = new Deployable{salt: salt}();
        dep.destroy(address(this));
    }

    function depositAndDeploy(bytes32 salt, uint256 amount) public {
        address addr = _getCreate2Address(salt, keccak256(abi.encodePacked(type(Deployable).creationCode)));
        bool success;
        assembly {
            success := call(gas(), addr, amount, 0, 0, 0, 0)
        }
        require(success);
        new Deployable{salt: salt}();
    }

    function depositDeployAndDestroy(bytes32 salt, uint256 amount) public {
        address addr = _getCreate2Address(salt, keccak256(abi.encodePacked(type(Deployable).creationCode)));
        bool success;
        assembly {
            success := call(gas(), addr, amount, 0, 0, 0, 0)
        }
        require(success);
        Deployable dep = new Deployable{salt: salt}();
        dep.destroy(address(this));
    }

    function _getCreate2Address(bytes32 salt, bytes32 codeHash) internal view returns (address) {
        return address(uint160(uint256(keccak256(abi.encodePacked(bytes1(0xff), address(this), salt, codeHash)))));
    }
}

contract Deployable { 
    uint256 number;
    constructor() payable {}
    function set(uint256 num) public {
        number = num;
    }
    function get() public view returns (uint256){
        return number;
    }
    function destroy(address etherDestination) external {
        selfdestruct(payable(etherDestination));
    }
}