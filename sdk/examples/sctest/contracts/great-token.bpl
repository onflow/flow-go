struct interface NFT {
  fun id(): Int {
    post {
      result > 0
    }
  }
}

struct GreatNFT: NFT {
  let _id: Int

  fun id(): Int {
    return self._id
  }

  init(id: Int) {
    pre {
      id > 0
    }
    self._id = id
  }
}

struct GreatNFTMinter {
  var _nextID: Int

  fun maybeMint(): GreatNFT? {
    if self._nextID % 2 == 0 {
      self._nextID = self._nextID + 1
      return nil
    }
    var nextNFT = GreatNFT(id: self._nextID)
    self._nextID = self._nextID + 1
    return nextNFT
  }

  fun test(): Int {
    return 100
  }

  init() {
    self._nextID = 1
  }
}
