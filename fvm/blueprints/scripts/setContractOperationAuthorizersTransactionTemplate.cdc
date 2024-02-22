transaction(addresses: [Address], path: StoragePath) {
	prepare(signer: auth(Storage) &Account) {
		signer.storage.load<[Address]>(from: path)
		signer.storage.save(addresses, to: path)
	}
}
