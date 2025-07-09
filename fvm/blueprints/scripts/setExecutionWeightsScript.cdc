transaction(newWeights: {UInt64: UInt64}, path: StoragePath) {
    prepare(signer: auth(Storage) &Account) {
        signer.storage.load<{UInt64: UInt64}>(from: path)
        signer.storage.save(newWeights, to: path)
    }
}
