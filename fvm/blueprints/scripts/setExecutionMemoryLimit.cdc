transaction(newLimit: UInt64, path: StoragePath) {
    prepare(signer: auth(Storage) &Account) {
        signer.storage.load<UInt64>(from: path)
        signer.storage.save(newLimit, to: path)
    }
}
