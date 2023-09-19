access(all) contract Flex {

    /// A Flex address is an evm-compatible address
    access(all) struct Address {
        /// Bytes of the address
        access(all) let bytes: [UInt8; 20]
    }

    /// Run runs a flex transaction, deducts the gas fees and deposits them into the 
    /// provided coinbase address 
		access(all) view fun run(tx: [UInt8], coinbase: Flex.Address)
}