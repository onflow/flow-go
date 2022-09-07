transaction(restricted: Bool, path: StoragePath) {
	prepare(signer: AuthAccount) {
		signer.load<Bool>(from: path)
		signer.save(restricted, to: path)
	}
}
