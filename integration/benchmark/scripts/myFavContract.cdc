access(all) contract MyFavContract {

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
                self.itemID = MyFavContract.itemCounter
                self.metadata = metadata

                // inc the counter
                MyFavContract.itemCounter = MyFavContract.itemCounter + UInt32(1)

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
            MyFavContract.AddItem({"data": "ABCDEFGHIJKLMNOP"})
            i = i + 1
        }
    }

    // heavy operations
    // computation heavy function
    access(all) fun ComputationHeavy(_ n: Int) {
    	var s: Int256 = 1024102410241024
        var i = 0
        var a = Int256(7)
        var b = Int256(5)
        var c = Int256(2)
        while i < n {
            s = s * a
            s = s / b
            s = s / c
            i = i + 1
        }
        log(i)
    }

    access(all) event LargeEvent(value: Int256, str: String, list: [UInt256], dic: {String: String})

    // event heavy function
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
        MyFavContract.AddManyRandomItems(n)
    }
}
