pub contract GreatToken {

  pub resource interface NFT {
    pub fun id(): Int {
      post {
        result > 0
      }
    }
  }

  pub resource GreatNFT: NFT {
    priv let _id: Int
    priv let _special: Bool

    pub fun id(): Int {
      return self._id
    }

    pub fun isSpecial(): Bool {
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

  pub resource GreatNFTMinter {
    pub var nextID: Int
    pub let specialMod: Int

    pub fun mint(): @GreatNFT {
      var isSpecial = self.nextID % self.specialMod == 0
      let nft <- create GreatNFT(id: self.nextID, isSpecial: isSpecial)
      self.nextID = self.nextID + 1
      return <-nft
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

  pub fun createGreatNFTMinter(firstID: Int, specialMod: Int): @GreatNFTMinter {
    return <-create GreatNFTMinter(firstID: firstID, specialMod: specialMod)
  }
}
