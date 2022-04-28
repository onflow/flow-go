pub contract ListedPairs {
  /****** Events ******/
  pub event PairAdded(key: String, name: String, token0: String, token1: String, address: Address)
  pub event PairUpdated(key: String)
  pub event PairRemoved(key: String)

  /****** Contract Variables ******/
  access(contract) var _pairs: { String: PairInfo }

  pub let AdminStoragePath: StoragePath

  /****** Composite Type Definitions ******/
  pub struct PairInfo {
    pub let name: String
    pub let token0: String
    pub let token1: String
    pub let address: Address
    pub var liquidityToken: String?

    init(name: String, token0: String, token1: String, address: Address, liquidityToken: String?) {
      self.name = name
      self.token0 = token0
      self.token1 = token1
      self.address = address
      self.liquidityToken = liquidityToken
    }

    pub fun update(liquidityToken: String?) {
      self.liquidityToken = liquidityToken ?? self.liquidityToken
    }
  }

  pub resource Admin {
    pub fun addPair(name: String, token0: String, token1: String, address: Address, liquidityToken: String?) {
      var key = name.concat(".").concat(address.toString())

      if (ListedPairs.pairExists(key: key)) {
        return
      }

      ListedPairs._pairs[key] = PairInfo(
        name: name,
        token0: token0,
        token1: token1,
        address: address, 
        liquidityToken: liquidityToken,
      )

      emit PairAdded(key: key, name: name, token0: token0, token1: token1, address: address)
    }

    pub fun updatePair(name: String, address: Address, liquidityToken: String?) {
      var key = name.concat(".").concat(address.toString())
      ListedPairs._pairs[key]!.update(liquidityToken: liquidityToken)

      emit PairUpdated(key: key)
    }

    pub fun removePair(key: String) {
      ListedPairs._pairs.remove(key: key)

      emit PairRemoved(key: key)
    }
  }

  /****** Methods ******/
  pub fun pairExists(key: String): Bool {
    return self._pairs.containsKey(key)
  }

  pub fun getPairs(): [PairInfo] {
    return self._pairs.values
  }

  init () {
    self._pairs = {}
    self.AdminStoragePath = /storage/bloctoSwapListedPairsAdmin

    let admin <- create Admin()
    self.account.save(<-admin, to: self.AdminStoragePath)
  }
}
