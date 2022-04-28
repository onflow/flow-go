// The KeyManager interface defines the functionality 
// used to securely add public keys to a Flow account
// through a Cadence contract.
//
// This interface is deployed once globally and implemented by 
// all token holders who wish to allow their keys to be managed
// by an administrator.
pub contract interface KeyManager {

  pub resource interface KeyAdder {
    pub let address: Address

    pub fun addPublicKey(_ publicKey: [UInt8])
  }
}
