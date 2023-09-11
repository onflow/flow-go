transaction(restricted: Bool, path: StoragePath) {
	prepare(signer: auth(Storage) &Account) {
		signer.load<Bool>(from: path)
		signer.save(restricted, to: path)
	}
}
