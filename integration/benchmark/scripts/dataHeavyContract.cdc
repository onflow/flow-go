access(all) contract DataHeavy {

    init() {
        self.itemCounter = UInt32(0)
        self.items = []
    }

    // items
    access(all) event NewItemAddedEvent(id: UInt32, metadata: {String: String})

    access(self) var itemCounter: UInt32

    access(all) struct Item {

            pub let itemID: UInt32

            pub let metadata: {String: String}

            init(_ metadata: {String: String}) {
                self.itemID = DataHeavy.itemCounter
                self.metadata = metadata

                // inc the counter
                DataHeavy.itemCounter = DataHeavy.itemCounter + UInt32(1)

                // emit event
                emit NewItemAddedEvent(id: self.itemID, metadata: self.metadata)
            }
    }

    access(self) var items: [Item]

    access(all) fun AddItem(_ metadata: {String: String}){
        let item = Item(metadata)
        self.items.append(item)
    }

    access(all) fun AddManyRandomItems(_ n: Int){
        var i = 0
        while i < n {
            DataHeavy.AddItem({"data": "ABCDEFGHIJKLMNOP"})
            i = i + 1
        }
    }


    access(all) event LargeEvent(value: Int256, str: String, list: [UInt256], dic: {String: String})

    access(all) fun EventHeavy(_ n: Int) {
        var s: Int256 = 1024102410241024
        var i = 0

        while i < n {
            emit LargeEvent(value: s, str: s.toString(), list:[], dic:{s.toString():s.toString()})
            i = i + 1
        }
        log(i)
    }

    access(all) fun LedgerInteractionHeavy(_ n: Int) {
        DataHeavy.AddManyRandomItems(n)
    }
}
