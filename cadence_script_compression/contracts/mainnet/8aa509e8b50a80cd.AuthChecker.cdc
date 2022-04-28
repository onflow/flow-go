access(all) contract AuthChecker {

  pub event AddressNonceEvent(nonceString: String)

  pub fun checkLogin(_ nonceString: String): Void {
    pre {
      nonceString.length != 0: "Empty string"
    }

    emit AddressNonceEvent(nonceString: nonceString)   
  }
}
