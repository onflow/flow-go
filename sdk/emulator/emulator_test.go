package emulator_test

// addTwoScript runs a script that adds 2 to a value.
const addTwoScript = `
	fun main(account: Account) {
		account.storage[Int] = (account.storage[Int] ?? 0) + 2
	}
`

const sampleCall = `
	fun main(): Int {
		return getAccount("%s").storage[Int] ?? 0
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
