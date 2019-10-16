struct interface NFT {
  fun id(): Int {
    post {
      result > 0
    }
  } 
}

struct GreatNFT: NFT {
  let _id: Int
  let _special: Bool

  fun id(): Int { 
    return self._id
  }

  fun isSpecial(): Bool {
    return self._special
  }

  init(id: Int, isSpecial: Bool) { 
    pre {
      id > 0
    }
    self._id = id
    self._special = isSpecial
  }
}

struct GreatNFTMinter {
  var nextID: Int
  let specialMod: Int

  fun mint(): GreatNFT {
    var isSpecial = self.nextID % self.specialMod == 0
    let nft = GreatNFT(id: self.nextID, isSpecial: isSpecial)
    self.nextID = self.nextID + 1
    return nft
  }

  init(firstID: Int, specialMod: Int) {
    pre {
      firstID > 0
      specialMod > 1
    }
    self.nextID = firstID
    self.specialMod = specialMod
  }
}
