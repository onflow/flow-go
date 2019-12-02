struct Car {
    pub var model: String
    pub var make: String
    pub var trim: String

    init(fullname: String) {
        //TODO
    }

    init(params: [String;3]) {
        //TODO
    }

    init(model:String, make:String, trim:String) {
        //TODO
    }
}

struct Fleet {
    pub let cars: [Car]

    init(car1: Car, car2: Car?, car3: Car?) {
        //TODO
    }
}
