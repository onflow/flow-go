access(all)
contract Flex {

    /// A Flex address is an EVM-compatible address
    access(all)
    struct FlexAddress {

        /// Constructs a new Flex address from the given byte representation.
        init(bytes: [UInt8; 20])

        /// Bytes of the address
        access(all)
        let bytes: [UInt8; 20]
    }

    access(all)
    struct Balance {

        /// Constructs a new
        init(flowAmount: UFix64)

        /// Returns the balance in FLOW
        access(all)
        fun toFLOW(): UFix64

        /// Returns the balance in terms of atto-FLOW.
        /// Atto-FLOW is the smallest denomination of FLOW inside Flex.
        access(all)
        fun toAttoFlow(): UInt64
    }

    access(all)
    resource FlowOwnedAccount {

        /// The address of the owned Flex account
        access(all)
        fun address(): Flex.FlexAddress

        /// Deposits the given vault into the Flex account's balance
        access(all)
        fun deposit(from: @FlowToken.Vault)

        /// Withdraws the balance from the Flex account's balance
        access(all)
        fun withdraw(balance: Flex.Balance): @FlowToken.Vault

        /// Deploys a contract to the Flex environment.
        /// Returns the address of the newly deployed.
        access(all)
        fun deploy(code: [UInt8], gaslimit: UInt64, value: Flex.Balance): Flex.FlexAddress

        /// Calls a function with the given data.
        /// The execution is limited by the given amount of gas.
        access(all)
        fun call(
            to: Flex.FlexAddress,
            data: [UInt8],
            gaslimit: UInt64,
            value: Flex.Balance
        ): [UInt8]
    }

    /// Creates a new Flex account
    access(all)
    fun createFlowOwnedAccount(): @Flex.FlowOwnedAccount

    /// Runs an a RLP-encoded Flex transaction,
    /// deducts the gas fees and deposits them into the
    /// provided coinbase address.
    ///
    /// Returns true if the transaction was successful,
    /// and returns false otherwise.
    access(all)
    fun run(tx: [UInt8], coinbase: Flex.FlexAddress): Bool
}
