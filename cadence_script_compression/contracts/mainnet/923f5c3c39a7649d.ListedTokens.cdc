pub contract ListedTokens {
  /****** Events ******/
  pub event TokenAdded(key: String, name: String, displayName: String, symbol: String, address: Address)
  pub event TokenUpdated(key: String, name: String, displayName: String, symbol: String, address: Address)
  pub event TokenRemoved(key: String)

  /****** Contract Variables ******/
  access(contract) var _tokens: { String: TokenInfo }

  pub let AdminStoragePath: StoragePath

  /****** Composite Type Definitions ******/
  pub struct TokenInfo {
    pub let name: String
    pub let displayName: String
    pub let symbol: String
    pub let address: Address
    pub let vaultPath: String
    pub let receiverPath: String
    pub let balancePath: String

    init(name: String, displayName: String, symbol: String, address: Address, vaultPath: String, receiverPath: String, balancePath: String) {
      self.name = name
      self.displayName = displayName
      self.symbol = symbol
      self.address = address
      self.vaultPath = vaultPath
      self.receiverPath = receiverPath
      self.balancePath = balancePath
    }
  }

  pub resource Admin {
    pub fun addToken(name: String, displayName: String, symbol: String, address: Address, vaultPath: String, receiverPath: String, balancePath: String) {
      var key = name.concat(".").concat(address.toString())

      if (ListedTokens.tokenExists(key: key)) {
        return
      }

      ListedTokens._tokens[key] = TokenInfo(
        name: name,
        displayName: displayName,
        symbol: symbol,
        address: address,
        vaultPath: vaultPath,
        receiverPath: receiverPath,
        balancePath: balancePath
      )

      emit TokenAdded(key: key, name: name, displayName: displayName, symbol: symbol, address: address)
    }

    pub fun updateToken(name: String, displayName: String, symbol: String, address: Address, vaultPath: String, receiverPath: String, balancePath: String) {
      var key = name.concat(".").concat(address.toString())

      ListedTokens._tokens[key] = TokenInfo(
        name: name,
        displayName: displayName,
        symbol: symbol,
        address: address,
        vaultPath: vaultPath,
        receiverPath: receiverPath,
        balancePath: balancePath
      )

      emit TokenUpdated(key: key, name: name, displayName: displayName, symbol: symbol, address: address)
    }

    pub fun removeToken(key: String) {
      ListedTokens._tokens.remove(key: key)

      emit TokenRemoved(key: key)
    }
  }

  /****** Methods ******/
  pub fun tokenExists(key: String): Bool {
    return self._tokens.containsKey(key)
  }

  pub fun getTokens(): [TokenInfo] {
    return self._tokens.values
  }

  init () {
    self._tokens = {}
    self.AdminStoragePath = /storage/bloctoSwapListedTokensAdmin

    let admin <- create Admin()
    self.account.save(<-admin, to: self.AdminStoragePath)
  }
}