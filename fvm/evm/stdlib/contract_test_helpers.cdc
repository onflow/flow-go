    /// Stores a value to an address' storage slot.
    access(all)
    fun store(target: EVM.EVMAddress, slot: String, value: String) {
        InternalEVM.store(target: target.bytes, slot: slot, value: value)
    }

    /// Loads a storage slot from an address.
    access(all)
    fun load(target: EVM.EVMAddress, slot: String): [UInt8] {
        return InternalEVM.load(target: target.bytes, slot: slot)
    }

    /// Runs a transaction by setting the call's `msg.sender` to be the `from` address.
    access(all)
    fun runTxAs(
        from: EVM.EVMAddress,
        to: EVM.EVMAddress,
        data: [UInt8],
        gasLimit: UInt64,
        value: EVM.Balance,
    ): Result {
        return InternalEVM.call(
            from: from.bytes,
            to: to.bytes,
            data: data,
            gasLimit: gasLimit,
            value: value.attoflow
        ) as! Result
    }
