/**

# Contract: FantastecSwapData

## Description

The purpose of this contract is to provide a central location to hold and maintain metadata about FantastecSwap's Cards and Collections.

Collections represent a themed set of Cards, as indicated on their attributes.
Collections have 0 or more Cards associated with them.

Collections

Cards represent an individual item or moment of interest - a digital card of a player or stadium, a video moment, a VR scene, or access to other resources.

An NFT will be minted against individual Card.

*/
// import NonFungibleToken from "../../standards/contracts/NonFungibleToken.cdc"


pub contract FantastecSwapData {

  /** EVENTS **/
  // Contract Events
  pub event ContractInitialized()

  // Card Events
  pub event CardCreated(item: FantastecSwapData.CardData)
  pub event CardUpdated(item: FantastecSwapData.CardData)
  pub event CardDeactivated(id: UInt64)
  pub event AddedEditionMintVolume(item: FantastecSwapData.CardData)

  // CardCollection Events
  pub event CardCollectionCreated(item: FantastecSwapData.CardCollectionData)
  pub event CardCollectionUpdated(item: FantastecSwapData.CardCollectionData)
  pub event CardCollectionDeactivated(id: UInt64)

  /** CONTRACT LEVEL PROPERTIES **/
  access(self) var cardCollectionData: {UInt64: CardCollectionData}
  access(self) var nextCardCollectionId: UInt64
  access(self) var cardData: {UInt64: CardData}
  access(self) var nextCardId: UInt64
  access(self) var defaultRoyaltyAddress: Address

  /** CONTRACT LEVEL RESOURCES */
  pub let AdminStoragePath: StoragePath

  pub struct Royalty {
    // TODO might address instead by a capability to deposit FUSD into an account?
    pub let address: Address;
    pub let percentage: UFix64;
    init(
      _ address: Address,
      _ percentage: UFix64,
    ){
      pre {
        percentage <= 100.0: "percentage cannot be higher than 100"
      }
      self.address = address;
      self.percentage = percentage;
    }
  }

  /** CONTRACT LEVEL STRUCTS */
  pub struct CardCollectionData {
    pub var id: UInt64
    pub var appId: String
    pub var title: String
    pub var shortTitle: String
    pub var description: String
    pub var level: String
    pub var metadata: {String: String}
    pub var marketplaceFee: UFix64
    pub var royalties : [Royalty]

    // when isDeactivated is true the collection is considered soft deleted
    // eg if a mistake was made on the collection isDeactivated will signify
    // never to use this collection under most circumstances
    // * Collection marked as {isDeactivated:true} and cannot be accessed through "getAll"-like functions
    // * but can be referenced through "getById"-like function
    // * no actions can be performed on the collection nor it’s cards
    pub var isDeactivated: Bool

    access(contract) fun deactivate(){
      self.isDeactivated = true
    }

    access(contract) fun save(){
      FantastecSwapData.cardCollectionData[self.id] = self
    }
 
    init(
      appId: String, 
      title: String, 
      shortTitle: String, 
      description: String, 
      level: String,
      metadata: {String: String},
      marketplaceFee: UFix64,
      id: UInt64?, // if nil, nextCardCollectionId is used
      ){
      pre {
        appId.length != 0: "appId cannot be empty"
        title.length != 0: "title cannot be empty"
        shortTitle.length != 0: "shortTitle cannot be empty"
        description.length != 0: "description cannot be empty"
        level.length != 0: "level cannot be empty"
        marketplaceFee <= 100.0: "marketplaceFee cannot be more than 100"
      }

      self.appId = appId
      self.title = title
      self.shortTitle = shortTitle
      self.description = description
      self.level = level
      self.marketplaceFee = marketplaceFee
      self.metadata = metadata

      // add a default Royalty
      var address: Address = FantastecSwapData.defaultRoyaltyAddress
      var percentage: UFix64 = 100.0
      var fantastecRoyalty = Royalty(address, percentage);
      self.royalties = [fantastecRoyalty] // the sum % of self.royalties MUST ALWAYS BE 100

      // set the id
      if (id == nil){
        self.id = FantastecSwapData.nextCardCollectionId
        FantastecSwapData.nextCardCollectionId =  FantastecSwapData.nextCardCollectionId + (1 as UInt64)
      } else {
        self.id = id!
      }

      // set locks
      self.isDeactivated = false
    }
  }

  pub struct CardData {
    pub var id: UInt64                  // a unique sequential number
    pub var name: String                // Card name
    pub var level: String               // Level, one of 15 different levels
    pub var cardType: String            // Currently either [Player, Lineup]??
    pub var aspectRatio: String         // ??
    pub var metadata: {String: String}  // a data structure determined by cardType
    pub var cardCollectionId: UInt64    // The id of the collection to which this card belongs

    // when isDeactivated is true the card is considered soft deleted
    // eg if a mistake was made on the card isDeactivated will signify
    // never to use this card under most circumstances
    // * Card is marked as {isDeactivated:true} and cannot be accessed through "getAll"-like functions
    // * but can be referenced through "getById"-like function
    // * no actions can be performed on the card
    pub var isDeactivated: Bool

    // The maxmimum number of NFTs that can be minted per edition.
    // Admin may add a new entry to editionMintVolume, but not change any entries already present.
    // All other users have READONLY access to this property.
    // For example, {1:70} means 1st edition, max 70 cards mintable.
    // As soon as a later edition is added to the object, any previous editions are locked from minting
    // TODO: function to implement this
    pub var editionMintVolume: {UInt64: UInt64}
    
    // Present number of NFTs minting by edition
    pub var editionTotalSupply: {UInt64: UInt64}

    // determine if an edition has ceased being minted, meaning all unminted numbers have been "burnt",
    // increasing the scarcity for all minted cards
    pub var editionHasCeased: {UInt64: Bool}

    pub var editionNextMintNumber: {UInt64: UInt64}

    init(
      name: String, 
      level: String, 
      cardType: String,
      aspectRatio: String,
      metadata: {String: String},
      cardCollectionId: UInt64,
      id: UInt64?,
    ){
      pre {
        name.length > 0: "name cannot be empty"
        level.length > 0: "level cannot be empty"
        cardType.length > 0: "cardType cannot be empty"
        aspectRatio.length > 0: "aspectRatio cannot be empty"
        FantastecSwapData.cardCollectionData[cardCollectionId] != nil: "cannot create cardData when cardCollectionId does not exist"
      }

      let cardCollection: CardCollectionData = FantastecSwapData.cardCollectionData[cardCollectionId]!
      if (cardCollection.isDeactivated){
        panic("cannot create cardData when cardCollectionId is inactive")
      }

      self.name = name
      self.level = level
      self.cardType = cardType
      self.aspectRatio = aspectRatio
      self.cardCollectionId = cardCollectionId
      self.editionMintVolume = {}
      self.editionTotalSupply = {}
      self.editionNextMintNumber = {}
      self.editionHasCeased = {}
      self.isDeactivated = false
      self.metadata = metadata

      // set the id
      if (id == nil){
        self.id = FantastecSwapData.nextCardId
        FantastecSwapData.nextCardId = FantastecSwapData.nextCardId + (1 as UInt64)
      } else {
        self.id = id!
      }
    }
    access(contract) fun save(){
      FantastecSwapData.cardData[self.id] = self
    }
    access(contract) fun deactivate(){
      // TODO: should this be blocked if ceased, exhausted or nfts self.editionTotalSupply[edition]>0 ?
      self.isDeactivated = true
    }
    access(contract) fun cease(edition: UInt64){
     // TODO: should this be blocked for any reasons?
      self.editionHasCeased[edition] = true
    }
    access(contract) fun addEditionMintVolume(edition: UInt64, mintVolume: UInt64) {
      if (self.editionMintVolume[edition] != nil){
        panic("Cannot change an existing edition's mintVolume")
      }

      // set the maximum mint number
      self.editionMintVolume[edition] = mintVolume

      // set total minted to zero
      self.editionTotalSupply[edition] = 0

      // set next mint number to 1 (not zero based)
      self.editionNextMintNumber[edition] = 1

      // this edition has not ceased
      self.editionHasCeased[edition] = false

      // cease the previous edition
      if (edition > 1) {
        self.cease(edition: edition - 1)
      }
    }
    access(contract) fun updateEditionMintVolume(edition: UInt64, mintVolume: UInt64) {
      if (self.editionMintVolume[edition] == nil){
        panic("Cannot update as edition does not exist")
      }
      if (self.editionTotalSupply[edition]! > mintVolume){
        panic("Cannot update as editionTotalSupply > mintVolume given")
      }
      if (self.editionHasCeased[edition]!) {
        panic("Cannot up as edition has ceased")
      }
      self.editionMintVolume[edition] = mintVolume
    }
    access(contract) fun bumpEditionTotalSupply(_ edition: UInt64) {
      self.editionTotalSupply[edition] = self.editionTotalSupply[edition]! + (1 as UInt64)
    }
    access(contract) fun bumpEditionNextMintNumber(_ edition: UInt64){
      self.editionNextMintNumber[edition] = self.editionNextMintNumber[edition]! + (1 as UInt64)
    }
    pub fun hasEditionBeenExhausted(_ edition: UInt64): Bool{
      if (self.editionTotalSupply[edition]! >= self.editionMintVolume[edition]!) {
        return true
      }
      return false
    }
    pub fun isEditionCurrent(_ edition: UInt64): Bool {
      if (self.editionMintVolume.keys[self.editionMintVolume.length-1] == edition) {
        return true
      }
      return false
    }
    pub fun hasEditionCeased(_ edition: UInt64): Bool {
      if (self.editionHasCeased[edition] == true) {
        return true
      }
      return false
    }
    pub fun isMintable(edition: UInt64): String {
      if (self.isDeactivated) { return "Card deactivated" }
      if (!self.isEditionCurrent(edition)) { return "not the current edition" }
      if (self.hasEditionBeenExhausted(edition)) { return "edition is exhausted" }
      if (self.hasEditionCeased(edition)) { return "edition has ceased" }
      return "yes"
    }
  }

  pub resource Admin {
    pub fun setDefaultRoyaltiesAccount(_ address: Address){
      FantastecSwapData.defaultRoyaltyAddress = address;
    }
    pub fun getDefaultRoyaltiesAccount(): Address {
      return FantastecSwapData.defaultRoyaltyAddress;
    }

    // Manage CardCollection functions
    pub fun addCardCollection(
      appId: String, 
      title: String, 
      shortTitle: String, 
      description: String, 
      level: String,
      metadata: {String: String},
      marketplaceFee: UFix64,
    ): CardCollectionData {

      var newCardCollection: CardCollectionData = CardCollectionData(
        appId: appId,
        title: title,
        shortTitle: shortTitle,
        description: description,
        level: level,
        metadata: metadata,
        marketplaceFee: marketplaceFee,
        id: nil,
      )

      newCardCollection.save()

      emit CardCollectionCreated(item: newCardCollection)

      return newCardCollection
    }
    pub fun updateCardCollectionById(
      appId: String, 
      title: String, 
      shortTitle: String, 
      description: String, 
      level: String,
      metadata: {String: String},
      marketplaceFee: UFix64,
      id: UInt64,
    ): CardCollectionData {

      // ensure the collection exists
      let cardCollection: CardCollectionData = FantastecSwapData.getCardCollectionById(id: id)
        ?? panic(FantastecSwapData.join(["No CardCollection found with id: ", id.toString()], ""))

      // ensure the collection is not deactivated
      if (cardCollection.isDeactivated == true){
        panic("CardCollection has been deactivated, updates are not allowed")
      }

      // create a new instance, which automatically updates the data dictionaries
      var newCardCollection: CardCollectionData = CardCollectionData(
        appId: appId,
        title: title,
        shortTitle: shortTitle,
        description: description,
        level: level,
        metadata: metadata,
        marketplaceFee: marketplaceFee,
        id: id,
      )

      newCardCollection.save()

      emit CardCollectionUpdated(item: newCardCollection)
      return newCardCollection
    }
    pub fun deactivateCardCollectionById(id: UInt64): Bool {
      // ensure the collection exists
      let cardCollection: CardCollectionData = FantastecSwapData.getCardCollectionById(id: id)
        ?? panic("No CardCollection found with id: ".concat(id.toString()))
      if (cardCollection.isDeactivated) {
        return cardCollection.isDeactivated
      }

      cardCollection.deactivate()
      cardCollection.save()

      // deactivate all cards linked to this contract
      for card in FantastecSwapData.cardData.values {
        if ((card as CardData).cardCollectionId == id) {
          self.deactivateCardById(id: card.id)
        }
      }

      emit CardCollectionDeactivated(id: id)
      return cardCollection.isDeactivated
    }

    // Manage Card functions
    pub fun addCard(
      name: String, 
      level: String, 
      cardType: String, 
      aspectRatio: String, 
      metadata: {String: String},
      cardCollectionId: UInt64
    ): CardData {

      var newCard: CardData = CardData(
        name: name,
        level: level,
        cardType: cardType,
        aspectRatio: aspectRatio,
        metadata: metadata,
        cardCollectionId: cardCollectionId,
        id: nil
      )

      newCard.save()

      emit CardCreated(item: newCard)

      return newCard
    }
    pub fun updateCardById(
      name: String, 
      level: String, 
      cardType: String, 
      aspectRatio: String, 
      metadata: {String: String},
      cardCollectionId: UInt64,
      id: UInt64,
    ): CardData {
      pre {
        FantastecSwapData.getCardById(id: id) != nil: "Card not found with id: ".concat(id.toString())
        FantastecSwapData.cardCollectionData[cardCollectionId] != nil: "CardCollection not found with cardCollectionId: ".concat(cardCollectionId.toString())
      }
      let card: CardData = FantastecSwapData.getCardById(id: id)!
      if (card.isDeactivated){
        panic("Card has been deactivated, updates are not allowed")
      }

      var updatedCard: CardData = CardData(
        name: name,
        level: level,
        cardType: cardType,
        aspectRatio: aspectRatio,
        metadata: metadata,
        cardCollectionId: cardCollectionId,
        id: id
      )

      updatedCard.save()

      emit CardUpdated(item: updatedCard)

      return updatedCard
    }
    pub fun deactivateCardById(id: UInt64): Bool {
      pre {
        FantastecSwapData.cardData[id] != nil: "No card found with id: ".concat(id.toString())
      }
      
      // ensure the collection exists
      let card: CardData = FantastecSwapData.getCardById(id: id)!
      if (card.isDeactivated){
        return card.isDeactivated
      }
      card.deactivate()
      card.save()
      emit CardDeactivated(id: id)
      return card.isDeactivated
    }
    pub fun addEditionMintVolume(cardId: UInt64, edition: UInt64, mintVolume: UInt64): Bool {
      pre {
        FantastecSwapData.cardData[cardId] != nil: "No card found with id: ".concat(cardId.toString())
        mintVolume > 0: "Mint volume must be greater than 0"
        edition > 0: "Edition must be greater than 0"
      }
      let card: CardData = FantastecSwapData.cardData[cardId]!

      // update edition with maximum number allowed to be minted
      card.addEditionMintVolume(edition: edition, mintVolume: mintVolume)
      card.save()

      emit AddedEditionMintVolume(item: card)

      return true
    }
  }

  /** PUBLIC GETTING FUNCTIONS */
  // CardCollection functions
  pub fun getAllCardCollections():[FantastecSwapData.CardCollectionData]{
    var cardCollections:[FantastecSwapData.CardCollectionData] = []
    for cardCollection in self.cardCollectionData.values {
      if (!cardCollection.isDeactivated){
        cardCollections.append(cardCollection)
      }
    }
    return cardCollections;
  }

  pub fun getCardCollectionById(id: UInt64): FantastecSwapData.CardCollectionData? {
    return FantastecSwapData.cardCollectionData[id]
  }

  pub fun getCardColletionMarketFee(id: UInt64): UFix64 {
    return FantastecSwapData.cardCollectionData[id]!.marketplaceFee
  }

  pub fun getCardCollectionIds(): [UInt64] {
    var keys:[UInt64] = []
    for collection in self.cardCollectionData.values {
      if (!collection.isDeactivated){
        keys.append(collection.id)
      }
    }
    return keys;
  }

  // Card functions
  pub fun getAllCards():[FantastecSwapData.CardData]{
    var cards:[FantastecSwapData.CardData] = []
    for card in self.cardData.values {
      if (!card.isDeactivated){
        cards.append(card)
      }
    }
    return cards;
  }

  pub fun getCardById(id: UInt64): FantastecSwapData.CardData? {
    return FantastecSwapData.cardData[id]
  }

  pub fun getCardIds(): [UInt64] {
    var keys:[UInt64] = []
    for card in self.cardData.values {
      if (!card.isDeactivated){
        keys.append(card.id)
      }
    }
    return keys;
  }

  pub fun isMintable(cardId: UInt64, edition: UInt64): String {
    let card = self.getCardById(id: cardId) ?? nil
    if (card == nil){
      return FantastecSwapData.join(["No Card with cardId=", cardId.toString()], "")
    }
    let isMintable = (card! as CardData).isMintable(edition: edition)
    if (isMintable != "yes"){
      return FantastecSwapData.join([isMintable, "with cardId=", cardId.toString()], "")
    }
    return "yes"
  }

  pub fun join(_ array: [String], _ separator: String): String {
    var res = ""
    for string in array {
      res = res.concat(" ").concat(string)
    }
    return res
  }

  init() {

    self.cardCollectionData = {}
    self.nextCardCollectionId = 1

    self.cardData = {}
    self.nextCardId = 1

    self.defaultRoyaltyAddress = self.account.address

    // set storage paths
    self.AdminStoragePath = /storage/FantastecSwapAdmin
    let oldAdmin <- self.account.load<@Admin>(from: self.AdminStoragePath)
    self.account.save<@Admin>(<- create Admin(), to: self.AdminStoragePath)
    destroy oldAdmin

    emit ContractInitialized()
  }
}
