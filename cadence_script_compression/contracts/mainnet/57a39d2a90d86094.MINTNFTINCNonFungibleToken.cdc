
import NonFungibleToken from 0x1d7e57aa55817448

pub contract MINTNFTINCNonFungibleToken: NonFungibleToken {

    pub var totalSupply: UInt64

  pub event ContractInitialized()
  pub event Withdraw(id: UInt64, from: Address?)
  pub event Deposit(id: UInt64, to: Address?)
  pub event NFTMinted(id: UInt64, name: String, symbol: String, description: String, collectionName: String, tokenURI: String, mintNFTId: String, mintNFTStandard: String, mintNFTVideoURI: String, mattelId: String , terms: String)
  pub event NFTDestroyed(id: UInt64)

  pub resource NFT: NonFungibleToken.INFT {
    pub let id: UInt64

    pub let name: String 
    pub let symbol: String
    pub let description: String
    pub let collectionName: String
    pub let tokenURI: String
    pub let mintNFTId: String
    pub let mintNFTStandard: String
    pub let mintNFTVideoURI: String
    pub let mattelId: String
    pub let terms: String
    pub let burnable: Bool


    init(initID: UInt64, _name: String, _symbol: String, _description: String, _collectionName: String, _tokenURI: String, _mintNFTId: String,_mintNFTStandard: String, _mintNFTVideoURI: String, _mattelId: String , _terms: String) {
      
      self.id = initID
   
      self.name = _name
      self.symbol = _symbol
      self.description = _description
      self.collectionName = _collectionName
      self.tokenURI = _tokenURI
      self.mintNFTId = _mintNFTId
      self.mintNFTStandard = _mintNFTStandard
      self.mintNFTVideoURI = _mintNFTVideoURI
      self.mattelId = _mattelId
      self.terms = _terms
      self.burnable = false;

      emit NFTMinted(id: self.id, name: self.name, symbol: self.symbol, description: self.description, collectionName: self.collectionName, tokenURI: self.tokenURI, mintNFTId: self.mintNFTId, mintNFTStandard: self.mintNFTStandard, mintNFTVideoURI: self.mintNFTVideoURI, mattelId: self.mattelId , terms: self.terms)

    }

     destroy() {
      emit NFTDestroyed(id: self.id)
    }
    }


   pub resource interface MINTNFTCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun batchDeposit(tokens: @NonFungibleToken.Collection)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowMINTNFT(id: UInt64): &MINTNFTINCNonFungibleToken.NFT? {

        post {
                (result == nil) || (result?.id == id): 
                    "Cannot borrow NFT reference: The ID of the returned reference is incorrect"
            }
        }
    }

    pub resource Collection: MINTNFTCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {

        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        init () {
            self.ownedNFTs <- {}
        }

        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("annot withdraw: NFT does not exist in the collection")
            emit Withdraw(id: token.id, from: self.owner?.address)
            return <-token
        }

        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @MINTNFTINCNonFungibleToken.NFT
            let id: UInt64 = token.id
            let oldToken <- self.ownedNFTs[id] <- token
            if self.owner?.address != nil {
                emit Deposit(id: id, to: self.owner?.address)
            }

            destroy oldToken
        }

        pub fun batchWithdraw(ids: [UInt64]): @NonFungibleToken.Collection {
            var batchCollection <- create Collection()
            
            for id in ids {
                batchCollection.deposit(token: <-self.withdraw(withdrawID: id))
            }
            
            return <-batchCollection
        }
        pub fun batchDeposit(tokens: @NonFungibleToken.Collection) {

            let keys = tokens.getIDs()

            for key in keys {
                self.deposit(token: <-tokens.withdraw(withdrawID: key))
            }

            destroy tokens
        }

        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }


        pub fun borrowMINTNFT(id: UInt64): &MINTNFTINCNonFungibleToken.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &MINTNFTINCNonFungibleToken.NFT
            } else {
                return nil
            }
        }

        destroy() {
            destroy self.ownedNFTs
        }
    }

  
    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <- create Collection()
    }


    pub resource MINTNFTMinter {

        pub fun mintNFT(recipient: &{MINTNFTCollectionPublic}, name: String, symbol: String, description: String, collectionName: String, tokenURI: String, mintNFTId: String, mintNFTStandard: String, mintNFTVideoURI: String, mattelId: String , terms: String) {


            var newNFT <- create NFT(initID: MINTNFTINCNonFungibleToken.totalSupply, _name: name, _symbol: symbol, _description: description, _collectionName: collectionName, _tokenURI: tokenURI, _mintNFTId: mintNFTId, _mintNFTStandard: mintNFTStandard, _mintNFTVideoURI: mintNFTVideoURI, _mattelId: mattelId , _terms: terms)

            recipient.deposit(token: <-newNFT)

            MINTNFTINCNonFungibleToken.totalSupply = MINTNFTINCNonFungibleToken.totalSupply + UInt64(1)
        }
    }

    init() {
       
        self.totalSupply = 0

     
        let collection <- create Collection()
        self.account.save(<-collection, to: /storage/MINTNFTCollection)

      
        self.account.link<&{MINTNFTCollectionPublic}>(
            /public/MINTNFTCollection,
            target: /storage/MINTNFTCollection
        )

    
        let minter <- create MINTNFTMinter()
        self.account.save(<-minter, to: /storage/MINTNFTMinter)

        emit ContractInitialized()
    }
}

