transaction(newWeights: {UInt64: UInt64}, path: StoragePath) {
    prepare(signer: AuthAccount) {
        signer.load<{UInt64: UInt64}>(from: path)
        signer.save(newWeights, to: path)
    }
}
