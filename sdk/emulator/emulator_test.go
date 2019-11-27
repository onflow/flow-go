package emulator_test

// addTwoScript runs a script that adds 2 to a value.
const addTwoScript = `
	transaction {
	  prepare(signer: Account) {
		signer.storage[Int] = (signer.storage[Int] ?? 0) + 2
	  }
	  execute {}
	}
`

const sampleCall = `
	pub fun main(): Int {
		return getAccount(0x%s).storage[Int] ?? 0
	}
`

// Returns a nonce value that is guaranteed to be unique.
var getNonce = func() func() uint64 {
	var nonce uint64
	return func() uint64 {
		nonce++
		return nonce
	}
}()
