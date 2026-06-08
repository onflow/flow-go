// SPDX-License-Identifier: GPL-3.0

pragma solidity >=0.7.0 <0.9.0;

contract FlowTransactionScheduler {
    address public constant cadenceArch =
        0x0000000000000000000000010000000000000001;

    address public authorizedCOA;

    error InvalidCOAAddress();
    error TransferFailed();

    constructor() {}

    function setAuthorizedCOA(address coaAddress) public {
        if (coaAddress == address(0)) revert InvalidCOAAddress();

        authorizedCOA = coaAddress;
    }

    function scheduleTransaction(
        uint256 timestamp,
        uint8 priority,
        uint64 executionEffort,
        address contractAddress,
        bytes calldata args
    ) external payable returns (uint64) {
        // 1. send fees to `authorizedCOA`
        // Native FLOW: send via low-level call
        (bool success, ) = authorizedCOA.call{value: msg.value}("");
        if (!success) revert TransferFailed();

        // 2. call the cadenceArch precompile
        (bool ok, bytes memory data) = cadenceArch.staticcall(
            abi.encodeWithSignature(
                "scheduleTransaction(uint256,uint8,uint64,address,uint256,address,bytes)",
                timestamp,
                priority,
                executionEffort,
                msg.sender,
                msg.value,
                contractAddress,
                args
            )
        );
        require(
            ok,
            "unsuccessful call to CadenceArch.scheduleTransaction() function"
        );
        uint64 txId = abi.decode(data, (uint64));

        return txId;
    }

    function estimate(
        uint256 timestamp,
        uint8 priority,
        uint64 executionEffort,
        address contractAddress,
        bytes calldata args
    ) external view returns (uint256) {
        (bool ok, bytes memory data) = cadenceArch.staticcall(
            abi.encodeWithSignature(
                "estimate(uint256,uint8,uint64,address,bytes)",
                timestamp,
                priority,
                executionEffort,
                contractAddress,
                args
            )
        );
        require(ok, "unsuccessful call to CadenceArch.estimate() function");
        uint256 fees = abi.decode(data, (uint256));

        return fees;
    }

    function getTransactionStatus(uint64 txId) external view returns (uint8) {
        (bool ok, bytes memory data) = cadenceArch.staticcall(
            abi.encodeWithSignature("getTransactionStatus(uint64)", txId)
        );
        require(
            ok,
            "unsuccessful call to CadenceArch.getTransactionStatus() function"
        );
        uint8 txStatus = abi.decode(data, (uint8));

        return txStatus;
    }

    function cancelTransaction(uint64 txId) external {
        (bool ok, ) = cadenceArch.staticcall(
            abi.encodeWithSignature("cancelTransaction(uint64)", txId)
        );
        require(
            ok,
            "unsuccessful call to CadenceArch.cancelTransaction() function"
        );
    }
}
