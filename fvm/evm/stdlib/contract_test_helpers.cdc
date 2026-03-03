    /// Stores a value to an address' storage slot.
    access(all)
    fun store(target: EVM.EVMAddress, slot: String, value: String) {
        pre {
            slot.length == 64 || slot.length == 66:
                "EVM.store(): Invalid hex string length for EVM slot. The provided string is \(slot.length), but the length must be 64 or 66."
            value.length == 64 || value.length == 66:
                "EVM.store(): Invalid hex string length for EVM value. The provided string is \(value.length), but the length must be 64 or 66."
        }
        InternalEVM.store(target: target.bytes, slot: slot, value: value)
    }

    /// Loads a storage slot from an address.
    access(all)
    fun load(target: EVM.EVMAddress, slot: String): [UInt8] {
        pre {
            slot.length == 64 || slot.length == 66:
                "EVM.load(): Invalid hex string length for EVM slot. The provided string is \(slot.length), but the length must be 64 or 66."
        }
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
