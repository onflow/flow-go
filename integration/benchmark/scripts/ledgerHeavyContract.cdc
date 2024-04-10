access(all) contract LedgerHeavy {
    access(all) fun LedgerInteractionHeavy(_ n: Int) {
        LedgerHeavy.AddManyRandomItems(n)
    }

    access(self) var items: [Item]

    // items
    access(all) event NewItemAddedEvent(id: UInt32, metadata: {String: String})

    access(self) var itemCounter: UInt32

    access(all) struct Item {

            pub let itemID: UInt32

            pub let metadata: {String: String}

            init(_ metadata: {String: String}) {
                self.itemID = LedgerHeavy.itemCounter
                self.metadata = metadata

                // inc the counter
                LedgerHeavy.itemCounter = LedgerHeavy.itemCounter + UInt32(1)

                // emit event
                emit NewItemAddedEvent(id: self.itemID, metadata: self.metadata)
            }
    }

    access(all) fun AddItem(_ metadata: {String: String}){
        let item = Item(metadata)
        self.items.append(item)
    }

    access(all) fun AddManyRandomItems(_ n: Int){
        var i = 0
        while i < n {
            LedgerHeavy.AddItem({"data": "ABCDEFGHIJKLMNOP"})
            i = i + 1
        }
    }

    init() {
        self.itemCounter = UInt32(0)
        self.items = []
    }
}
