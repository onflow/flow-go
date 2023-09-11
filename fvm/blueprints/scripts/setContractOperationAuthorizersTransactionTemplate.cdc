transaction(addresses: [Address], path: StoragePath) {
	prepare(signer: auth(Storage) &Account) {
		signer.load<[Address]>(from: path)
		signer.save(addresses, to: path)
	}
}
