transaction(newLimit: UInt64, path: StoragePath) {
    prepare(signer: AuthAccount) {
        signer.load<UInt64>(from: path)
        signer.save(newLimit, to: path)
    }
}
