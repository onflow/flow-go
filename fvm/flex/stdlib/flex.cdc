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

    /// Run runs a flex transaction, deducts the gas fees and deposits them into the
    /// provided coinbase address
    access(all)
    fun run(tx: [UInt8], coinbase: Flex.FlexAddress)
}
