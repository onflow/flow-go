import FUSD from 0x3c5959b568896393
import FungibleToken from 0xf233dcee88fe0abe

pub contract ChainmonstersProducts {

  /**
   * Contract events
   */

  pub event ContractInitialized()
  pub event ProductCreated(
    productID: UInt32,
    price: UFix64,
    paymentVaultType: Type,
    saleEnabled: Bool,
    totalSupply: UInt32?,
    saleEndTime: UFix64?,
    metadata: String?
  )
  pub event ProductSaleChanged(productID: UInt32, saleEnabled: Bool)
  pub event ProductPurchased(productID: UInt32, receiptID: UInt64, buyer: Address?, playerID: String?)

  /**
   * Contract-level fields
   */

  pub let CollectionStoragePath: StoragePath
  pub let CollectionPublicPath: PublicPath

  pub var nextProductID: UInt32

  access(self) var products: {UInt32: Product}
  access(self) var salesPerProduct: {UInt32: UInt32}

  /**
   * Structs
   */

  pub struct Product {
    pub let productID: UInt32

    pub let priceCuts: [PriceCut]
    pub let price: UFix64
    pub let paymentVaultType: Type
    pub var saleEnabled: Bool
    pub let totalSupply: UInt32?
    pub let saleEndTime: UFix64?
    pub let metadata: String?

    access(contract) fun setSaleEnabled(saleEnabled: Bool) {
      self.saleEnabled = saleEnabled
    }

    pub fun getSales(): UInt32 {
      return ChainmonstersProducts.salesPerProduct[self.productID]!
    }

    init(priceCuts: [PriceCut], paymentVaultType: Type, saleEnabled: Bool, totalSupply: UInt32?, saleEndTime: UFix64?, metadata: String?) {
      pre {
        priceCuts.length > 0: "Product must have at least one price cut"
      }

      let productID = ChainmonstersProducts.nextProductID

      self.productID = productID
      self.priceCuts = priceCuts
      self.paymentVaultType = paymentVaultType
      self.saleEnabled = saleEnabled
      self.totalSupply = totalSupply
      self.saleEndTime = saleEndTime
      self.metadata = metadata

      // Initialize product sale count to 0
      ChainmonstersProducts.salesPerProduct[productID] = 0

      // Increment contract-level productID counter
      ChainmonstersProducts.nextProductID = productID + 1

      var price = 0.0

      for cut in priceCuts {
        // Check if the cut receiver vault is available
        assert(cut.receiver.check(), message: "Price cut receiver capability not available")
        assert(cut.receiver.borrow()!.isInstance(paymentVaultType), message: "Cut receiver must be of given payment vault type")
        price = price + cut.amount
      }

      self.price = price

      emit ProductCreated(
        productID: self.productID,
        price: self.price,
        paymentVaultType: self.paymentVaultType,
        saleEnabled: self.saleEnabled,
        totalSupply: self.totalSupply,
        saleEndTime: self.saleEndTime,
        metadata: self.metadata
      )
    }
  }

  // A price cut represents a cut of the full product price.
  pub struct PriceCut {
    access(contract) let receiver: Capability<&{FungibleToken.Receiver}>
    pub let amount: UFix64

    init(receiver: Capability<&{FungibleToken.Receiver}>, amount: UFix64) {
      self.receiver = receiver
      self.amount = amount
    }
  }

  /**
   * Resources
   */

  // The receipt collection a user will receive when purchasing a product
  pub resource ReceiptCollection {
    access(contract) var receipts: @{UInt64: Receipt}

    // Contract-level function to save a new receipt after purchase
    access(contract) fun saveReceipt(receipt: @Receipt) {
      self.receipts[receipt.uuid] <-! receipt
    }

    // Get all receipt IDs in the collection
    pub fun getIds(): [UInt64] {
      return self.receipts.keys
    }

    pub fun borrowReceipt(receiptID: UInt64): &Receipt? {
      if self.receipts[receiptID] != nil {
        return &self.receipts[receiptID] as! &Receipt
      }

      return nil
    }

    // Check if the collection has a receipt for a given productID
    pub fun hasBoughtProduct(productID: UInt32): Bool {
      var i = 0

      for receiptID in self.getIds() {
        if self.receipts[receiptID]?.product?.productID == productID {
          return true
        }
      }

      return false
    }

    init () {
      self.receipts <- {}
    }

    destroy() {
      destroy self.receipts
    }
  }

  // A receipt references the product and the timestamp when it was purchased
  pub resource Receipt {
    pub let product: Product
    pub let purchasedAt: UFix64

    init(product: Product) {
      self.product = product
      self.purchasedAt = getCurrentBlock().timestamp
    }
  }

  // Whoever owns an admin resource can create new products, set a product enabled/disabled and create new admin resources
  pub resource Admin {
    pub fun createNewProduct(priceCuts: [PriceCut], paymentVaultType: Type, saleEnabled: Bool, totalSupply: UInt32?, saleEndTime: UFix64?, metadata: String?) {
      var product = Product(
        priceCuts: priceCuts,
        paymentVaultType: paymentVaultType,
        saleEnabled: saleEnabled,
        totalSupply: totalSupply,
        saleEndTime: saleEndTime,
        metadata: metadata
      )

      ChainmonstersProducts.products[product.productID] = product
    }

    pub fun setProductSaleEnabled(productID: UInt32, saleEnabled: Bool) {
      var product = ChainmonstersProducts.products[productID] ?? panic("Product not found")

      if product.saleEnabled == saleEnabled {
        // Do nothing if the sale is already in the given state
        return
      }

      product.setSaleEnabled(saleEnabled: saleEnabled)

      ChainmonstersProducts.products[productID] = product

      emit ProductSaleChanged(productID: productID, saleEnabled: saleEnabled)
    }

    // Purchase a product if it is available
    pub fun purchase(
      productID: UInt32,
      buyerReceiptCollection: &ReceiptCollection,
      paymentVault: @FungibleToken.Vault,
      playerID: String
    ) {
      pre {
        ChainmonstersProducts.getProduct(productID: productID) != nil:
          "Product not found"
        ChainmonstersProducts.getProduct(productID: productID)!.saleEnabled:
          "Product sale is not enabled"
        ChainmonstersProducts.getProduct(productID: productID)!.totalSupply == nil ||
        ChainmonstersProducts.getProduct(productID: productID)!.getSales() < ChainmonstersProducts.getProduct(productID: productID)!.totalSupply!:
          "Product out of stock"
        ChainmonstersProducts.getProduct(productID: productID)!.saleEndTime == nil ||
        getCurrentBlock().timestamp < ChainmonstersProducts.getProduct(productID: productID)!.saleEndTime!:
          "Product sale has ended"
        paymentVault.isInstance(ChainmonstersProducts.getProduct(productID: productID)!.paymentVaultType):
          "Payment vault is of wrong type"
        paymentVault.balance == ChainmonstersProducts.products[productID]!.price:
          "Payment does not equal product price"
      }

      let product = ChainmonstersProducts.getProduct(productID: productID)!

      // We set a fallback payment receiver in case not all price cut receivers are available.
      // The first valid price cut receiver will be elected to receive all the rest funds.
      var fallbackPaymentReceiver: &{FungibleToken.Receiver}? = nil

      for cut in product.priceCuts {
        if let paymentReceiver = cut.receiver.borrow() {
          paymentReceiver.deposit(from: <- paymentVault.withdraw(amount: cut.amount))
          if (fallbackPaymentReceiver == nil) {
            fallbackPaymentReceiver = paymentReceiver
          }
        }
      }

      // Panic if there are no valid payment receivers at all
      assert(fallbackPaymentReceiver != nil, message: "No valid payment receivers")

      // Fallback payment receiver gets all the rest funds
      fallbackPaymentReceiver!.deposit(from: <- paymentVault)

      // Create a new receipt resource
      let receipt <- create Receipt(product: product)

      // Get the receipt ID for the purchase event
      let receiptID = receipt.uuid

      // Save receipt to the buyer's collection
      buyerReceiptCollection.saveReceipt(receipt: <- receipt)

      // Increment sales counter for this product
      ChainmonstersProducts.salesPerProduct[productID] = ChainmonstersProducts.salesPerProduct[productID]! + 1

      // Emit purchase event
      emit ProductPurchased(productID: product.productID, receiptID: receiptID, buyer: buyerReceiptCollection.owner?.address, playerID: playerID)
    }

    // createNewAdmin creates a new Admin resource
    pub fun createNewAdmin(): @Admin {
        return <-create Admin()
    }
  }

  /**
   * Contract-level functions
   */

  // Create a new empty receipt collection for a purchaser
  pub fun createReceiptCollection(): @ReceiptCollection {
    return <-create ReceiptCollection()
  }

  // Get a single product by id
  pub fun getProduct(productID: UInt32): Product? {
    return self.products[productID]
  }

  init() {
    self.CollectionStoragePath = /storage/chainmonstersProductsCollection
    self.CollectionPublicPath = /public/chainmonstersProductsCollection

    self.products = {}
    self.salesPerProduct = {}
    self.nextProductID = 1

    self.account.save<@Admin>(<- create Admin(), to: /storage/chainmonstersProductsAdmin)

    emit ContractInitialized()
  }
}
