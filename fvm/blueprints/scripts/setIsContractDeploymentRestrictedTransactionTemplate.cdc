transaction(restricted: Bool, path: StoragePath) {
	prepare(signer: auth(Storage) &Account) {
		signer.storage.load<Bool>(from: path)
		signer.storage.save(restricted, to: path)
	}
}
